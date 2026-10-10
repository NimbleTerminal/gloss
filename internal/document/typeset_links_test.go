package document

import (
	"image"
	"strings"
	"testing"
	"testing/fstest"
)

func TestTypesetLinksWrapReflowAndViewport(t *testing.T) {
	const url = "https://example.org/"
	source := "# Heading\n\n[" + strings.Repeat("linked words ", 24) + "](" + url + ")\n\n<https://example.com/>\n\n| Table |\n| --- |\n| [cell](https://example.net/) |\n\n[![image](local.png)](https://example.edu/)"

	// Use the original fixture image for the linked figure.
	l := &Loader{Files: fstest.MapFS{"doc.md": {Data: []byte(source)}, "local.png": {Data: pngBytes(t)}}}
	defer l.Close()
	q := Request{Path: "doc.md", Typeset: true, TypesetWidth: 192, TypesetView: &TypesetViewport{Height: 1200}}
	first := l.Load(q)
	if first.Err != nil {
		t.Fatal(first.Err)
	}
	counts := map[string]int{}
	for _, link := range first.Links {
		counts[link.URL]++
		if link.Rect.Empty() || !link.Rect.In(first.Image.Bounds()) {
			t.Fatal("bad link rect", link)
		}
	}
	if counts[url] < 2 {
		t.Fatal("link did not wrap", counts)
	}
	for _, url := range []string{"https://example.com/", "https://example.net/", "https://example.edu/"} {
		if counts[url] == 0 {
			t.Fatal("missing link", url)
		}
	}
	q.TypesetView = &TypesetViewport{Top: 100, Height: 150, Zoom: 1, Pixels: image.Pt(384, 300)}
	next := l.Load(q)
	if next.Err != nil {
		t.Fatal(next.Err)
	}
	for _, link := range next.Links {
		if !link.Rect.In(next.Image.Bounds()) {
			t.Fatal("link not clipped after scroll/zoom")
		}
	}
	q.TypesetWidth = 384
	q.TypesetView = &TypesetViewport{Height: 1200}
	wide := l.Load(q)
	if wide.Err != nil {
		t.Fatal(wide.Err)
	}
	n := 0
	for _, link := range wide.Links {
		if link.URL == url {
			n++
		}
	}
	if n >= counts[url] {
		t.Fatal("reflow kept old link rectangles", n, counts[url])
	}
}

func TestTypesetShapeCachePreservesLinkDestinations(t *testing.T) {
	e := testEngine(t)
	a, b := []span{{text: "same", href: "https://a.example/"}}, []span{{text: "same", href: "https://b.example/"}}
	if e.shape(a, 25) != e.shape(b, 25) {
		t.Fatal("link URL unnecessarily changed cached shaping")
	}
	l := &layout{e: e}
	first, _ := l.text(a, 25, 0, 0, 400)
	second, _ := l.text(b, 25, 0, 0, 400)
	if first[0].href != a[0].href || second[0].href != b[0].href {
		t.Fatal("cache reused wrong link")
	}
}
