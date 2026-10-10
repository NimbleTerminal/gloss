package main

import (
	"bytes"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestTypesetExportCLI(t *testing.T) {
	var out, errs bytes.Buffer
	opts, _, err := parse([]string{"--markdown", "typeset", "--page", "all", "--output-dir", t.TempDir(), "--max-edge", "256", "../../examples/typeset.md"}, &out)
	if err != nil {
		t.Fatal(err)
	}
	if err = exportFiles(opts, &out, &errs); err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob(filepath.Join(opts.OutputDir, "*.png"))
	if err != nil || len(files) < 2 {
		t.Fatalf("expected multiple page PNGs: %v %v", files, err)
	}
	for _, name := range files {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		cfg, err := png.DecodeConfig(bytes.NewReader(data))
		if err != nil || max(cfg.Width, cfg.Height) != 256 {
			t.Fatalf("PNG %v %v", cfg, err)
		}
	}
	for _, args := range [][]string{{"--markdown", "bad"}, {"--markdown", "typeset", "--menu", "--output", "x.png", "x.md"}} {
		if _, _, err := parse(args, &out); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	opts, _, err = parse([]string{"--markdown", "typeset", "--type", "markdown", "--page", "999", "--output", "-", "../../examples/typeset.md"}, &out)
	if err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err = exportFiles(opts, &out, &errs); err == nil || out.Len() != 0 {
		t.Fatal("out-of-range page exported")
	}
}

func TestTypesetDoesNotChangeTextExtraction(t *testing.T) {
	var out, errs bytes.Buffer
	opts, _, err := parse([]string{"--typeset", "--text", "../../examples/typeset.md"}, &out)
	if err != nil {
		t.Fatal(err)
	}
	if err = textFiles(opts, &out, &errs); err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(opts.Files[0])
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bytes.TrimSpace(out.Bytes()), bytes.TrimSpace(original)) {
		t.Fatal("--text should retain Markdown source")
	}
}

func TestTypesetTextExportCLI(t *testing.T) {
	for _, filename := range []string{"plain.txt", "main.go", "data.json"} {
		t.Run(filename, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, filename)
			content := "# Literal *text*"
			if filename == "main.go" {
				content = "package main\nfunc main() {}"
			}
			if filename == "data.json" {
				content = "{\"a\": 1}"
			}
			if err := os.WriteFile(path, []byte(content), 0600); err != nil {
				t.Fatal(err)
			}
			var out, errs bytes.Buffer
			opts, _, err := parse([]string{"--typeset", "--output", "-", "--max-edge", "256", path}, &out)
			if err != nil || !opts.Typeset {
				t.Fatal("parse", err)
			}
			if err := exportFiles(opts, &out, &errs); err != nil {
				t.Fatal(err)
			}
			if _, err := png.DecodeConfig(bytes.NewReader(out.Bytes())); err != nil {
				t.Fatal(err)
			}
			out.Reset()
			opts.Text = true
			if err := textFiles(opts, &out, &errs); err != nil || !bytes.Contains(out.Bytes(), []byte(content[:1])) {
				t.Fatal("text extraction", err)
			}
		})
	}
}
