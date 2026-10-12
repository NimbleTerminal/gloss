package document

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"nimbleterminal.dev/gloss/examples"
)

func jxlBox(kind string, payload []byte) []byte {
	b := binary.BigEndian.AppendUint32(nil, uint32(8+len(payload)))
	b = append(b, kind...)
	return append(b, payload...)
}

// jxlBoxed wraps a bare codestream in the JPEG XL ISOBMFF container, with
// any metadata boxes in front of it.
func jxlBoxed(codestream []byte, metadata ...[]byte) []byte {
	out := jxlBox("JXL ", []byte{0x0d, 0x0a, 0x87, 0x0a})
	out = append(out, jxlBox("ftyp", []byte("jxl \x00\x00\x00\x00jxl "))...)
	for _, box := range metadata {
		out = append(out, box...)
	}
	return append(out, jxlBox("jxlc", codestream)...)
}

// The lossless JPEG XL fixture decodes to the PNG's pixels, from a bare
// codestream or a container, whatever the file is called.
func TestJXLDetectionAndExport(t *testing.T) {
	data, err := examples.Files.ReadFile("landscape.jxl")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(data, []byte{0xff, 0x0a}) {
		t.Fatalf("fixture is not a bare codestream: % x", data[:4])
	}
	pngData, err := examples.Files.ReadFile("landscape.png")
	if err != nil {
		t.Fatal(err)
	}
	want, err := png.Decode(bytes.NewReader(pngData))
	if err != nil {
		t.Fatal(err)
	}
	for name, file := range map[string][]byte{"photo.jxl": data, "photo.JXL": data, "undecorated": data, "boxed.jxl": jxlBoxed(data), "boxed": jxlBoxed(data)} {
		path := filepath.Join(t.TempDir(), name)
		if err := os.WriteFile(path, file, 0600); err != nil {
			t.Fatal(err)
		}
		if kind, err := Probe(path, ""); err != nil || kind != "jxl" {
			t.Fatalf("probe %s: %s %v", name, kind, err)
		}
		l := &Loader{}
		r := l.Load(Request{Path: path, Generation: 1})
		l.Close()
		if r.Err != nil {
			t.Fatalf("%s: %v", name, r.Err)
		}
		if r.Image.Bounds() != image.Rect(0, 0, 640, 400) {
			t.Fatal(r.Image.Bounds())
		}
		for _, p := range []image.Point{{0, 0}, {470, 110}, {320, 399}, {639, 300}} {
			if got, want := toRGBA(r.Image.At(p.X, p.Y)), toRGBA(want.At(p.X, p.Y)); got != want {
				t.Fatalf("%s pixel %v: %v, want %v", name, p, got, want)
			}
		}
		out, err := ExportImage(r, 320)
		if err != nil || out.Bounds().Dx() != 320 {
			t.Fatalf("export: %v", err)
		}
	}
	for _, n := range []int{2, 12, 100, len(data) / 2} {
		if img, err := decodeRaster(data[:n]); err == nil {
			t.Fatalf("truncated to %d bytes decoded %v", n, img.Bounds())
		}
	}
}

func TestJXLInfoAndAccept(t *testing.T) {
	fields := loadInfo(t, "../../examples/landscape.jxl")
	expect(t, fields, map[string]string{"Format": "JPEG XL", "Dimensions": "640 × 400 (0.3 MP)", "Color": "8-bit RGBA"})
	if field(fields, "Camera") != "" {
		t.Error("a drawing has no camera")
	}
	images, err := ParseAccept("image/*")
	if err != nil {
		t.Fatal(err)
	}
	data, err := examples.Files.ReadFile("landscape.jxl")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"photo.jxl", "photo", "lie.html"} {
		if err := images.Check(name, data); err != nil {
			t.Fatal(err)
		}
	}
}

// An Exif box carries the TIFF structure behind a 4-byte header offset.
func TestJXLInfoReadsEXIF(t *testing.T) {
	data, err := examples.Files.ReadFile("landscape.jxl")
	if err != nil {
		t.Fatal(err)
	}
	exif := jxlBox("Exif", append([]byte{0, 0, 0, 0}, photoEXIF(binary.LittleEndian)...))
	fields := loadInfo(t, write(t, "photo.jxl", jxlBoxed(data, exif)))
	expect(t, fields, camera)
	expect(t, fields, map[string]string{"Format": "JPEG XL", "Dimensions": "640 × 400 (0.3 MP)"})
}

func toRGBA(c color.Color) color.RGBA {
	return color.RGBAModel.Convert(c).(color.RGBA)
}
