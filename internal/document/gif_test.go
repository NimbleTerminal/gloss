package document

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"testing"
	"testing/fstest"
)

var gifPalette = color.Palette{color.Transparent, color.RGBA{255, 0, 0, 255}, color.RGBA{0, 255, 0, 255}, color.RGBA{0, 0, 255, 255}, color.RGBA{255, 255, 0, 255}}

func gifTestData(t *testing.T, loop int) []byte {
	t.Helper()
	frames := []*image.Paletted{
		image.NewPaletted(image.Rect(0, 0, 3, 1), gifPalette),
		image.NewPaletted(image.Rect(0, 0, 2, 1), gifPalette),
		image.NewPaletted(image.Rect(2, 0, 3, 1), gifPalette),
		image.NewPaletted(image.Rect(0, 0, 1, 1), gifPalette),
	}
	copy(frames[0].Pix, []byte{1, 1, 1})
	copy(frames[1].Pix, []byte{0, 2})
	frames[2].Pix[0] = 3
	frames[3].Pix[0] = 4
	var out bytes.Buffer
	if err := gif.EncodeAll(&out, &gif.GIF{Image: frames, Delay: []int{0, 1, 2, 15}, Disposal: []byte{gif.DisposalNone, gif.DisposalPrevious, gif.DisposalBackground, gif.DisposalNone}, LoopCount: loop, Config: image.Config{ColorModel: gifPalette, Width: 3, Height: 1}}); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func assertGIFPixels(t *testing.T, img image.Image, indexes ...int) {
	t.Helper()
	for x, index := range indexes {
		if got, want := color.RGBAModel.Convert(img.At(x, 0)), color.RGBAModel.Convert(gifPalette[index]); got != want {
			t.Fatalf("pixel %d: %v, want %v", x, got, want)
		}
	}
}

func TestGIFLoaderModes(t *testing.T) {
	files := fstest.MapFS{"motion.gif": {Data: gifTestData(t, 0)}}
	loader := &Loader{Files: files}
	defer loader.Close()
	for _, q := range []Request{{}, {Animate: true}, {Animate: true, Preview: true}} {
		q.Path = "motion.gif"
		r := loader.Load(q)
		if r.Err != nil {
			t.Fatal(r.Err)
		}
		if (r.Animation != nil) != (q.Animate && !q.Preview) {
			t.Fatalf("wrong playback mode for %+v", q)
		}
		assertGIFPixels(t, r.Image, 1, 1, 1)
	}
}
