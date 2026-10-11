package document

import (
	"image"
	"image/color"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/NimbleTerminal/gloss/internal/theme"
)

var selectSource = "# Title\n\nHello wide world of words.\n\nSecond paragraph here.\n\n```\ncode line one\ncode line two\n```\n\n| A | B |\n| --- | --- |\n| alpha | beta |\n\n" +
	strings.Repeat("wrapped words ", 40) + "end.\n"

func selectLoader(t *testing.T) *Loader {
	t.Helper()
	l := &Loader{Files: fstest.MapFS{"doc.md": {Data: []byte(selectSource)}}}
	t.Cleanup(func() { _ = l.Close() })
	return l
}

// runAt is the first text operation whose paragraph holds s, and the page x
// where s starts in it.
func runAt(t *testing.T, l *Loader, s string) (*operation, int) {
	t.Helper()
	for i := range l.direct.ops {
		op := &l.direct.ops[i]
		if op.run == nil {
			continue
		}
		at := strings.Index(string(op.text), s)
		if at < 0 {
			continue
		}
		r := len([]rune(string(op.text)[:at]))
		if r < op.run.Runes.Offset || r >= op.run.Runes.Offset+op.run.Runes.Count {
			continue
		}
		return op, runX(op, r-op.run.Runes.Offset)
	}
	t.Fatalf("no run holds %q", s)
	return nil, 0
}

func TestTypesetSelectionTextAndPlaces(t *testing.T) {
	l := selectLoader(t)
	q := Request{Path: "doc.md", Typeset: true, TypesetWidth: 192, TypesetView: &TypesetViewport{Height: 1200}}
	if r := l.Load(q); r.Err != nil {
		t.Fatal(r.Err)
	}
	from, x0 := runAt(t, l, "wide")
	to, _ := runAt(t, l, "Second")
	anchor := image.Pt(x0+1, (from.rect.Min.Y+from.rect.Max.Y)/2)
	focus := image.Pt(runX(to, strings.Index(string(to.text), "Second")+len("Second")-to.run.Runes.Offset)-1, (to.rect.Min.Y+to.rect.Max.Y)/2)
	q.TypesetSelect = &TypesetSelection{Anchor: TypesetPlace{Point: anchor}, Focus: TypesetPlace{Point: focus}}
	r := l.Load(q)
	if r.Err != nil {
		t.Fatal(r.Err)
	}
	if r.TypesetSelected != "wide world of words.\n\nSecond" {
		t.Fatalf("selected %q", r.TypesetSelected)
	}
	s := r.TypesetSelection
	if s == nil || !s.Anchor.Placed || !s.Focus.Placed || s.Anchor.Block == s.Focus.Block {
		t.Fatalf("places not resolved: %+v", s)
	}
	// The same text reads back when the places, not the points, are given,
	// in either order and at another width.
	q.TypesetSelect = &TypesetSelection{Anchor: s.Focus, Focus: s.Anchor}
	q.TypesetSelect.Anchor.Point, q.TypesetSelect.Focus.Point = image.Point{}, image.Point{}
	q.TypesetWidth = 384
	wide := l.Load(q)
	if wide.Err != nil || wide.TypesetSelected != r.TypesetSelected {
		t.Fatalf("reflowed selection %q %v", wide.TypesetSelected, wide.Err)
	}
	s.Anchor.Point, s.Focus.Point = image.Point{}, image.Point{}
	if ws := wide.TypesetSelection; ws == nil || ws.Anchor != s.Focus || ws.Focus != s.Anchor {
		t.Fatalf("places changed across widths: %+v", wide.TypesetSelection)
	}
	// A place that is not in the document selects nothing.
	q.TypesetSelect = &TypesetSelection{Anchor: TypesetPlace{Block: 99, Placed: true}, Focus: s.Focus}
	if gone := l.Load(q); gone.Err != nil || gone.TypesetSelection != nil || gone.TypesetSelected != "" {
		t.Fatalf("missing place selected %q", gone.TypesetSelected)
	}
}

