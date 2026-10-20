package document

import (
	"context"
	"errors"
	"image"
	"strings"
	"testing"
	"testing/fstest"
)

func TestTypesetViewportProgressAndCancellation(t *testing.T) {
	var l Loader
	l.Files = fstest.MapFS{"x.md": &fstest.MapFile{Data: []byte(strings.Repeat("A paragraph with **bold** text.\n\n", 100))}}
	defer l.Close()
	q := Request{Theme: testTheme("light"), Path: "x.md", MarkdownMode: "typeset", TypesetView: &TypesetViewport{Height: 300}, DPI: 150}
	r := l.Load(q)
	if r.Err != nil {
		t.Fatal(r.Err)
	}
	if r.TypesetDone || l.direct.Done() || l.direct.Laid() >= len(l.typesetDocument.Blocks) {
		t.Fatal("first viewport completed whole document")
	}
	first, engine := l.direct, l.typesetEngine
	q.TypesetView = &TypesetViewport{Top: 300, Height: 400}
	r = l.Load(q)
	if r.Err != nil || l.direct != first || l.typesetEngine != engine || r.TypesetView.Top != 300 {
		t.Fatalf("scroll did not reuse layout: %v", r.Err)
	}
	q.TypesetView = &TypesetViewport{End: true, Height: 400, Zoom: 1}
	r = l.Load(q)
	if r.Err != nil || !r.TypesetDone || r.TypesetView.Top+r.TypesetView.Height != r.TypesetExtent {
		t.Fatalf("End missed bottom: %+v", r.TypesetView)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	q.Context = ctx
	if r = l.Load(q); !errors.Is(r.Err, context.Canceled) {
		t.Fatalf("ignored cancellation: %v", r.Err)
	}
	q.Context = nil
	q.TypesetView = &TypesetViewport{Height: 300}
	q.DPI = 300
	r = l.Load(q)
	if r.Err != nil || r.Image.Bounds().Dx() != 1600 || r.Image.Bounds().Dy() != 600 {
		t.Fatalf("DPI raster scaling: %v", r.Err)
	}
}

func TestTypesetLayoutLimits(t *testing.T) {
	for name, source := range map[string]string{
		"paragraph":  strings.Repeat("a", 16385),
		"code-lines": "```\n" + strings.Repeat("\n", 8193) + "```\n",
	} {
		t.Run(name, func(t *testing.T) {
			var l Loader
			l.Files = fstest.MapFS{"x.md": &fstest.MapFile{Data: []byte(source)}}
			defer l.Close()
			r := l.Load(Request{Theme: testTheme("light"), Path: "x.md", MarkdownMode: "typeset"})
			if r.Err == nil {
				t.Fatal("accepted oversized layout unit")
			}
		})
	}
}

// A viewer names the device pixels its picture will fill, so the raster is
// exactly that size and the terminal picture needs no resample. The reading
// scale is unchanged: the same layout units fill more or fewer pixels.
func TestTypesetViewportRastersToRequestedPixels(t *testing.T) {
	var l Loader
	l.Files = fstest.MapFS{"x.md": &fstest.MapFile{Data: []byte(strings.Repeat("A paragraph with **bold** text.\n\n", 40))}}
	defer l.Close()
	for _, tc := range []struct {
		pixels image.Point
		zoom   int
	}{{image.Pt(2400, 1234), 0}, {image.Pt(1001, 333), 0}, {image.Pt(2400, 1234), 1}} {
		q := Request{Theme: testTheme("light"), Path: "x.md", MarkdownMode: "typeset", TypesetWidth: 960, DPI: 150, TypesetView: &TypesetViewport{Top: 100, Height: 1000, Zoom: tc.zoom, Pixels: tc.pixels}}
		r := l.Load(q)
		if r.Err != nil {
			t.Fatal(r.Err)
		}
		if got := r.Image.Bounds().Size(); got != tc.pixels {
			t.Fatalf("pixels %v zoom %d: raster %v", tc.pixels, tc.zoom, got)
		}
		if r.TypesetView.Pixels != tc.pixels || r.TypesetView.Zoom != tc.zoom {
			t.Fatalf("viewport did not echo pixels: %+v", r.TypesetView)
		}
	}
	q := Request{Theme: testTheme("light"), Path: "x.md", MarkdownMode: "typeset", TypesetWidth: 960, DPI: 150, TypesetView: &TypesetViewport{Height: 1000}}
	if r := l.Load(q); r.Err != nil || r.Image.Bounds().Size() != image.Pt(2000, 1000) {
		t.Fatalf("zero pixels should keep the layout scale: %v", r.Image.Bounds())
	}
	q.TypesetView.Pixels = image.Pt(8000, 8000)
	if r := l.Load(q); r.Err == nil {
		t.Fatal("raster over the pixel budget was accepted")
	}
}
