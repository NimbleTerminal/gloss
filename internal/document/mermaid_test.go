package document

import (
	stdimage "image"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/yuin/goldmark/ast"
)

const mermaidMarkdown = "# Diagram\n\n```mermaid\nflowchart LR\n  A --> B\n```\n\nBetween.\n\n```mermaid\nflowchart LR\n  A --> B\n```\n\n```mermaid\nflowchart LR\n  A -->\n```\n"

func TestMarkdownMermaidBecomesImage(t *testing.T) {
	md, err := loadMarkdownFrom("diagram.md", []byte(mermaidMarkdown), t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(md.Images) != 3 {
		t.Fatalf("images = %d, want 3", len(md.Images))
	}
	first, repeat, broken := md.Images[0], md.Images[1], md.Images[2]
	if first.Err != nil || first.Image == nil {
		t.Fatalf("diagram not drawn: %v", first.Err)
	}
	if !strings.HasPrefix(first.Destination, mermaidPrefix) || first.Destination != repeat.Destination {
		t.Fatalf("destinations %q and %q should share one content key", first.Destination, repeat.Destination)
	}
	if first.Layout.Width == 0 || first.Layout != repeat.Layout {
		t.Fatalf("layouts %+v and %+v should carry the reading size", first.Layout, repeat.Layout)
	}
	if first.Alt != "Mermaid diagram: flowchart" {
		t.Fatalf("alt = %q", first.Alt)
	}
	if broken.Err == nil || broken.Image != nil {
		t.Fatal("a diagram that does not parse should report an error")
	}
}

func TestTypesetMermaidFallsBackToSource(t *testing.T) {
	_, root, images, _, _, err := prepareTypeset(Request{Path: "diagram.md", BaseDir: t.TempDir()}, []byte(mermaidMarkdown), nil)
	if err != nil {
		t.Fatal(err)
	}
	drawn := 0
	for _, im := range images {
		if im != nil {
			drawn++
		}
	}
	if drawn != 1 {
		t.Fatalf("drawn diagrams = %d, want 1 shared by two references", drawn)
	}
	// The diagram that does not parse is back in the tree as a note and its
	// own fence, for any layout to set as ordinary text and code.
	var kinds []string
	for n := root.FirstChild(); n != nil; n = n.NextSibling() {
		kinds = append(kinds, n.Kind().String())
	}
	want := "Heading Paragraph Paragraph Paragraph Paragraph FencedCodeBlock"
	if got := strings.Join(kinds, " "); got != want {
		t.Fatalf("blocks = %s, want %s", got, want)
	}
	fence := root.LastChild().(*ast.FencedCodeBlock)
	if code := string(fence.Lines().Value([]byte(mermaidMarkdown))); !strings.Contains(code, "A -->") {
		t.Fatalf("fallback code = %q, want the diagram source", code)
	}
	note, ok := root.LastChild().PreviousSibling().FirstChild().FirstChild().(*ast.String)
	if !ok || !strings.Contains(string(note.Value), "could not be drawn") {
		t.Fatal("no note before the fallback source")
	}
}

// go-mermaid paints its own page, so a diagram stays legible on a dark
// terminal without Gloss putting it on a card.
func TestMermaidDiagramIsOpaque(t *testing.T) {
	img, _, _, err := renderMermaid("flowchart LR\n  A --> B\n", 0, false)
	if err != nil {
		t.Fatal(err)
	}
	// One pixel in: the outermost column is antialiased where the diagram's
	// fractional width meets the pixel grid.
	b := img.Bounds().Inset(1)
	for _, p := range []stdimage.Point{b.Min, {b.Max.X - 1, b.Min.Y}, {b.Min.X, b.Max.Y - 1}, {b.Max.X - 1, b.Max.Y - 1}} {
		if _, _, _, a := img.At(p.X, p.Y).RGBA(); a != 0xffff {
			t.Fatalf("corner %v alpha %#x, want opaque", p, a)
		}
	}
}

// A dark theme draws diagrams on go-mermaid's dark page, a light one on its
// light page.
func TestMermaidPaletteFollowsTheme(t *testing.T) {
	luma := func(dark bool) uint32 {
		img, _, _, err := renderMermaid("flowchart LR\n  A --> B\n", 0, dark)
		if err != nil {
			t.Fatal(err)
		}
		r, g, b, _ := img.At(img.Bounds().Min.X+1, img.Bounds().Min.Y+1).RGBA()
		return (r + g + b) / 3 >> 8
	}
	if light, dark := luma(false), luma(true); light < 200 || dark > 80 {
		t.Fatalf("page brightness light %d, dark %d", light, dark)
	}
}

// Typeset reuses its preparation for the same file, except when a diagram's
// palette must change with the theme.
func TestTypesetRedrawsMermaidForTheme(t *testing.T) {
	files := fstest.MapFS{"diagram.md": {Data: []byte(mermaidMarkdown)}}
	l := &Loader{Files: files}
	defer l.Close()
	load := func(themeName string) *Markdown {
		r := l.Load(Request{Path: "diagram.md", MarkdownMode: "typeset", Theme: testTheme(themeName), Page: 1, DPI: 72, Generation: 1})
		if r.Err != nil {
			t.Fatal(r.Err)
		}
		return r.Typeset
	}
	first := load("light")
	if first.Diagrams != 3 {
		t.Fatalf("diagrams = %d, want 3", first.Diagrams)
	}
	if again := load("light"); again != first {
		t.Fatal("the same palette should reuse the preparation")
	}
	if dark := load("dark"); dark == first || !l.typesetDark {
		t.Fatal("a dark theme should draw the diagrams again")
	}
}

// A .mmd file is one diagram: detected by its name, drawn in the theme's
// palette, described, and its source is its text.
func TestMermaidFile(t *testing.T) {
	src := "flowchart LR\n  A --> B\n"
	for _, name := range []string{"flow.mmd", "flow.MERMAID"} {
		if kind, err := Detect(name, []byte(src), ""); err != nil || kind != "mermaid" {
			t.Fatalf("Detect(%q) = %q, %v", name, kind, err)
		}
	}
	files := fstest.MapFS{"flow.mmd": {Data: []byte(src)}, "bad.mmd": {Data: []byte("flowchart LR\n  A -->\n")}}
	l := &Loader{Files: files}
	defer l.Close()
	r := l.Load(Request{Path: "flow.mmd", Theme: testTheme("light"), Page: 1})
	if r.Err != nil || r.Kind != "mermaid" || r.Image == nil {
		t.Fatalf("load = %q, %v, image %v", r.Kind, r.Err, r.Image != nil)
	}
	var fields []string
	for _, f := range r.Info {
		fields = append(fields, f.Label+"="+f.Value)
	}
	for _, want := range []string{"Format=Mermaid", "Type=flowchart", "Lines=2"} {
		if !strings.Contains(strings.Join(fields, ";"), want) {
			t.Errorf("info %v lacks %s", fields, want)
		}
	}
	if text, ext, err := Text(r); err != nil || string(text) != src || ext != ".mmd" {
		t.Fatalf("Text = %q, %q, %v", text, ext, err)
	}
	if r := l.Load(Request{Path: "flow.mmd", Page: 1, TextOnly: true}); r.Err != nil || r.Image != nil || r.Text != src {
		t.Fatalf("text-only load drew the diagram or lost its source: %v", r.Err)
	}
	if r := l.Load(Request{Path: "bad.mmd", Page: 1}); r.Err == nil {
		t.Fatal("a diagram that does not parse should be an error")
	}
	if r := l.Load(Request{Path: "flow.mmd", Page: 1, MaxEdge: 300}); r.Err != nil || max(r.Image.Bounds().Dx(), r.Image.Bounds().Dy()) != 300 {
		t.Fatalf("max edge not applied: %v", r.Err)
	}
}
