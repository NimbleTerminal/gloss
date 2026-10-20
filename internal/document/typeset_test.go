package document

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
)

type forbidTypesetNetwork struct{ t *testing.T }

func (f forbidTypesetNetwork) RoundTrip(*http.Request) (*http.Response, error) {
	f.t.Error("typesetting attempted network access")
	return nil, os.ErrPermission
}

func TestTypesetDirect(t *testing.T) {
	old := http.DefaultTransport
	http.DefaultTransport = forbidTypesetNetwork{t}
	defer func() { http.DefaultTransport = old }()
	var l Loader
	defer l.Close()
	r := l.Load(Request{Theme: testTheme("light"), Path: "../../examples/typeset.md", MarkdownMode: "typeset", Page: 1, DPI: 150})
	if r.Err != nil {
		t.Fatal(r.Err)
	}
	if r.Image == nil || r.Typeset == nil || r.Kind != "markdown" || l.pdf != nil || l.pdfReader != nil {
		t.Fatal("typeset should draw without PDFium")
	}
	if r.Image.Bounds().Dx() != 800 || r.Pages < 2 {
		t.Fatalf("bad slices: %v %d", r.Image.Bounds(), r.Pages)
	}
}

func TestTypesetBudgets(t *testing.T) {
	for name, source := range map[string][]byte{"bytes": bytes.Repeat([]byte("x"), MaxMarkdownBytes+1), "utf8": {0xff}, "references": []byte(strings.Repeat("![x](missing.png)\n", 33))} {
		t.Run(name, func(t *testing.T) {
			_, _, _, _, err := prepareTypeset(Request{Path: "x.md"}, source, nil)
			if err == nil {
				t.Fatal("budget accepted")
			}
		})
	}
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	// PNG config reads dimensions before decompression. Fix the IHDR checksum
	// so this is a valid header asking for an over-budget allocation.
	data := b.Bytes()
	binary.BigEndian.PutUint32(data[16:20], 5000)
	binary.BigEndian.PutUint32(data[20:24], 5000)
	binary.BigEndian.PutUint32(data[29:33], crc32.ChecksumIEEE(data[12:29]))
	files := fstest.MapFS{"big.png": &fstest.MapFile{Data: data}}
	_, _, _, _, err := prepareTypeset(Request{Path: "x.md"}, []byte("![big](big.png)"), files)
	if err == nil {
		t.Fatal("oversized decoded image accepted")
	}
}

func TestTypesetCache(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.md")
	if err := os.WriteFile(path, []byte("# First\n\nCafé."), 0600); err != nil {
		t.Fatal(err)
	}
	var l Loader
	defer l.Close()
	q := Request{Theme: testTheme("light"), Path: path, MarkdownMode: "typeset", DPI: 150, Page: 1}
	if r := l.Load(q); r.Err != nil {
		t.Fatal(r.Err)
	}
	first, engine := l.direct, l.typesetEngine
	q.DPI = 200
	q.MaxEdge = 640
	if r := l.Load(q); r.Err != nil {
		t.Fatal(r.Err)
	}
	if first != l.direct {
		t.Fatal("resolution changed layout")
	}
	q.TypesetWidth = 240
	r := l.Load(q)
	if r.Err != nil {
		t.Fatal(r.Err)
	}
	if l.direct == first || l.typesetEngine != engine {
		t.Fatal("resize must reuse fonts but change layout")
	}
	first = l.direct
	if r := l.Load(q); r.Err != nil || l.direct != first {
		t.Fatal("identical request changed layout")
	}
	if err := os.WriteFile(path, []byte("# Changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if r := l.Load(q); r.Err != nil || l.direct == first {
		t.Fatal("stale content")
	}
	q.MarkdownMode = "text"
	if r := l.Load(q); r.Err != nil || r.Markdown == nil || r.Typeset != nil {
		t.Fatal("text mode")
	}
}

func TestTypesetLocalImages(t *testing.T) {
	var b bytes.Buffer
	im := image.NewRGBA(image.Rect(0, 0, 8, 6))
	im.Set(0, 0, color.White)
	if err := png.Encode(&b, im); err != nil {
		t.Fatal(err)
	}
	files := fstest.MapFS{"docs/pic.png": &fstest.MapFile{Data: b.Bytes()}}
	_, _, images, _, err := prepareTypeset(Request{Path: "docs/doc.md"}, []byte("![ok](pic.png) ![no](https://example.invalid/x.png) ![missing](none.png)"), files)
	if err != nil {
		t.Fatal(err)
	}
	if images["pic.png"] == nil || images["https://example.invalid/x.png"] != nil || images["none.png"] != nil {
		t.Fatal("local resolution/placeholder policy")
	}
}

func TestTypesetConcurrent(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Go(func() {
			var l Loader
			l.Files = fstest.MapFS{"x.md": &fstest.MapFile{Data: []byte("# Concurrent\n\n| A | B |\n|---|---|\n| café | résumé |")}}
			defer l.Close()
			if r := l.Load(Request{Theme: testTheme("light"), Path: "x.md", MarkdownMode: "typeset"}); r.Err != nil || r.Image == nil {
				t.Errorf("render: %v", r.Err)
			}
		})
	}
	wg.Wait()
}

func TestTypesetSourceSafetyAndStdin(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("local.png", b.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	data := []byte("![local](local.png)\n\nsource\x1b]2;title\a\n")
	source, _, images, _, err := prepareTypeset(Request{Path: "/elsewhere/stdin", Stdin: true}, data, nil)
	if err != nil || images["local.png"] == nil {
		t.Fatalf("stdin cwd resolution: %v", err)
	}
	if bytes.ContainsAny(source.Source, "\x1b\a") {
		t.Fatal("terminal controls reached source viewer")
	}
	if _, _, _, _, err := prepareTypeset(Request{Path: "x.md"}, []byte(strings.Repeat("![x](missing.png)\n", 32)), nil); err != nil {
		t.Fatalf("32 references: %v", err)
	}
}
