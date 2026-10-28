package document

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// An export of a page draws it as its own CSS lays it out, with the
// pictures beside it, in slices that end between lines.
func TestHTMLLayoutExport(t *testing.T) {
	dir := t.TempDir()
	pic := image.NewRGBA(image.Rect(0, 0, 120, 60))
	// A blue no anti-aliased text makes.
	for i := 0; i < len(pic.Pix); i += 4 {
		copy(pic.Pix[i:], []byte{0x10, 0x80, 0xf0, 0xff})
	}
	var b bytes.Buffer
	if err := png.Encode(&b, pic); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pic.png"), b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	page := `<!doctype html><html><body style="background: #ffffff"><h1 style="color: #ff0000">Report</h1><p><img src="pic.png" alt="a pic"><img src="https://example.com/x.png" alt="remote"></p>` +
		strings.Repeat("<p>A line of the report that goes on long enough to wrap across the page more than once or twice.</p>", 80) + `</body></html>`
	path := filepath.Join(dir, "report.html")
	if err := os.WriteFile(path, []byte(page), 0o644); err != nil {
		t.Fatal(err)
	}
	l := &Loader{}
	defer l.Close()
	r := l.Load(Request{Path: path, Layout: true, Page: 1, Strict: true})
	if r.Err != nil {
		t.Fatal(r.Err)
	}
	if r.Kind != "html" || r.Markdown != nil || r.Image == nil || r.Pages < 2 {
		t.Fatalf("kind=%q markdown=%v image=%v pages=%d", r.Kind, r.Markdown != nil, r.Image != nil, r.Pages)
	}
	if b := r.Image.Bounds(); b.Dx() != htmlLayoutWidth || b.Dy() > htmlSliceHeight || b.Dy() < htmlSliceHeight/2 {
		t.Fatalf("slice %v", b)
	}
	// The picture beside the page is drawn at its own size; red is the heading.
	blue, red := 0, 0
	b2 := r.Image.Bounds()
	for y := b2.Min.Y; y < b2.Max.Y; y++ {
		for x := b2.Min.X; x < b2.Max.X; x++ {
			c := color.RGBAModel.Convert(r.Image.At(x, y)).(color.RGBA)
			switch {
			case c.R == 0x10 && c.G == 0x80 && c.B == 0xf0:
				blue++
			case c.R > 200 && c.G < 60 && c.B < 60:
				red++
			}
		}
	}
	if blue != 120*60 || red == 0 {
		t.Fatalf("picture pixels %d, want %d; heading pixels %d", blue, 120*60, red)
	}
	img, err := ExportImage(r, 1024)
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); max(b.Dx(), b.Dy()) != 1024 {
		t.Fatalf("export %v, want a 1024 edge", b)
	}
	last := l.Load(Request{Path: path, Layout: true, Page: r.Pages, Strict: true})
	if last.Err != nil || last.Page != r.Pages || last.Image == nil || last.Image.Bounds().Dy() > htmlSliceHeight {
		t.Fatalf("last slice: %v page %d %v", last.Err, last.Page, last.Image.Bounds())
	}
	if past := l.Load(Request{Path: path, Layout: true, Page: r.Pages + 1, Strict: true}); past.Err == nil || !strings.Contains(past.Err.Error(), "past the end") {
		t.Fatalf("past the end: %v", past.Err)
	}
	// Text and typeset still take the Markdown.
	if md := l.Load(Request{Path: path, Layout: true, TextOnly: true}); md.Err != nil || md.Markdown == nil {
		t.Fatalf("text: %v", md.Err)
	}
}

