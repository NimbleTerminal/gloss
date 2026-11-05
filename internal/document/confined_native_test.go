//go:build !js

package document

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"nimbleterminal.dev/ntmediabox"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func TestStrictRefusesUnavailableConfinement(t *testing.T) {
	old := parserCommand
	t.Cleanup(func() { parserCommand = old })
	calls := 0
	parserCommand = func(_ string, required bool) (*exec.Cmd, error) {
		calls++
		if !required {
			t.Fatal("strict load requested fallback")
		}
		return nil, errors.New("confinement unavailable")
	}
	l := &Loader{RequireConfinement: true, Files: fstest.MapFS{"a.png": &fstest.MapFile{Data: []byte("malformed")}}}
	defer l.Close()
	out := l.Load(Request{Path: "a.png"})
	if out.Err == nil || !strings.Contains(out.Err.Error(), "requires OS confinement") || calls != 1 {
		t.Fatalf("load: %v calls=%d", out.Err, calls)
	}
	if _, err := ProbeFS(l.Files, "a.png", "", true); err == nil || calls != 2 {
		t.Fatalf("probe: %v calls=%d", err, calls)
	}
}

func TestStrictDocuments(t *testing.T) {
	if !ntmediabox.Available() {
		t.Skip("OS confinement unavailable; refusal is covered separately")
	}
	// Exercise the real OS boundary. Failure on a supported native CI host is
	// a regression, not permission to retry without confinement.
	started := countWorkers(t)
	old := parserCommand
	parserCommand = func(exe string, required bool) (*exec.Cmd, error) {
		if !required {
			t.Fatal("strict load spawned non-strict nested worker")
		}
		return old(exe, required)
	}
	data := encodeTestPNG(t, image.NewRGBA(image.Rect(0, 0, 8, 8)))
	var gifBytes bytes.Buffer
	frame := image.NewPaletted(image.Rect(0, 0, 4, 4), color.Palette{color.Black, color.White})
	if err := gif.EncodeAll(&gifBytes, &gif.GIF{Image: []*image.Paletted{frame, frame}, Delay: []int{1, 1}}); err != nil {
		t.Fatal(err)
	}
	pdf := pagesPDF("BT /F1 12 Tf 10 10 Td (confined text) Tj ET")
	files := fstest.MapFS{
		"anim.gif":    &fstest.MapFile{Data: gifBytes.Bytes()},
		"still.png":   &fstest.MapFile{Data: data},
		"diagram.svg": &fstest.MapFile{Data: []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16"><rect width="16" height="16" fill="red"/></svg>`)},
		"diagram.mmd": &fstest.MapFile{Data: []byte("graph TD\n A --> B")},
		"one.pdf":     &fstest.MapFile{Data: pdf},
		"notes.md":    &fstest.MapFile{Data: []byte("# Not supported yet")},
	}
	l := &Loader{Files: files, RequireConfinement: true}
	defer l.Close()
	for _, name := range []string{"still.png", "diagram.svg", "diagram.mmd", "one.pdf"} {
		t.Run(name, func(t *testing.T) {
			r := l.Load(Request{Path: name, Page: 1, DPI: 72})
			if r.Err != nil || r.Image == nil {
				t.Fatalf("strict raster: %v image=%v", r.Err, r.Image != nil)
			}
		})
	}
	r := l.Load(Request{Path: "one.pdf", TextOnly: true, Page: 1})
	if r.Err != nil || !strings.Contains(r.Text, "confined text") {
		t.Fatalf("text: %q %v", r.Text, r.Err)
	}
	r = l.Load(Request{Path: "one.pdf", Describe: true, Page: 1})
	if r.Err != nil || r.Image != nil || len(r.Info) == 0 {
		t.Fatalf("metadata: %+v", r)
	}
	r = l.Load(Request{Path: "notes.md"})
	if r.Err == nil || !strings.Contains(r.Err.Error(), "strict mode does not yet support") {
		t.Fatalf("unsupported: %v", r.Err)
	}
	r = l.Load(Request{Path: "one.pdf", TextOnly: true, Page: 2, Strict: true})
	if r.Err == nil {
		t.Fatal("missing page accepted")
	}
	r = l.Load(Request{Path: "anim.gif", Animate: true})
	if r.Err == nil || !strings.Contains(r.Err.Error(), "does not yet support animation") {
		t.Fatalf("animation accepted: %v", r.Err)
	}
	r = l.Load(Request{Path: "anim.gif"})
	if r.Err != nil || r.Image == nil {
		t.Fatalf("still GIF: %v", r.Err)
	}
	r = l.Load(Request{Path: "still.png", Typeset: true})
	if r.Err == nil {
		t.Fatal("typesetting accepted")
	}
	for _, cmd := range *started {
		reaped(t, cmd)
	}
}

func TestStrictProbeAndAccept(t *testing.T) {
	if !ntmediabox.Available() {
		t.Skip("OS confinement unavailable; refusal is covered separately")
	}
	path := filepath.Join(t.TempDir(), "image.png")
	if err := os.WriteFile(path, encodeTestPNG(t, image.NewRGBA(image.Rect(0, 0, 4, 4))), 0600); err != nil {
		t.Fatal(err)
	}
	if kind, err := Probe(path, "", true); err != nil || kind != "png" {
		t.Fatalf("probe %q %v", kind, err)
	}
	if err := AcceptFilter("pdf").CheckFile(nil, path, true); err == nil {
		t.Fatal("accepted image as PDF")
	}
}

func TestStrictNoTextPreservesPDFMetadata(t *testing.T) {
	if !ntmediabox.Available() {
		t.Skip("OS confinement unavailable")
	}
	l := &Loader{RequireConfinement: true, Files: fstest.MapFS{"scan.pdf": &fstest.MapFile{Data: pagesPDF("", "BT /F1 12 Tf 10 10 Td (later text) Tj ET")}}}
	defer l.Close()
	r := l.Load(Request{Path: "scan.pdf", Page: 1, TextOnly: true})
	if !errors.Is(r.Err, ErrNoTextLayer) || r.Kind != "pdf" || r.Page != 1 || r.Pages != 2 || r.Layer == nil || len(r.Info) == 0 {
		t.Fatalf("document error lost: %+v", r)
	}
	r = l.Load(Request{Path: "scan.pdf", Page: 2, TextOnly: true})
	if r.Err != nil || !strings.Contains(r.Text, "later text") {
		t.Fatalf("later page: %+v", r)
	}
}

func TestStrictLayoutStillRejectsHTML(t *testing.T) {
	if !ntmediabox.Available() {
		t.Skip("OS confinement unavailable")
	}
	l := &Loader{RequireConfinement: true, Files: fstest.MapFS{"page.html": &fstest.MapFile{Data: []byte("<!doctype html><html><body>untrusted</body></html>")}}}
	defer l.Close()
	r := l.Load(Request{Path: "page.html", Layout: true})
	if r.Err == nil || !strings.Contains(r.Err.Error(), "strict mode does not yet support html") || r.Image != nil {
		t.Fatalf("HTML escaped strict format gate: %+v", r)
	}
}
