package document

import (
	"image"
	"image/color"
	"image/draw"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/NimbleTerminal/gloss/internal/theme"
)

// Glyphs are drawn from cached masks: a second raster of the same view hits
// the cache for every glyph and is identical, the result stays within a
// quarter pixel of drawing each run straight from its outlines, and the cache
// stays within its budget.
func TestTypesetGlyphMasks(t *testing.T) {
	var l Loader
	l.Files = fstest.MapFS{"x.md": &fstest.MapFile{Data: []byte("# Glyph masks\n\n" + strings.Repeat("Café and *a paragraph* with `code`.\n\n", 30))}}
	defer l.Close()
	q := Request{Path: "x.md", MarkdownMode: "typeset", TypesetWidth: 400, DPI: 150, TypesetView: &TypesetViewport{Height: 600, Pixels: image.Pt(1000, 720)}}
	r := l.Load(q)
	if r.Err != nil {
		t.Fatal(r.Err)
	}
	e := l.typesetEngine
	if e.glyphMisses == 0 || e.glyphHits == 0 || len(e.glyphs) == 0 {
		t.Fatalf("cache unused: %d misses, %d hits, %d masks", e.glyphMisses, e.glyphHits, len(e.glyphs))
	}
	misses := e.glyphMisses
	again := l.Load(q)
	if again.Err != nil || e.glyphMisses != misses {
		t.Fatalf("a second raster missed %d more glyphs", e.glyphMisses-misses)
	}
	first, second := r.Image.(*image.RGBA), again.Image.(*image.RGBA)
	for i := range first.Pix {
		if first.Pix[i] != second.Pix[i] {
			t.Fatal("cached raster differs from the first")
		}
	}
	// Against go-text drawing every run straight from its outlines.
	direct := image.NewRGBA(first.Bounds())
	draw.Draw(direct, direct.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	lay := l.direct
	v := r.TypesetView
	scale := float64(v.Pixels.X) / float64(lay.width)
	inkRuns := 0
	for _, op := range lay.ops {
		if op.run == nil {
			continue
		}
		rr := op.rect.Sub(image.Pt(v.Left, v.Top))
		rr = image.Rect(int(float64(rr.Min.X)*scale+0.5), int(float64(rr.Min.Y)*scale+0.5), int(float64(rr.Max.X)*scale+0.5), int(float64(rr.Max.Y)*scale+0.5))
		if !rr.Overlaps(direct.Bounds()) {
			continue
		}
		inkRuns++
		ink := theme.Get(e.theme).Foreground
		if op.ink.A != 0 {
			ink = op.ink
		}
		if e.theme == "dark" && op.darkInk.A != 0 {
			ink = op.darkInk
		}
		e.renderer.FontSize, e.renderer.PixScale, e.renderer.Color = float32(op.size), float32(scale), ink
		pad := int(float64(op.size)*scale) + 1
		tile := image.NewRGBA(image.Rect(0, 0, rr.Dx()+2*pad, rr.Dy()+2*pad))
		e.renderer.DrawShapedRunAt(*op.run, tile, pad, pad+int(float64(op.baseline-v.Top)*scale+0.5)-rr.Min.Y)
		draw.Draw(direct, tile.Bounds().Add(rr.Min.Sub(image.Pt(pad, pad))), tile, image.Point{}, draw.Over)
	}
	if inkRuns == 0 {
		t.Fatal("no text runs in view")
	}
	var sum, worst, ink int
	for i := 0; i < len(first.Pix); i += 4 {
		d := 0
		for c := 0; c < 3; c++ {
			d = max(d, int(first.Pix[i+c])-int(direct.Pix[i+c]), int(direct.Pix[i+c])-int(first.Pix[i+c]))
		}
		if first.Pix[i] != 255 || direct.Pix[i] != 255 {
			ink++
			sum += d
		}
		worst = max(worst, d)
	}
	if ink == 0 || sum/ink > 12 || worst > 160 {
		t.Fatalf("cached glyphs stray from direct rendering: mean %d over %d ink pixels, worst %d", sum/ink, ink, worst)
	}
	// The budget is kept on insertion: a new scale misses on every glyph and
	// empties the cache as needed, so it holds at most the budget plus the
	// one mask that overflowed it.
	e.glyphBudget = 1 << 12
	q.TypesetWidth = 420
	if again := l.Load(q); again.Err != nil || e.glyphMisses == misses || e.glyphBytes > 2*e.glyphBudget {
		t.Fatalf("cache over budget: %d bytes after %d misses", e.glyphBytes, e.glyphMisses-misses)
	}
}