func TestHTMLLayoutTooLarge(t *testing.T) {
	path := filepath.Join(t.TempDir(), "big.html")
	if err := os.WriteFile(path, []byte("<html>"+strings.Repeat("<p>x</p>", 30000)+"</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	l := &Loader{}
	defer l.Close()
	if r := l.Load(Request{Path: path, Layout: true, Page: 1}); r.Err == nil || !strings.Contains(r.Err.Error(), "--typeset") {
		t.Fatalf("err = %v", r.Err)
	}
}

// The viewer's page view: the part of the page a viewport asks for, at the
// viewer's device pixels, from one layout kept while it scrolls.
func TestHTMLView(t *testing.T) {
	page := `<html><body><p>See <a href="https://example.com/more">more</a>.</p>` +
		strings.Repeat("<p>A line of the report that goes on long enough to wrap across the page.</p>", 60) +
		`<h2 id="end">The end</h2><p>Last.</p></body></html>`
	path := filepath.Join(t.TempDir(), "page.html")
	if err := os.WriteFile(path, []byte(page), 0o644); err != nil {
		t.Fatal(err)
	}
	l := &Loader{}
	defer l.Close()
	ask := func(v TypesetViewport, measure int) Result {
		t.Helper()
		r := l.Load(Request{Path: path, MarkdownMode: "typeset", TypesetView: &v, TypesetWidth: measure, Theme: Theme{Name: "light"}, Page: 1})
		if r.Err != nil {
			t.Fatal(r.Err)
		}
		return r
	}
	// 768 points of measure are 1024 CSS pixels, 1600 typeset pixels; the
	// raster is the device pixels asked, twice those.
	r := ask(TypesetViewport{Height: 900, Pixels: image.Pt(3200, 1800)}, 768)
	if r.Kind != "html" || r.Typeset == nil || r.Markdown != nil || r.Image.Bounds().Size() != image.Pt(3200, 1800) {
		t.Fatalf("kind %q typeset %v markdown %v image %v", r.Kind, r.Typeset != nil, r.Markdown != nil, r.Image.Bounds())
	}
	if v := r.TypesetView; v == nil || v.Width != 1600 || v.Height != 900 || r.TypesetExtent <= 900 || !r.TypesetDone {
		t.Fatalf("view %+v extent %d", r.TypesetView, r.TypesetExtent)
	}
	if len(r.Links) != 1 || r.Links[0].URL != "https://example.com/more" || !r.Links[0].Rect.In(r.Image.Bounds()) {
		t.Fatalf("links %v", r.Links)
	}
	kept := l.html
	end := ask(TypesetViewport{Height: 900, End: true}, 768)
	if l.html != kept || end.TypesetView.Top != r.TypesetExtent-900 || len(end.Links) != 0 {
		t.Fatalf("end: kept %v top %d of %d, links %d", l.html == kept, end.TypesetView.Top, r.TypesetExtent, len(end.Links))
	}
	if at := ask(TypesetViewport{Height: 300, Link: "#end"}, 768); at.TypesetView.Top <= 0 || at.TypesetView.Top >= r.TypesetExtent {
		t.Fatalf("anchor at %d of %d", at.TypesetView.Top, r.TypesetExtent)
	}
	// A drag from the top of the page to the link's last letter selects the
	// text between, and comes back placed, to be asked for again as it
	// scrolls; one to the middle of the link stops in the word.
	link := r.Links[0].Rect
	middle := image.Pt((link.Min.X+link.Max.X)/4, (link.Min.Y+link.Max.Y)/4) // device pixels, at twice typeset ones
	last := image.Pt((link.Max.X-2)/2, middle.Y)
	drag := TypesetSelection{Anchor: TypesetPlace{Point: image.Pt(0, 0)}, Focus: TypesetPlace{Point: last}}
	selected := func(sel TypesetSelection, measure int) Result {
		t.Helper()
		v := TypesetViewport{Height: 900}
		res := l.Load(Request{Path: path, MarkdownMode: "typeset", TypesetView: &v, TypesetWidth: measure, TypesetSelect: &sel, Theme: Theme{Name: "light"}, Page: 1})
		if res.Err != nil {
			t.Fatal(res.Err)
		}
		return res
	}
	first := selected(drag, 768)
	if first.TypesetSelected != "See more" || first.TypesetSelection == nil || !first.TypesetSelection.Anchor.Placed || !first.TypesetSelection.Focus.Placed {
		t.Fatalf("selected %q, %+v", first.TypesetSelected, first.TypesetSelection)
	}
	if again := selected(*first.TypesetSelection, 768); again.TypesetSelected != "See more" {
		t.Fatalf("placed selection gave %q", again.TypesetSelected)
	}
	part := selected(TypesetSelection{Anchor: drag.Anchor, Focus: TypesetPlace{Point: middle}}, 768).TypesetSelected
	if !strings.HasPrefix(part, "See m") || len(part) >= len("See more") {
		t.Fatalf("to the middle of the link selected %q", part)
	}
	// Another width is another layout, where the old places are not.
	narrow := selected(*first.TypesetSelection, 384)
	if l.html == kept || narrow.TypesetView.Width != 800 || narrow.TypesetExtent <= r.TypesetExtent {
		t.Fatalf("narrow: relaid %v width %d extent %d", l.html != kept, narrow.TypesetView.Width, narrow.TypesetExtent)
	}
	if narrow.TypesetSelection != nil || narrow.TypesetSelected != "" || narrow.TypesetAsked == nil {
		t.Fatalf("stale selection: %q", narrow.TypesetSelected)
	}
	// A page that sets no colors takes the theme's, and a new theme or font
	// lays it out again.
	kept = l.html
	v := TypesetViewport{Height: 900}
	dark := l.Load(Request{Path: path, MarkdownMode: "typeset", TypesetView: &v, TypesetWidth: 384, Theme: testTheme("dark"), Page: 1})
	if dark.Err != nil || l.html == kept || dark.Image.At(1, 1) != testTheme("dark").Palette.Background {
		t.Fatalf("dark: %v relaid %v background %v", dark.Err, l.html != kept, dark.Image.At(1, 1))
	}
	kept = l.html
	v = TypesetViewport{Height: 900}
	if tech := l.Load(Request{Path: path, MarkdownMode: "typeset", TypesetView: &v, TypesetWidth: 384, Theme: testTheme("dark"), Font: "technical", Page: 1}); tech.Err != nil || l.html == kept {
		t.Fatalf("technical: %v relaid %v", tech.Err, l.html != kept)
	}
}

// A page nthtml will not lay out is shown as its text, saying why.
func TestHTMLViewTooLarge(t *testing.T) {
	path := filepath.Join(t.TempDir(), "big.html")
	if err := os.WriteFile(path, []byte("<html>"+strings.Repeat("<p>x</p>", 30000)+"</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	l := &Loader{}
	defer l.Close()
	r := l.Load(Request{Path: path, MarkdownMode: "typeset", TypesetView: &TypesetViewport{Height: 900}, Theme: Theme{Name: "light"}, Page: 1})
	if r.Err != nil || r.Markdown == nil || r.Image != nil {
		t.Fatalf("err %v markdown %v", r.Err, r.Markdown != nil)
	}
	found := false
	for _, f := range r.Info {
		found = found || (f.Label == "Not laid out" && strings.Contains(f.Value, "too large"))
	}
	if !found {
		t.Fatalf("info %v", r.Info)
	}
}

// A page far taller than one picture may be is laid out, and scrolled to
// its end, as the page view paints only what is shown.
func TestHTMLViewLong(t *testing.T) {
	path := filepath.Join(t.TempDir(), "long.html")
	page := "<html><body>" + strings.Repeat("<p>A paragraph of a long page.</p>", 3000) + "<p>Last.</p></body></html>"
	if err := os.WriteFile(path, []byte(page), 0o644); err != nil {
		t.Fatal(err)
	}
	l := &Loader{}
	defer l.Close()
	v := TypesetViewport{Height: 900, End: true}
	r := l.Load(Request{Path: path, MarkdownMode: "typeset", TypesetView: &v, Theme: Theme{Name: "light"}, Page: 1})
	if r.Err != nil || r.Image == nil || r.TypesetExtent <= 16384*htmlScale || r.TypesetView.Top != r.TypesetExtent-900 {
		t.Fatalf("err %v extent %d view %+v", r.Err, r.TypesetExtent, r.TypesetView)
	}
}

// A page shorter than the view goes on in its own canvas color: the
// theme's where it sets no colors, its own, or white, where it does.
func TestHTMLViewCanvas(t *testing.T) {
	for _, c := range []struct {
		page string
		want color.Color
	}{
		{`<p>Short.</p>`, testTheme("dark").Palette.Background},
		{`<style>.a{color:#666}</style><p class=a>Short.</p>`, color.RGBA{0xff, 0xff, 0xff, 0xff}},
		{`<style>body{background:#223;color:#eee}</style><p>Short.</p>`, color.RGBA{0x22, 0x22, 0x33, 0xff}},
	} {
		path := filepath.Join(t.TempDir(), "short.html")
		if err := os.WriteFile(path, []byte("<html><body>"+c.page+"</body></html>"), 0o644); err != nil {
			t.Fatal(err)
		}
		l := &Loader{}
		v := TypesetViewport{Height: 900}
		r := l.Load(Request{Path: path, MarkdownMode: "typeset", TypesetView: &v, Theme: testTheme("dark"), Page: 1})
		l.Close()
		if r.Err != nil {
			t.Fatal(r.Err)
		}
		b := r.Image.Bounds()
		if top, bottom := r.Image.At(1, 1), r.Image.At(1, b.Dy()-2); top != c.want || bottom != c.want {
			t.Errorf("%s: top %v bottom %v, want %v", c.page, top, bottom, c.want)
		}
	}
}

// A page's prefers-color-scheme rules follow the theme, light or dark, and
// a change of theme lays it out again.
func TestHTMLViewColorScheme(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scheme.html")
	page := `<html><head><style>body{background:#fff;color:#000}
@media (prefers-color-scheme: dark){body{background:#123;color:#eee}}</style></head><body><p>Short.</p></body></html>`
	if err := os.WriteFile(path, []byte(page), 0o644); err != nil {
		t.Fatal(err)
	}
	l := &Loader{}
	defer l.Close()
	for _, c := range []struct {
		theme string
		want  color.Color
	}{
		{"light", color.RGBA{0xff, 0xff, 0xff, 0xff}},
		{"dark", color.RGBA{0x11, 0x22, 0x33, 0xff}},
		{"light", color.RGBA{0xff, 0xff, 0xff, 0xff}},
	} {
		v := TypesetViewport{Height: 900}
		r := l.Load(Request{Path: path, MarkdownMode: "typeset", TypesetView: &v, Theme: testTheme(c.theme), Page: 1})
		if r.Err != nil {
			t.Fatal(r.Err)
		}
		if got := r.Image.At(1, 1); got != c.want {
			t.Errorf("%s: %v, want %v", c.theme, got, c.want)
		}
	}
}

// A page's links are its items, outlined when focused, and one to a place
// on the page scrolls there.
func TestHTMLViewSections(t *testing.T) {
	path, err := filepath.Abs("../../examples/sections.html")
	if err != nil {
		t.Fatal(err)
	}
	l := &Loader{}
	defer l.Close()
	ask := func(v TypesetViewport, focus []image.Rectangle) Result {
		t.Helper()
		r := l.Load(Request{Path: path, MarkdownMode: "typeset", TypesetView: &v, TypesetWidth: 600, TypesetFocus: focus, Theme: testTheme("light"), Page: 1})
		if r.Err != nil {
			t.Fatal(r.Err)
		}
		return r
	}
	r := ask(TypesetViewport{Height: 600}, nil)
	if len(r.TypesetItems) != 2 || !r.TypesetItemsAll || r.TypesetItems[0].URL != "#wednesday" || r.TypesetItems[1].URL != "#monday-tide" {
		t.Fatalf("items %+v, all %v", r.TypesetItems, r.TypesetItemsAll)
	}
	for _, it := range r.TypesetItems {
		if it.Kind != TypesetLinkItem || len(it.Rects) == 0 || it.Rects[0].Max.Y > r.TypesetExtent {
			t.Fatalf("item %+v", it)
		}
	}
	// Focused, a link is outlined in the theme's accent.
	focus := r.TypesetItems[0].Rects
	accent := testTheme("light").Palette.Accent
	outlined := ask(TypesetViewport{Height: 1000}, focus)
	if len(outlined.TypesetFocus) == 0 {
		t.Fatal("focus not answered")
	}
	edge := focus[0].Min.Sub(image.Pt(1, 1))
	if got := color.RGBAModel.Convert(outlined.Image.At(edge.X, edge.Y)).(color.RGBA); got != accent {
		t.Fatalf("outline at %v is %v, not %v", edge, got, accent)
	}
	// Wednesday is below the first screen; Monday's tide is on it.
	at := ask(TypesetViewport{Height: 600, Link: "#wednesday"}, nil)
	back := ask(TypesetViewport{Height: 600, Link: "#monday-tide"}, nil)
	if at.TypesetView.Top <= 600 || back.TypesetView.Top >= at.TypesetView.Top {
		t.Fatalf("wednesday at %d, monday's tide at %d", at.TypesetView.Top, back.TypesetView.Top)
	}
}
