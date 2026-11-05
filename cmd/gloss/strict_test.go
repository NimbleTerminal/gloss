package main

import (
	"bytes"
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
