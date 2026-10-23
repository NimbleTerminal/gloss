package document

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMarkdownAssets(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "shape one.svg"), []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="20" height="10"><rect width="20" height="10" fill="red"/></svg>`), 0600); err != nil {
		t.Fatal(err)
	}
	source := "# Title\n![local](shape%20one.svg)\n![ref][asset]\n\n[asset]: shape%20one.svg\n\n![remote](https://example.com/a.png)\n![missing](missing.png)\n\n```md\n![code](ignore.png)\n```\n"
	md, err := loadMarkdown(filepath.Join(dir, "doc.md"), []byte(source), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(md.Images) != 4 {
		t.Fatalf("images: %d", len(md.Images))
	}
	for _, asset := range md.Images[:2] {
		if asset.Err != nil || asset.Image == nil {
			t.Fatalf("local asset: %+v", asset)
		}
	}
	for _, asset := range md.Images[2:] {
		if asset.Err == nil || asset.Image != nil {
			t.Fatalf("unavailable asset: %+v", asset)
		}
	}
	if md.Images[0].Image != md.Images[1].Image {
		t.Fatal("repeated assets should share decoded pixels")
	}
}

func TestMarkdownLimitsAndControls(t *testing.T) {
	const unicodeText = "👩🏽‍💻 🇯🇵 ❤️ é ▀▄▌▐░▒▓█ ｶﾀｶﾅ カタカナ a\u200cb"
	if got := cleanMarkdownSource([]byte(unicodeText)); got != unicodeText {
		t.Fatalf("Unicode text changed: %q", got)
	}
	if _, err := loadMarkdown("x.md", make([]byte, MaxMarkdownBytes+1), ""); err == nil {
		t.Fatal("size limit not enforced")
	}
	if _, err := loadMarkdown("x.md", []byte{255}, ""); err == nil {
		t.Fatal("invalid UTF-8 accepted")
	}
	md, err := loadMarkdown("x.md", []byte("\x1b[2J\u202e\n"+strings.Repeat("![x](https://example.com/x)\n", 40)), "")
	if err != nil {
		t.Fatal(err)
	}
	if strings.ContainsAny(string(md.Source), "\x1b\u202e") {
		t.Fatal("terminal controls retained")
	}
	if len(md.Images) != MaxMarkdownImages {
		t.Fatal("asset limit not enforced")
	}
	kind, err := Detect("doc.md", []byte("<svg> is an example"), "")
	if err != nil || kind != "markdown" {
		t.Fatalf("extension detection: %s, %v", kind, err)
	}
}

// A picture beside the document keeps the file it was read from; a drawn
// diagram and a notebook's own picture have none.
func TestMarkdownImagePath(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "x.png"), pngBytes(t), 0600); err != nil {
		t.Fatal(err)
	}
	doc := filepath.Join(dir, "doc.md")
	md, err := loadMarkdown(doc, []byte("![a](x.png)\n\n```mermaid\nflowchart LR\n  A --> B\n```\n"), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(md.Images) != 2 || md.Images[0].Err != nil || md.Images[1].Err != nil {
		t.Fatalf("images: %+v", md.Images)
	}
	if want := filepath.Join(dir, "x.png"); md.Images[0].Path != want {
		t.Fatalf("path %q, want %q", md.Images[0].Path, want)
	}
	if md.Images[1].Path != "" {
		t.Fatalf("diagram path %q", md.Images[1].Path)
	}
	l := &Loader{}
	defer l.Close()
	r := l.Load(Request{Path: write(t, "nb.ipynb", notebook(t)), Page: 1, DPI: 72, Generation: 1})
	if r.Err != nil || r.Markdown == nil || len(r.Markdown.Images) != 1 || r.Markdown.Images[0].Image == nil {
		t.Fatalf("notebook: err=%v %+v", r.Err, r.Markdown)
	}
	if p := r.Markdown.Images[0].Path; p != "" {
		t.Fatalf("notebook picture path %q", p)
	}
}
