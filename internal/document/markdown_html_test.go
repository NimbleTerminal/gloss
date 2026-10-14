package document

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/yuin/goldmark/ast"
	"nimbleterminal.dev/ntpicture/animation"
)

func TestReadmeHTMLImages(t *testing.T) {
	sources := []string{
		"[![Hello, Tiny World](assets/hello.gif)](./devices/README.md)",
		`<img src="assets/hello.gif" alt="Hello, Tiny World">`,
		"<p align=\"center\">\n <a href=\"./devices/README.md\">\n <img src=\"assets/hello.gif\"\n alt=\"Hello, Tiny World\" width=\"600\">\n </a>\n</p>",
		`before <a href="./devices/README.md"><img src="assets/hello.gif" alt="Hello, Tiny World"></a> after`,
	}
	for _, source := range sources {
		for _, typeset := range []bool{false, true} {
			// A nested document directory makes a CWD-relative implementation fail.
			files := fstest.MapFS{"project/docs/README.md": {Data: []byte(source)}, "project/docs/assets/hello.gif": {Data: gifTestData(t, 0)}}
			l := &Loader{Files: files}
			r := l.Load(Request{Path: "project/docs/README.md", Typeset: typeset, Animate: true, TypesetView: &TypesetViewport{Height: 500}})
			if r.Err != nil {
				t.Fatal(r.Err)
			}
			if typeset && strings.HasPrefix(source, "before") {
				l.Close()
				continue
			} // Mixed inline figures use the existing text placeholder in typeset.
			if typeset {
				if len(r.Animated) != 1 || r.Playback == nil {
					t.Fatalf("HTML figure missing: %q", source)
				}
				before := r.Playback.Image()
				advanced, more, err := r.Playback.Advance(context.Background())
				if err != nil {
					t.Fatal(err)
				}
				next := advanced.(*animation.Composite)
				after := next.Image()
				rect := r.Animated[0].Rect
				x, y := rect.Min.X+rect.Dx()/2, rect.Min.Y+rect.Dy()/2
				if !more || before.At(x, y) == after.At(x, y) {
					t.Fatal("typeset GIF did not animate")
				}
			} else {
				if len(r.Markdown.Images) != 1 {
					t.Fatalf("missing image: %q", source)
				}
				im := r.Markdown.Images[0]
				if im.Err != nil || im.Image == nil || im.Animation == nil {
					t.Fatalf("image: %+v", im)
				}
				next, more := im.Animation.Next()
				if !more || next.Frame() == im.Animation.Frame() {
					t.Fatal("GIF did not advance")
				}
				if string(r.Markdown.Source) != source {
					t.Fatal("source rewritten")
				}
			}
			l.Close()
		}
	}
}

func TestReadmeHTMLSafetyAndLayout(t *testing.T) {
	source := []byte(`<p align="center"><a href="javascript:alert(1)"><img src="local.gif" width="600" height="300" onerror="bad()" style="bad" alt="safe"></a><script><img src="hidden.gif"></script><iframe><img src="hidden2.gif"></iframe></p>`)
	root, err := readMarkdownTree(source)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	_ = ast.Walk(root, func(n ast.Node, enter bool) (ast.WalkStatus, error) {
		if im, ok := n.(*ast.Image); ok && enter {
			count++
			if _, linked := im.Parent().(*ast.Link); linked {
				t.Fatal("active URL survived")
			}
			if string(im.Destination) != "local.gif" {
				t.Fatal("active subtree survived")
			}
			layout := MarkdownImageLayout(im)
			if layout != (ImageLayout{600, 300, "center"}) {
				t.Fatalf("layout: %+v", layout)
			}
			w, h := layout.Fit(1200, 400)
			if w != 600 || h != 200 || layout.Offset(1000, w) != 200 {
				t.Fatal("bad aspect or alignment")
			}
		}
		return ast.WalkContinue, nil
	})
	if count != 1 {
		t.Fatalf("images: %d", count)
	}
	for _, s := range []string{"0", "-1", "100%", "100000000000000000000", "4097"} {
		if readmeDimension(s) != 0 {
			t.Fatal(s)
		}
	}
	// HTML-looking code is never an image, and remote/data assets stay rejected.
	md, err := loadMarkdownFrom("doc.md", []byte("`<img src=\"local.gif\">`\n\n```html\n<img src=\"local.gif\">\n```\n\n<img src=\"https://example.invalid/a.gif\">\n\n<img src=\"data:image/gif;base64,AA\">"), "", fstest.MapFS{})
	if err != nil || len(md.Images) != 2 {
		t.Fatalf("code/asset discovery: %v %+v", err, md)
	}
	for _, im := range md.Images {
		if im.Err == nil || im.Image != nil {
			t.Fatal("nonlocal image loaded")
		}
	}
}

func TestHTMLImageBudgets(t *testing.T) {
	source := []byte(strings.Repeat(`<img src="missing.png">`+"\n\n", MaxMarkdownImages+1))
	md, err := loadMarkdownFrom("doc.md", source, "", fstest.MapFS{})
	if err != nil || len(md.Images) != MaxMarkdownImages {
		t.Fatalf("image cap: %v", err)
	}
	l := &Loader{Files: fstest.MapFS{"doc.md": {Data: source}}}
	defer l.Close()
	r := l.Load(Request{Path: "doc.md", Typeset: true})
	if r.Err == nil || !strings.Contains(r.Err.Error(), "image references") {
		t.Fatalf("typeset cap: %v", r.Err)
	}
}
