package document

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
	"testing/fstest"

	"github.com/alecthomas/chroma/v2"
	"nimbleterminal.dev/gloss/internal/theme"
)

func TestTypesetThemeRepaintsWithoutRelayout(t *testing.T) {
	for _, tc := range []struct{ path, source string }{
		{"page.md", "# Heading\n\nA paragraph with **bold** and ~~strike~~.\n\n> Quote\n\n| A | B |\n|---|---|\n| 1 | 2 |\n\n~~~go\nfunc hello() string { return \"world\" }\n~~~"},
		{"main.go", "package main\nfunc main() { println(\"hello\") }"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			l := &Loader{Files: fstest.MapFS{tc.path: {Data: []byte(tc.source)}}}
			defer l.Close()
			q := Request{Path: tc.path, Typeset: true, Theme: "light", TypesetView: &TypesetViewport{Height: 900}}
			light := l.Load(q)
			if light.Err != nil {
				t.Fatal(light.Err)
			}
			eng, prep, layout := l.typesetEngine, l.typesetPrepared, l.direct
			cacheBytes, glyphs, ops := eng.cacheBytes, layout.glyphs, len(layout.ops)
			q.Theme, q.Generation = "dark", 1
			dark := l.Load(q)
			if dark.Err != nil {
				t.Fatal(dark.Err)
			}
			if l.typesetEngine != eng || l.typesetPrepared != prep || l.direct != layout || eng.cacheBytes != cacheBytes || layout.glyphs != glyphs || len(layout.ops) != ops {
				t.Fatal("theme change repeated preparation, shaping or layout")
			}
			if *light.TypesetView != *dark.TypesetView {
				t.Fatal("theme moved viewport")
			}
			for _, r := range []Result{light, dark} {
				if got := color.RGBAModel.Convert(r.Image.At(0, 0)); got != theme.Get(r.Theme).Background {
					t.Fatal("wrong background", r.Theme, got)
				}
			}
			for _, name := range []string{"nord", "catppuccin-latte", "dracula"} {
				q.Theme = name
				r := l.Load(q)
				if r.Err != nil || l.direct != layout || eng.cacheBytes != cacheBytes || len(layout.ops) != ops {
					t.Fatal("named theme reshaped", name, r.Err)
				}
				if color.RGBAModel.Convert(r.Image.At(0, 0)) != theme.Get(name).Background {
					t.Fatal("named background", name)
				}
			}
			q.Theme, q.Generation = "auto", 2
			auto := l.Load(q)
			if auto.Err != nil || auto.Theme != "light" {
				t.Fatal("headless auto is not light", auto.Err)
			}
			if !bytes.Equal(auto.Image.(*image.RGBA).Pix, light.Image.(*image.RGBA).Pix) {
				t.Fatal("theme switch changed layout")
			}
		})
	}
}

func TestTypesetDarkSyntaxAndFills(t *testing.T) {
	spans := highlightCode("func hi() string { return \"hello\" }", "go", codeTheme)
	tokens := map[chroma.TokenType]bool{}
	for _, s := range spans {
		if s.token != 0 {
			tokens[s.token] = true
		}
	}
	if len(tokens) < 2 {
		t.Fatal("syntax tokens missing")
	}
	p, err := prepareForTest(testInput(t, "```go\nfunc hi() string { return \"hello\" }\n```\n"))
	if err != nil {
		t.Fatal(err)
	}
	l := testEngine(t).layout(p, 600)
	l.ensure(100000)
	dark := paletteFor("dark")
	img := l.rasterTo(0, l.y, 0, 600, 600, l.y, 1, dark, false)
	var fill *operation
	var keyword *operation
	for i := range l.ops {
		op := &l.ops[i]
		if op.run == nil && op.asset == nil && op.fill == fillCode && fill == nil {
			fill = op
		}
		if op.run != nil && op.token.InCategory(chroma.Keyword) && keyword == nil {
			keyword = op
		}
	}
	if fill == nil || keyword == nil {
		t.Fatal("no code fill or keyword run in the layout")
	}
	if got, want := img.RGBAAt(fill.rect.Min.X, fill.rect.Min.Y), theme.Get("dark").Code; got != want {
		t.Fatalf("code background is %v, want %v", got, want)
	}
	// The pixel of the keyword's run farthest from the code background is its ink.
	bg := theme.Get("dark").Code
	dist := func(c color.RGBA) int {
		d := 0
		for _, v := range [3]int{int(c.R) - int(bg.R), int(c.G) - int(bg.G), int(c.B) - int(bg.B)} {
			d += v * v
		}
		return d
	}
	var far color.RGBA
	for y := keyword.rect.Min.Y; y < keyword.rect.Max.Y; y++ {
		for x := keyword.rect.Min.X; x < keyword.rect.Max.X; x++ {
			if c := img.RGBAAt(x, y); dist(c) > dist(far) {
				far = c
			}
		}
	}
	want := theme.Ink("dark", keyword.token)
	if far != want {
		t.Fatalf("keyword ink is %v, want %v", far, want)
	}
	if want == theme.Get("dark").Foreground || want == theme.Ink("light", keyword.token) {
		t.Fatal("dark keyword ink is not distinct")
	}
}

func TestTypesetThemePreservesImage(t *testing.T) {
	im := image.NewRGBA(image.Rect(0, 0, 20, 20))
	for y := 0; y < 20; y++ {
		for x := 0; x < 20; x++ {
			im.SetRGBA(x, y, color.RGBA{200, 50, 90, 255})
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, im); err != nil {
		t.Fatal(err)
	}
	l := &Loader{Files: fstest.MapFS{"page.md": {Data: []byte("![local](local.png)")}, "local.png": {Data: encoded.Bytes()}}}
	defer l.Close()
	q := Request{Path: "page.md", Typeset: true, Theme: "light", TypesetView: &TypesetViewport{Height: 100}}
	a := l.Load(q)
	if a.Err != nil {
		t.Fatal(a.Err)
	}
	asset := l.typesetPrepared.images["local.png"]
	q.Theme, q.Generation = "dark", 1
	b := l.Load(q)
	if b.Err != nil || l.typesetPrepared.images["local.png"] != asset {
		t.Fatal("image was decoded again", b.Err)
	}
	for y := 50; y < 70; y++ {
		for x := 50; x < 70; x++ {
			if a.Image.At(x, y) != b.Image.At(x, y) {
				t.Fatal("theme recolored embedded image")
			}
		}
	}
}
