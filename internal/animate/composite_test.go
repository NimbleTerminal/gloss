package animate

import (
	"image"
	"image/color"
	"nimbleterminal.dev/ntpicture/animation"
	"testing"
)

// A window of a composite shows exactly what the full composite shows there,
// frame after frame, and a page laid out again can take over the positions
// its windows have reached.
func TestCompositeWindowMatchesPage(t *testing.T) {
	base := image.NewRGBA(image.Rect(0, 0, 60, 40))
	for y := 0; y < 40; y++ {
		for x := 0; x < 60; x++ {
			base.SetRGBA(x, y, color.RGBA{uint8(x * 4), uint8(y * 6), 90, 255})
		}
	}
	_, player, err := animation.DecodeGIF(gifTestData(t, 0), true)
	if err != nil {
		t.Fatal(err)
	}
	regions := []Region{{Rect: image.Rect(10, 10, 40, 30), Player: player, Destination: "a.gif"}}
	page := NewComposite(base, regions, nil)
	window := image.Rect(5, 15, 45, 40)
	w := page.Window(window, image.Point{})
	if w.Image().Bounds() != image.Rect(0, 0, 40, 25) || len(w.Regions) != 1 || w.Regions[0].Rect != image.Rect(5, -5, 35, 15) {
		t.Fatalf("window geometry: %v %+v", w.Image().Bounds(), w.Regions)
	}
	same := func(step string) {
		full, part := page.Image(), w.Image()
		for y := window.Min.Y; y < window.Max.Y; y++ {
			for x := window.Min.X; x < window.Max.X; x++ {
				if full.At(x, y) != part.At(x-window.Min.X, y-window.Min.Y) {
					t.Fatalf("%s: pixel %d,%d differs: %v vs %v", step, x, y, full.At(x, y), part.At(x-window.Min.X, y-window.Min.Y))
				}
			}
		}
	}
	same("first frame")
	for i := 0; i < 3; i++ {
		_, d1, _ := page.Next()
		_, d2, _ := w.Next()
		if d1 != d2 {
			t.Fatalf("frame %d: delays differ %v %v", i, d1, d2)
		}
		same("after frame")
	}
	if page.Window(image.Rect(50, 0, 60, 10), image.Point{}).Regions != nil {
		t.Fatal("a window away from every region kept one")
	}
	fresh := NewComposite(base, regions, nil)
	if fresh.Delay() == w.Delay() {
		t.Skip("test GIF reached the same delay again")
	}
	carried := fresh.WithPositions(w)
	if carried.Delay() != w.Delay() || carried.Image() != fresh.Image() {
		t.Fatal("positions were not carried without recomposing")
	}
	if next := NewComposite(base, regions, carried); next.Image().At(12, 12) != page.Image().At(12, 12) {
		t.Fatal("a relayout from the carried positions shows a different frame")
	}
}

// A window at another size resamples the base once and places each region at
// that scale, snapping to its frames' own size so they are copied in.
func TestCompositeWindowAtOwnResolution(t *testing.T) {
	base := image.NewRGBA(image.Rect(0, 0, 90, 60))
	for y := 0; y < 60; y++ {
		for x := 0; x < 90; x++ {
			base.SetRGBA(x, y, color.RGBA{uint8(x), uint8(y), 0, 255})
		}
	}
	_, player, _ := animation.DecodeGIF(gifTestData(t, 0), true)
	own := player.Image().Bounds().Size() // 3×1
	// The page shows the GIF at 1.5× its pixels: 3×1 drawn in a 5×2 (rounded) rectangle.
	page := NewComposite(base, []Region{{Rect: image.Rect(30, 15, 35, 17), Player: player, Destination: "a.gif"}}, nil)
	w := page.Window(image.Rect(15, 0, 60, 30), image.Pt(30, 20))
	if w.Image().Bounds().Size() != image.Pt(30, 20) {
		t.Fatalf("window size %v", w.Image().Bounds())
	}
	if len(w.Regions) != 1 || w.Regions[0].Rect != image.Rect(10, 10, 10+own.X, 10+own.Y) {
		t.Fatalf("region not at its own size and place: %+v", w.Regions)
	}
	if c := w.Image().At(0, 0); c != base.At(15, 0) && c != base.At(16, 1) {
		t.Fatalf("base not resampled from the window: %v", c)
	}
	if w.Image().At(10, 10) != player.Image().At(0, 0) {
		t.Fatal("frame not copied at its own pixels")
	}
}
