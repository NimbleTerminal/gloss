package main

import (
	"bytes"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/NimbleTerminal/gloss/internal/theme"
)

func TestThemeExportCLI(t *testing.T) {
	path := filepath.Join(t.TempDir(), "note.txt")
	if err := os.WriteFile(path, []byte("Café and literal *text*"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"auto", "light", "dark"} {
		t.Run(name, func(t *testing.T) {
			var out, errs bytes.Buffer
			opts, _, err := parse([]string{"--typeset", "--theme", name, "--output", "-", "--max-edge", "128", path}, &out)
			if err != nil {
				t.Fatal(err)
			}
			if err := exportFiles(opts, &out, &errs); err != nil {
				t.Fatal(err)
			}
			im, err := png.Decode(bytes.NewReader(out.Bytes()))
			if err != nil {
				t.Fatal(err)
			}
			if color.RGBAModel.Convert(im.At(0, 0)) != theme.Get(name).Background {
				t.Fatal("wrong export theme")
			}
		})
	}
	var out bytes.Buffer
	if _, _, err := parse([]string{"--theme", "invalid"}, &out); err == nil {
		t.Fatal("accepted invalid theme")
	}
}
