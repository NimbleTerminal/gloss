package main

import (
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQRFile(t *testing.T) {
	path, remove, err := qrFile("https://nimbleterminal.dev/gloss/")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != "qr.png" {
		t.Fatalf("path=%s", path)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(f)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	// Square, sharp enough to scale up, within the default export edge.
	if b := img.Bounds(); b.Dx() != b.Dy() || b.Dx() < 400 || b.Dx() > 1536 {
		t.Fatalf("bounds=%v", b)
	}
	remove()
	if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
		t.Fatalf("the folder outlived remove: %v", err)
	}
	if _, _, err := qrFile(strings.Repeat("x", 8000)); err == nil || !strings.Contains(err.Error(), "--qr") {
		t.Fatalf("an oversized text: err=%v", err)
	}
}

func TestParseQR(t *testing.T) {
	opts, _, err := parse([]string{"--serve", "--qr", "https://x.dev/"}, io.Discard)
	if err != nil || opts.QR != "https://x.dev/" || len(opts.Files) != 0 {
		t.Fatalf("opts.QR=%q files=%v err=%v", opts.QR, opts.Files, err)
	}
	for args, want := range map[string]string{
		"--qr=":                             "needs the text",
		"--qr x a.png":                      "no input files",
		"--qr x --glob images":              "no input files",
		"--qr x --pick":                     "--pick",
		"--qr x --pick-web":                 "--pick",
		"--qr x --output a.png --prompt hi": "--prompt cannot",
	} {
		if _, _, err := parse(strings.Fields(args), io.Discard); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: err=%v, want %q", args, err, want)
		}
	}
}
