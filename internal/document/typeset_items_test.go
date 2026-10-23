package document

import (
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const itemsSource = "![a](x.png)\n\n[l](https://e.org/)\n\n|h|i|\n|-|-|\n|1|2|\n\n[^1]\n\n[^1]: note"

// typesetItemsPage typesets itemsSource beside x.png, with the focus given.
func typesetItemsPage(t *testing.T, focus []image.Rectangle) (Result, string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "x.png"), pngBytes(t), 0600); err != nil {
		t.Fatal(err)
	}
	doc := filepath.Join(dir, "doc.md")
	if err := os.WriteFile(doc, []byte(itemsSource), 0600); err != nil {
		t.Fatal(err)
	}
	l := &Loader{}
	t.Cleanup(func() { _ = l.Close() })
	r := l.Load(Request{Theme: testTheme("light"), Path: doc, Typeset: true, TypesetWidth: 384, TypesetView: &TypesetViewport{Height: 1500}, TypesetFocus: focus})
	if r.Err != nil {
		t.Fatal(r.Err)
	}
	return r, dir
}

func TestTypesetItemsMarkdown(t *testing.T) {
	r, dir := typesetItemsPage(t, nil)
	items := r.TypesetItems
	if len(items) != 3 || items[0].Kind != TypesetPicture || items[1].Kind != TypesetLinkItem || items[2].Kind != TypesetTable {
		t.Fatalf("items %+v", items)
	}
	if want := filepath.Join(dir, "x.png"); items[0].Path != want || items[0].Image == nil || items[0].Destination != "x.png" {
		t.Fatalf("picture %+v, want path %q", items[0], want)
	}
	if len(items[1].Rects) == 0 || items[1].URL != "https://e.org/" {
		t.Fatalf("link %+v", items[1])
	}
	for _, rect := range items[1].Rects {
		if rect.Min.Y < 0 || rect.Empty() {
			t.Fatalf("link rect %v", rect)
		}
	}
	if c := items[2].Cells; len(c) != 2 || strings.Join(c[0], ",") != "h,i" || strings.Join(c[1], ",") != "1,2" {
		t.Fatalf("cells %q", c)
	}
	for _, it := range items {
		if _, note := FootnotePeer(it.URL); note || strings.Contains(it.URL, "#fn") {
			t.Fatalf("footnote item %+v", it)
		}
		if len(it.Rects) == 0 {
			t.Fatalf("item without rects %+v", it)
		}
	}
	// The items are in reading order down the page.
	if !(items[0].Rects[0].Min.Y < items[1].Rects[0].Min.Y && items[1].Rects[0].Min.Y < items[2].Rects[0].Min.Y) {
		t.Fatalf("order %v %v %v", items[0].Rects, items[1].Rects, items[2].Rects)
	}
}

func TestTypesetFocusOutline(t *testing.T) {
	plain, _ := typesetItemsPage(t, nil)
	link := plain.TypesetItems[1].Rects
	focused, _ := typesetItemsPage(t, link)
	if len(focused.TypesetFocus) != len(link) {
		t.Fatalf("focus echoed %v", focused.TypesetFocus)
	}
	a, b := plain.Image, focused.Image
	if a.Bounds() != b.Bounds() {
		t.Fatalf("bounds %v %v", a.Bounds(), b.Bounds())
	}
	// The raster is at the layout scale with this request.
	near := func(p image.Point) bool {
		for _, r := range link {
			if p.In(r.Inset(-6)) {
				return true
			}
		}
		return false
	}
	changed := 0
	for y := a.Bounds().Min.Y; y < a.Bounds().Max.Y; y++ {
		for x := a.Bounds().Min.X; x < a.Bounds().Max.X; x++ {
			r1, g1, b1, _ := a.At(x, y).RGBA()
			r2, g2, b2, _ := b.At(x, y).RGBA()
			if r1 == r2 && g1 == g2 && b1 == b2 {
				continue
			}
			changed++
			if !near(image.Pt(x, y)) {
				t.Fatalf("pixel %d,%d changed away from the link %v", x, y, link)
			}
		}
	}
	if changed == 0 {
		t.Fatal("no outline drawn")
	}
}
