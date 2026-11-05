package main

import (
	"bytes"
	"encoding/json"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nimbleterminal.dev/ntmediabox"
)

func TestStrictOptions(t *testing.T) {
	for _, args := range [][]string{{"photo.png"}, {"--info", "photo.png"}, {"--text", "report.pdf"}, {"--output", "page.png", "report.pdf"}, {"--print", "photo.png"}} {
		opts, _, err := parse(append([]string{"--strict"}, args...), io.Discard)
		if err != nil || !opts.StrictMode {
			t.Fatalf("%v: strict=%v err=%v", args, opts.StrictMode, err)
		}
	}
	for _, args := range [][]string{{"--serve"}, {"--pick"}, {"--fetch", "image.png"}, {"https://example.com/a.png"}, {"HTTP://example.com/a.png"}} {
		if _, _, err := parse(append([]string{"--strict"}, args...), io.Discard); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	opts, _, err := parse([]string{"image.png"}, io.Discard)
	if err != nil || opts.StrictMode {
		t.Fatalf("strict should be opt-in: %+v %v", opts.StrictMode, err)
	}
}
func TestStrictDescribeReportsUnsupported(t *testing.T) {
	if !ntmediabox.Available() {
		t.Skip("OS confinement unavailable")
	}
	path := filepath.Join(t.TempDir(), "notes.md")
	if err := os.WriteFile(path, []byte("# notes"), 0600); err != nil {
		t.Fatal(err)
	}
	opts, _, err := parse([]string{"--strict", "--info", "--json", path}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	_ = describe(opts, &out, io.Discard)
	if !strings.Contains(out.String(), "strict mode does not yet support markdown") {
		t.Fatalf("expected error: %s", out.String())
	}
}

func TestStrictPNGExport(t *testing.T) {
	if !ntmediabox.Available() {
		t.Skip("OS confinement unavailable")
	}
	for _, path := range []string{"../../examples/landscape.png", "../../examples/shapes.svg", "../../examples/field-guide.pdf"} {
		t.Run(path, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			opts, _, err := parse([]string{"--strict", "--output", "-", path}, io.Discard)
			if err != nil {
				t.Fatal(err)
			}
			if err := exportFiles(opts, &stdout, &stderr); err != nil {
				t.Fatalf("export: %v; %s", err, stderr.String())
			}
			cfg, err := png.DecodeConfig(&stdout)
			if err != nil || cfg.Width < 1 || cfg.Height < 1 {
				t.Fatalf("PNG: %+v %v", cfg, err)
			}
		})
	}
}

func TestStrictAllTextPagesAfterScan(t *testing.T) {
	if !ntmediabox.Available() {
		t.Skip("OS confinement unavailable")
	}
	path := figurePDF(t, "q /Im1 Do Q", "BT /F1 12 Tf 10 10 Td (needle after scan) Tj ET")
	opts, _, err := parse([]string{"--strict", "--text", "--page", "all", "--json", path}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if err := textFiles(opts, &stdout, &stderr); err == nil {
		t.Fatal("scan should report missing text")
	}
	var got []made
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil || len(got) != 2 {
		t.Fatalf("manifest: %v %s", err, stdout.String())
	}
	if got[0].Kind != "pdf" || got[0].Page != 1 || got[0].Pages != 2 || got[0].Error == "" || got[0].Images == nil || *got[0].Images != 1 {
		t.Fatalf("scan metadata lost: %+v", got[0])
	}
	if got[1].Page != 2 || got[1].Error != "" || !strings.Contains(got[1].Text, "needle after scan") {
		t.Fatalf("later text lost: %+v", got[1])
	}
}
