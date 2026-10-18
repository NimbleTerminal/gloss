package document

import (
	"fmt"
	"strings"
	"testing"
	"testing/fstest"
)

func linesSource(n int) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "line %d of the file\n", i)
	}
	return b.String()
}

// With numbers asked for, a text document's view says so; how the gutter is
// laid out is nttypeset's.
func TestTypesetLineNumbersGutter(t *testing.T) {
	l := &Loader{Files: fstest.MapFS{"main.go": {Data: []byte("package main\n\nfunc main() {}\n")}}}
	defer l.Close()
	q := Request{Path: "main.go", Typeset: true, Page: 1, TypesetView: &TypesetViewport{Height: 400}}
	plain := l.Load(q)
	if plain.Err != nil {
		t.Fatal(plain.Err)
	}
	if plain.TypesetView.Lines != 3 || plain.TypesetView.Line != 1 || plain.TypesetView.Numbers {
		t.Fatalf("view without numbers %+v", plain.TypesetView)
	}
	q.TypesetLineNumbers = true
	r := l.Load(q)
	if r.Err != nil {
		t.Fatal(r.Err)
	}
	if !r.TypesetView.Numbers || r.TypesetView.Lines != 3 || r.TypesetView.Line != 1 {
		t.Fatalf("view with numbers %+v", r.TypesetView)
	}
}

// Asking for a line puts that line where the first line sits at the top of
// the page; the result says which line is at the top, and how many there
// are. Past the end is the end. Markdown pages have no lines.
func TestTypesetGoToLine(t *testing.T) {
	l := &Loader{Files: fstest.MapFS{"long.txt": {Data: []byte(linesSource(300))}, "page.md": {Data: []byte("# Title\n\nProse.\n")}}}
	defer l.Close()
	q := Request{Path: "long.txt", Typeset: true, Page: 1, TypesetView: &TypesetViewport{Height: 400, Line: 120}}
	r := l.Load(q)
	if r.Err != nil {
		t.Fatal(r.Err)
	}
	if want := l.direct.Starts()[119] - 50; r.TypesetView.Top != want || r.TypesetView.Line != 120 || r.TypesetView.Lines != 300 {
		t.Fatalf("go to 120: %+v, want top %d", r.TypesetView, want)
	}
	// Scrolling on from there reports the line at the top.
	q.TypesetView = &TypesetViewport{Height: 400, Top: r.TypesetView.Top + 200}
	if r = l.Load(q); r.Err != nil || r.TypesetView.Line <= 120 || r.TypesetView.Line > 130 {
		t.Fatalf("scrolled view %+v", r.TypesetView)
	}
	q.TypesetView = &TypesetViewport{Height: 400, Line: 9999}
	r = l.Load(q)
	if r.Err != nil || !r.TypesetDone || r.TypesetView.Top != max(0, r.TypesetExtent-400) {
		t.Fatalf("past the end %+v extent %d", r.TypesetView, r.TypesetExtent)
	}
	q = Request{Path: "page.md", Typeset: true, Page: 1, TypesetLineNumbers: true, TypesetView: &TypesetViewport{Height: 400, Line: 2}}
	if r = l.Load(q); r.Err != nil || r.TypesetView.Lines != 0 || r.TypesetView.Line != 0 || r.TypesetView.Top != 0 {
		t.Fatalf("markdown page %+v", r.TypesetView)
	}
}

// Chopped, a long line of a text page is laid out on one row, however wide,
// and the page pans as far as the widest line. A Markdown page keeps wrapping.
func TestTypesetChopLongLines(t *testing.T) {
	long := "short\n" + strings.Repeat("a long line of words that will wrap at the measure ", 8) + "\nend\n"
	l := &Loader{Files: fstest.MapFS{"long.txt": {Data: []byte(long)}, "page.md": {Data: []byte("# Title\n\n" + strings.Repeat("prose ", 300) + "\n")}}}
	defer l.Close()
	q := Request{Path: "long.txt", Typeset: true, Page: 1, TypesetView: &TypesetViewport{Height: 400}}
	if r := l.Load(q); r.Err != nil || r.TypesetView.Chop {
		t.Fatalf("plain %+v %v", r.TypesetView, r.Err)
	}
	// A line taller than the one-row first line has wrapped.
	wrapped := func(block int) bool {
		starts := l.direct.Starts()
		return starts[block+1]-starts[block] > starts[1]-starts[0]
	}
	if !wrapped(1) {
		t.Fatal("long line did not wrap without chop")
	}
	q.TypesetChop = true
	r := l.Load(q)
	if r.Err != nil || !r.TypesetView.Chop {
		t.Fatalf("chopped %+v %v", r.TypesetView, r.Err)
	}
	if wrapped(1) {
		t.Fatal("chopped line wrapped")
	}
	widest := l.direct.ExtentX()
	if widest <= l.direct.Width() {
		t.Fatal("chopped line does not reach past the page width")
	}
	q.TypesetView = &TypesetViewport{Height: 400, Left: 1 << 20}
	r = l.Load(q)
	if r.Err != nil || r.TypesetView.Left != widest+50-r.TypesetView.Width || r.TypesetView.Left <= 0 {
		t.Fatalf("pan clamp %+v, widest %d", r.TypesetView, widest)
	}
	// Panning back to the left and wrapping again.
	q.TypesetChop = false
	q.TypesetView = &TypesetViewport{Height: 400, Left: 1 << 20}
	if r = l.Load(q); r.Err != nil || r.TypesetView.Chop || r.TypesetView.Left != 0 || !wrapped(1) {
		t.Fatalf("wrapped again %+v", r.TypesetView)
	}
	q = Request{Path: "page.md", Typeset: true, Page: 1, TypesetChop: true, TypesetView: &TypesetViewport{Height: 400}}
	if r = l.Load(q); r.Err != nil || r.TypesetView.Chop {
		t.Fatalf("markdown page chopped: %+v", r.TypesetView)
	}
}
