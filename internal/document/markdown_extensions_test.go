package document

import (
	"context"
	"strings"
	"testing"

	"github.com/yuin/goldmark/ast"
	"nimbleterminal.dev/gloss/internal/theme"
	"nimbleterminal.dev/nttypeset"
)

func TestTypesetQuoteBordersAreContinuous(t *testing.T) {
	source := "> [!WARNING]\n> First paragraph.\n>\n> - A list item.\n> - Another item.\n>\n> ```go\n> x := 1\n> ```\n>\n> | A | B |\n> | - | - |\n> | one | two |\n>\n> > A nested quote.\n> >\n> > Another paragraph.\n>\n> Last paragraph.\n"
	doc, _, err := prepareForTest(testInput(t, source))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	muted := theme.Get("light").Muted
	for _, width := range []int{320, 800} {
		l := testLayout(t, doc, width)
		if err := l.Ensure(ctx, 100); err != nil { // Finish later, as scrolling does.
			t.Fatal(err)
		}
		if err := l.Complete(ctx); err != nil {
			t.Fatal(err)
		}
		img, err := l.Raster(ctx, nttypeset.View{Width: width, Height: l.Extent(), Scale: 1}, nttypeset.RasterOptions{Palette: typesetPalette("light")})
		if err != nil {
			t.Fatal(err)
		}
		// The outer quote's bar is at x 56, the nested one's at 80: each is
		// one unbroken run of muted pixels, more than a block high.
		for _, x := range []int{56, 80} {
			first, end := -1, -1
			for y := 0; y < img.Bounds().Dy(); y++ {
				if img.RGBAAt(x+1, y) == muted {
					if first < 0 {
						first = y
					}
					end = y + 1
				}
			}
			if first < 0 || end-first < 2*37 {
				t.Fatalf("width %d: missing quote bar at x=%d (%d..%d)", width, x, first, end)
			}
			for y := first; y < end; y++ {
				if img.RGBAAt(x+1, y) != muted {
					t.Fatalf("width %d, x %d: gap at %d in %d..%d", width, x, y, first, end)
				}
			}
		}
	}
}

func TestMarkdownExtensionTypesetParity(t *testing.T) {
	doc, _, err := prepareForTest("../../examples/markdown-extensions.md")
	if err != nil {
		t.Fatal(err)
	}
	var content strings.Builder
	for _, b := range doc.Blocks {
		for _, s := range b.Spans {
			content.WriteString(s.Text)
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
		l := testLayout(t, doc, width)
		if err := l.Complete(context.Background()); err != nil {
			t.Fatal(err)
		}
		img, err := l.Raster(context.Background(), nttypeset.View{Width: width, Height: l.Extent(), Scale: 1}, nttypeset.RasterOptions{Palette: typesetPalette("light")})
		if err != nil || img.Bounds().Dx() != width || !l.Done() {
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
