package document

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testEngine(t *testing.T) *engine {
	t.Helper()
	e, err := newEngine()
	if err != nil {
		t.Fatal(err)
	}
	return e
}
func testInput(t *testing.T, source string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "input.md")
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}
func TestFixtureAndProgressiveLayout(t *testing.T) {
	p, err := prepareForTest("../../examples/typeset.md")
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]bool{}
	for _, b := range p.blocks {
		kinds[b.kind] = true
	}
	for _, kind := range []string{"heading", "paragraph", "quote", "code", "table", "image"} {
		if !kinds[kind] {
			t.Errorf("missing %s", kind)
		}
	}
	if p.images["landscape.png"] == nil || p.images["missing-typeset-image.png"] != nil {
		t.Fatal("local image/placeholder")
	}
	e := testEngine(t)
	l := e.layout(p, 800)
	img := l.viewport(0, 200)
	if img.Bounds() != image.Rect(0, 0, 800, 200) || l.next == len(p.blocks) {
		t.Fatal("first viewport laid out the whole fixture")
	}
	firstOps := len(l.ops)
	l.ensure(100000)
	if l.next != len(p.blocks) || len(l.ops) <= firstOps {
		t.Fatal("remaining layout did not finish")
	}
	full := l.viewport(0, l.y+50)
	if full.RGBAAt(50, 50) == (color.RGBA{}) {
		t.Fatal("transparent canvas")
	}
	// A scrolled viewport must agree pixel-for-pixel with the same full layout.
	top, height := 317, 240
	scrolled := l.viewport(top, height)
	for y := 0; y < height; y++ {
		for x := 0; x < 800; x++ {
			if full.RGBAAt(x, y+top) != scrolled.RGBAAt(x, y) {
				t.Fatalf("scroll differs at %d,%d", x, y)
			}
		}
	}
	// Cached shaping must produce the same narrow image as a fresh engine.
	cachedNarrow := e.layout(p, 600).viewport(0, 900)
	freshNarrow := testEngine(t).layout(p, 600).viewport(0, 900)
	if !bytes.Equal(cachedNarrow.Pix, freshNarrow.Pix) {
		t.Fatal("wrapping mutated cached shaping across widths")
	}
	small := e.layout(p, 400)
	small.ensure(100000)
	if small.y <= l.y {
		t.Fatal("narrow layout should wrap to more lines")
	}
	again := l.viewport(0, 200)
	if !bytes.Equal(img.Pix, again.Pix) {
		t.Fatal("resize/remaining layout mutated existing glyph runs")
	}
}
func TestTextWrappingPreservesUnicodeAndStyles(t *testing.T) {
	e := testEngine(t)
	spans := []span{{text: "Café naïve "}, {text: "résumé ", style: bold}, {text: "e\u0301lan and a long sentence", style: italic}}
	s := e.shape(spans, 25)
	for _, run := range s.runs {
		for _, g := range run.Glyphs {
			if g.GlyphID == 0 {
				t.Fatal("missing Latin glyph")
			}
		}
	}
	l := e.layout(&prepared{}, 400)
	ops, h := l.text(spans, 25, 0, 0, 120)
	if h <= 37 {
		t.Fatal("text did not wrap")
	}
	covered := make([]bool, len(s.text))
	styles := map[int]bool{}
	for _, op := range ops {
		styles[op.style] = true
		for i := op.run.Runes.Offset; i < op.run.Runes.Offset+op.run.Runes.Count; i++ {
			if i >= len(covered) {
				t.Fatal("run out of bounds")
			}
			covered[i] = true
		}
	}
	for i, r := range s.text {
		if r != ' ' && !covered[i] {
			t.Fatalf("lost rune %q at %d", r, i)
		}
	}
	if !styles[bold] || !styles[italic] {
		t.Fatal("lost mixed styles")
	}
	_, h = l.text([]span{{text: "one\n\nthree", style: mono}}, 25, 0, 0, 300)
	if h != 3*37 {
		t.Fatalf("hard/empty line height %d", h)
	}
	before := len(e.cache)
	e.shape(spans, 25)
	if len(e.cache) != before {
		t.Fatal("identical shaping was not reused")
	}
	e.shape([]span{{text: "changed"}}, 25)
	if len(e.cache) <= before {
		t.Fatal("content change reused stale text")
	}
}
func TestPreparationBudgetsAndOfflineImages(t *testing.T) {
	cases := []struct{ name, source string }{
		{"source", strings.Repeat("a", (2<<20)+1)},
		{"utf8", string([]byte{0xff})},
		{"refs", strings.Repeat("![x](https://example.invalid/x.png)\n\n", 33)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := prepareForTest(testInput(t, tc.source)); err == nil {
				t.Fatal("accepted over-budget input")
			}
		})
	}
	p, err := prepareForTest(testInput(t, strings.Repeat("![x](https://example.invalid/x.png)\n\n", 32)))
	if err != nil || len(p.images) != 0 || len(p.blocks) != 32 {
		t.Fatalf("remote references must be placeholders: %v", err)
	}
	path := testInput(t, "![huge](huge.png)")
	var b bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	if err = png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	data := b.Bytes()
	binary.BigEndian.PutUint32(data[16:20], 8192)
	binary.BigEndian.PutUint32(data[20:24], 8192)
	binary.BigEndian.PutUint32(data[29:33], crc32.ChecksumIEEE(data[12:29]))
	if err = os.WriteFile(filepath.Join(filepath.Dir(path), "huge.png"), data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = prepareForTest(path); err == nil || !strings.Contains(err.Error(), "16 Mi") {
		t.Fatalf("pixel budget not rejected before decode: %v", err)
	}
}

func prepareForTest(path string) (*prepared, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	_, root, assets, err := prepareTypeset(Request{Path: path}, data, nil)
	if err != nil {
		return nil, err
	}
	p := &prepared{images: map[string]image.Image{}}
	for name, a := range assets {
		if a != nil {
			p.images[name] = a.image
		}
	}
	p.walk(root, data, 0)
	return p, nil
}
