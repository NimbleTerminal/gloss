package document

import (
	stdimage "image"
	"strings"
	"testing"

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
	_, root, assets, err := prepareTypeset(Request{Path: "diagram.md", BaseDir: t.TempDir()}, []byte(mermaidMarkdown), nil)
	if err != nil {
		t.Fatal(err)
	}
	drawn := 0
	for _, a := range assets {
		if a != nil && a.image != nil {
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
	img, _, _, err := renderMermaid("flowchart LR\n  A --> B\n", 0)
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
