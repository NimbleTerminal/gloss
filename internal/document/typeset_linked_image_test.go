package document

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"testing"
	"testing/fstest"
)

func TestTypesetLinkedImageCompositesLikeStandalone(t *testing.T) {
	var pngData, gifData bytes.Buffer
	pic := image.NewNRGBA(image.Rect(0, 0, 8, 6))
	for y := 0; y < 6; y++ {
		for x := 0; x < 8; x++ {
			pic.SetNRGBA(x, y, color.NRGBA{R: 220, G: 80, B: 40, A: 128})
		}
	}
	if err := png.Encode(&pngData, pic); err != nil {
		t.Fatal(err)
	}
	palette := color.Palette{color.RGBA{R: 255, A: 255}, color.RGBA{G: 255, A: 255}}
	first := image.NewPaletted(image.Rect(0, 0, 8, 6), palette)
	second := image.NewPaletted(first.Bounds(), palette)
	for i := range second.Pix {
		second.Pix[i] = 1
	}
	if err := gif.EncodeAll(&gifData, &gif.GIF{Image: []*image.Paletted{first, second}, Delay: []int{10, 10}}); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{"pic.png": pngData.Bytes(), "pic.gif": gifData.Bytes()} {
		for _, theme := range []string{"light", "dark"} {
			t.Run(name+"/"+theme, func(t *testing.T) {
				files := fstest.MapFS{"docs/" + name: {Data: data}}
				render := func(source string) *image.RGBA {
					files["docs/page.md"] = &fstest.MapFile{Data: []byte(source)}
					l := &Loader{Files: files}
					defer l.Close()
					r := l.Load(Request{Path: "docs/page.md", Typeset: true, Theme: theme, TypesetView: &TypesetViewport{Height: 150}})
					if r.Err != nil {
						t.Fatal(r.Err)
					}
					if len(l.typesetPrepared.blocks) != 1 || l.typesetPrepared.blocks[0].kind != "image" || l.typesetPrepared.blocks[0].asset == nil {
						t.Fatal("image replaced with text")
					}
					return r.Image.(*image.RGBA)
				}
				plain := render("![local](" + name + ")")
				linked := render("[![local](" + name + ")](https://example.invalid/never-fetch)")
				html := render(`<p><a href="./next.md"><img src="` + name + `" alt="local"></a></p>`)
				if !bytes.Equal(plain.Pix, html.Pix) {
					t.Fatal("HTML image differs from Markdown")
				}
				if !bytes.Equal(plain.Pix, linked.Pix) {
					t.Fatal("link wrapper changed composition")
				}
				if linked.RGBAAt(51, 51) == linked.RGBAAt(0, 0) {
					t.Fatal("image not composited")
				}
				if name == "pic.gif" && linked.RGBAAt(51, 51) != (color.RGBA{R: 255, A: 255}) {
					t.Fatal("GIF must show first frame")
				}
			})
		}
	}
}

func TestTypesetLinkedMissingImagePlaceholder(t *testing.T) {
	l := &Loader{Files: fstest.MapFS{"page.md": {Data: []byte("[![missing](no.png)](https://example.invalid)")}}}
	defer l.Close()
	r := l.Load(Request{Path: "page.md", Typeset: true})
	if r.Err != nil || len(l.typesetPrepared.blocks) != 1 || l.typesetPrepared.blocks[0].kind != "image" || l.typesetPrepared.blocks[0].asset != nil {
		t.Fatal("missing image should keep its placeholder", r.Err)
	}
}

func TestTypesetAnimatedImageIsReservedRectangle(t *testing.T) {
	var gifData bytes.Buffer
	palette := color.Palette{color.RGBA{R: 255, A: 255}, color.RGBA{B: 255, A: 255}}
	first := image.NewPaletted(image.Rect(0, 0, 40, 30), palette)
	second := image.NewPaletted(first.Bounds(), palette)
	for i := range second.Pix {
		second.Pix[i] = 1
	}
	if err := gif.EncodeAll(&gifData, &gif.GIF{Image: []*image.Paletted{first, second}, Delay: []int{10, 10}}); err != nil {
		t.Fatal(err)
	}
	files := fstest.MapFS{
		"page.md":  {Data: []byte("![a](anim.gif)\n")},
		"anim.gif": {Data: gifData.Bytes()},
	}
	load := func(animate bool) Result {
		l := &Loader{Files: files}
		defer l.Close()
		r := l.Load(Request{Path: "page.md", Typeset: true, Animate: animate, TypesetView: &TypesetViewport{Height: 400}})
		if r.Err != nil {
			t.Fatal(r.Err)
		}
		return r
	}
	animated := load(true)
	if len(animated.Animated) != 1 || animated.Animated[0].Destination != "anim.gif" {
		t.Fatalf("Animated = %+v, want one region for anim.gif", animated.Animated)
	}
	if animated.AnimationBase == nil {
		t.Fatal("AnimationBase is nil")
	}
	rect := animated.Animated[0].Rect
	center := image.Pt((rect.Min.X+rect.Max.X)/2, (rect.Min.Y+rect.Max.Y)/2)
	want := color.RGBAModel.Convert(paletteFor("light").background)
	if got := color.RGBAModel.Convert(animated.AnimationBase.At(center.X, center.Y)); got != want {
		t.Fatalf("AnimationBase at %v = %v, want background %v", center, got, want)
	}
	still := load(false)
	if len(still.Animated) != 0 {
		t.Fatalf("still page reports Animated regions: %+v", still.Animated)
	}
	if got := color.RGBAModel.Convert(still.Image.At(center.X, center.Y)); got != color.RGBAModel.Convert(palette[0]) {
		t.Fatalf("still Image at %v = %v, want first frame %v", center, got, palette[0])
	}
}
