package document

import (
	"strings"
	"testing"

	"github.com/yuin/goldmark/ast"
	"nimbleterminal.dev/gloss/internal/theme"
)

func TestTypesetQuoteBordersAreContinuous(t *testing.T) {
	source := "> [!WARNING]\n> First paragraph.\n>\n> - A list item.\n> - Another item.\n>\n> ```go\n> x := 1\n> ```\n>\n> | A | B |\n> | - | - |\n> | one | two |\n>\n> > A nested quote.\n> >\n> > Another paragraph.\n>\n> Last paragraph.\n"
	p, err := prepareForTest(testInput(t, source))
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []int{320, 800} {
		e := testEngine(t)
		l := e.layout(p, width)
		l.ensure(100) // Finish later, as scrolling does.
		l.ensure(100000)
		for _, x := range []int{56, 80} {
			first, end, count := -1, -1, 0
			for _, op := range l.ops {
				if op.fill != fillMuted || op.rect.Min.X != x {
					continue
				}
				if end >= 0 && op.rect.Min.Y != end {
					t.Fatalf("width %d, x %d: gap %d..%d", width, x, end, op.rect.Min.Y)
				}
				if first < 0 {
					first = op.rect.Min.Y
				}
				end, count = op.rect.Max.Y, count+1
			}
			if count < 2 {
				t.Fatalf("missing quote segments at x=%d", x)
			}
			img := l.viewport(0, l.y+50, paletteFor("light"))
			for y := first; y < end; y++ {
				if img.RGBAAt(x+1, y) != theme.Get("light").Muted {
					t.Fatalf("missing border pixel at %d,%d", x+1, y)
				}
			}
		}
	}
}

func TestMarkdownExtensionTypesetParity(t *testing.T) {
	p, err := prepareForTest("../../examples/markdown-extensions.md")
	if err != nil {
		t.Fatal(err)
	}
	var content strings.Builder
	for _, b := range p.blocks {
		for _, s := range b.spans {
			content.WriteString(s.text)
		}
		content.WriteByte('\n')
	}
	got := content.String()
	for _, want := range []string{"[1]", "[2]", "Note", "Tip", "Important", "Warning", "Caution", "Footnotes", "1. [back 1] [back 2] Footnotes support", "second paragraph", "2. [back] Another explanation", "Lists stay inside"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %s", want, got)
		}
	}
	for _, width := range []int{320, 800} {
		e := testEngine(t)
		l := e.layout(p, width)
		l.ensure(100000)
		img := l.viewport(0, l.y+50, paletteFor("light"))
		if img.Bounds().Dx() != width || l.next != len(p.blocks) {
			t.Fatal("incomplete extension raster")
		}
	}
}

func TestMarkdownAlertsLeaveOrdinaryQuotesAlone(t *testing.T) {
	for _, source := range []string{"> [!UNKNOWN]\n> Text", "> `[!NOTE]`\n> Text", "> \\[!NOTE]\n> Text", "> [!NOTE] inline text"} {
		root, err := readMarkdownTree([]byte(source))
		if err != nil {
			t.Fatal(err)
		}
		_ = ast.Walk(root, func(n ast.Node, enter bool) (ast.WalkStatus, error) {
			if _, ok := n.(*ast.String); ok {
				t.Errorf("ordinary quote changed: %q", source)
			}
			return ast.WalkContinue, nil
		})
	}
}
