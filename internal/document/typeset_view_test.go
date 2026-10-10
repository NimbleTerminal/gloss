package document

import (
	"context"
	"errors"
	"strings"
	"testing"
	"testing/fstest"
)

func TestTypesetViewportProgressAndCancellation(t *testing.T) {
	var l Loader
	l.Files = fstest.MapFS{"x.md": &fstest.MapFile{Data: []byte(strings.Repeat("A paragraph with **bold** text.\n\n", 100))}}
	defer l.Close()
	q := Request{Path: "x.md", MarkdownMode: "typeset", TypesetView: &TypesetViewport{Height: 300}, DPI: 150}
	r := l.Load(q)
	if r.Err != nil {
		t.Fatal(r.Err)
	}
	if r.TypesetDone || l.direct.next >= len(l.typesetPrepared.blocks) {
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
			r := l.Load(Request{Path: "x.md", MarkdownMode: "typeset"})
			if r.Err == nil {
				t.Fatal("accepted oversized layout unit")
			}
		})
	}
}