func TestTypesetSelectionSeparators(t *testing.T) {
	l := selectLoader(t)
	q := Request{Path: "doc.md", Typeset: true, TypesetWidth: 192, TypesetView: &TypesetViewport{Height: 2400}}
	if r := l.Load(q); r.Err != nil {
		t.Fatal(r.Err)
	}
	pick := func(from, to string) string {
		t.Helper()
		a, x0 := runAt(t, l, from)
		b, _ := runAt(t, l, to)
		q.TypesetSelect = &TypesetSelection{
			Anchor: TypesetPlace{Point: image.Pt(x0+1, (a.rect.Min.Y+a.rect.Max.Y)/2)},
			Focus:  TypesetPlace{Point: image.Pt(runX(b, strings.Index(string(b.text), to)+len([]rune(to))-b.run.Runes.Offset)-1, (b.rect.Min.Y+b.rect.Max.Y)/2)},
		}
		r := l.Load(q)
		if r.Err != nil {
			t.Fatal(r.Err)
		}
		return r.TypesetSelected
	}
	if got := pick("code line one", "code line two"); got != "code line one\ncode line two" {
		t.Fatalf("code lines %q", got)
	}
	if got := pick("alpha", "beta"); got != "alpha\tbeta" {
		t.Fatalf("table cells %q", got)
	}
	got := pick("wrapped", "end.")
	if strings.Contains(got, "  ") || strings.Contains(got, "\n") || !strings.HasSuffix(got, "end.") || strings.Count(got, "wrapped") != 40 {
		t.Fatalf("wrapped paragraph %q", got)
	}
	// Points off the text snap to the nearest line: the margin left of a
	// line is its start, below the last line is its end.
	a, _ := runAt(t, l, "Hello")
	q.TypesetSelect = &TypesetSelection{Anchor: TypesetPlace{Point: image.Pt(0, a.rect.Min.Y)}, Focus: TypesetPlace{Point: image.Pt(1<<20, 1125*MaxPages)}}
	r := l.Load(q)
	if r.Err != nil || !strings.HasPrefix(r.TypesetSelected, "Hello wide") || !strings.HasSuffix(r.TypesetSelected, "end.") {
		t.Fatalf("snapped selection %q %v", r.TypesetSelected, r.Err)
	}
}

func TestTypesetSelectionHighlight(t *testing.T) {
	l := selectLoader(t)
	q := Request{Path: "doc.md", Typeset: true, TypesetWidth: 192, TypesetView: &TypesetViewport{Height: 1200}}
	plain := l.Load(q)
	if plain.Err != nil {
		t.Fatal(plain.Err)
	}
	a, x0 := runAt(t, l, "wide")
	x1 := runX(a, strings.Index(string(a.text), "world")-a.run.Runes.Offset)
	y := (a.rect.Min.Y + a.rect.Max.Y) / 2
	q.TypesetSelect = &TypesetSelection{Anchor: TypesetPlace{Point: image.Pt(x0+1, y)}, Focus: TypesetPlace{Point: image.Pt(x1-1, y)}}
	r := l.Load(q)
	if r.Err != nil || r.TypesetSelected != "wide " {
		t.Fatalf("selected %q %v", r.TypesetSelected, r.Err)
	}
	want := theme.Get("light").Selection
	b := plain.Image.Bounds()
	inside, outside := 0, 0
	for yy := b.Min.Y; yy < b.Max.Y; yy++ {
		for xx := b.Min.X; xx < b.Max.X; xx++ {
			before, after := plain.Image.At(xx, yy), r.Image.At(xx, yy)
			if before == after {
				continue
			}
			if color.RGBAModel.Convert(after) == want {
				if xx >= x0 && xx < x1 && yy >= a.rect.Min.Y && yy < a.rect.Max.Y {
					inside++
				} else {
					outside++
				}
			}
		}
	}
	if inside == 0 || outside > 0 {
		t.Fatalf("highlight pixels inside %d outside %d", inside, outside)
	}
	if r.TypesetView == nil || r.TypesetView.Width == 0 {
		t.Fatal("view width missing")
	}
}

// Copy uses logical text, including whitespace omitted by visual wrapping.
func TestTypesetSelectionPreservesWrappedText(t *testing.T) {
	token := strings.Repeat("abcdefghij", 25)
	for _, tc := range []struct{ name, source, want string }{
		{"token", token, token},
		{"url", "https://example.com/" + token, "https://example.com/" + token},
		{"spaces", strings.Repeat("café   résumé ", 20) + "end", strings.Repeat("café   résumé ", 20) + "end"},
		{"code", "```text\n" + token + "   tail\n    next line\n```", token + "   tail\n    next line"},
		{"styles", "start **" + token + "** end", "start " + token + " end"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			l := &Loader{Files: fstest.MapFS{"doc.md": {Data: []byte(tc.source)}}}
			defer l.Close()
			q := Request{Path: "doc.md", Typeset: true, TypesetWidth: 192, TypesetView: &TypesetViewport{Height: 4096}}
			if r := l.Load(q); r.Err != nil {
				t.Fatal(r.Err)
			}
			q.TypesetSelect = &TypesetSelection{Anchor: TypesetPlace{Point: image.Pt(0, 0)}, Focus: TypesetPlace{Point: image.Pt(99999, 99999)}}
			r := l.Load(q)
			if r.Err != nil {
				t.Fatal(r.Err)
			}
			if r.TypesetSelected != tc.want {
				t.Fatalf("copy differs: got %q, want %q", r.TypesetSelected, tc.want)
			}
			// Placed endpoints preserve the exact bytes through font and width changes.
			q.TypesetSelect = r.TypesetSelection
			q.TypesetWidth = 384
			q.Font = "technical"
			r = l.Load(q)
			if r.Err != nil || r.TypesetSelected != tc.want {
				t.Fatalf("reflowed copy %q: %v", r.TypesetSelected, r.Err)
			}
		})
	}
}
