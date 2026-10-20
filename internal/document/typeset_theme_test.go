package document

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"slices"
	"testing"
	"testing/fstest"

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
			q := Request{Path: tc.path, Typeset: true, Theme: testTheme("light"), TypesetView: &TypesetViewport{Height: 900}}
			light := l.Load(q)
			if light.Err != nil {
				t.Fatal(light.Err)
			}
			eng, doc, layout := l.typesetEngine, l.typesetDocument, l.direct
			starts := layout.Starts()
			q.Theme, q.Generation = testTheme("dark"), 1
			dark := l.Load(q)
			if dark.Err != nil {
				t.Fatal(dark.Err)
			}
			if l.typesetEngine != eng || l.typesetDocument != doc || l.direct != layout || !slices.Equal(layout.Starts(), starts) {
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
				q.Theme = testTheme(name)
				r := l.Load(q)
				if r.Err != nil || l.direct != layout || !slices.Equal(layout.Starts(), starts) {
					t.Fatal("named theme reshaped", name, r.Err)
				}
				if color.RGBAModel.Convert(r.Image.At(0, 0)) != theme.Get(name).Background {
					t.Fatal("named background", name)
				}
			}
			q.Theme, q.Generation = testTheme(theme.Resolve("auto")), 2 // As exportRequest resolves it.
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
	q := Request{Path: "page.md", Typeset: true, Theme: testTheme("light"), TypesetView: &TypesetViewport{Height: 100}}
	a := l.Load(q)
	if a.Err != nil {
		t.Fatal(a.Err)
	}
	asset := l.typesetImages["local.png"]
	q.Theme, q.Generation = testTheme("dark"), 1
	b := l.Load(q)
	if b.Err != nil || l.typesetImages["local.png"] != asset {
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
