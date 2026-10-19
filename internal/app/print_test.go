package app

import (
	"context"
	"image"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"nimbleterminal.dev/gloss/internal/document"
)

func TestPrintSafe(t *testing.T) {
	input := "a\x1b_Ga=d,d=A\x1b\\b\x1b]52;c;YWJj\a\x1b[2J\x1b[?25l\x1b[31mred\x1b[0m\nend\r\x00"
	got := printSafe(input)
	if got != "ab\x1b[31mred\x1b[0m\nend" {
		t.Fatalf("unsafe or damaged output: %q", got)
	}
	if got := printSafe("cafe\u0301"); got != "cafe\u0301" {
		t.Fatalf("combining text damaged: %q", got)
	}
}

func TestPrintMarkdownPicturesAndBounds(t *testing.T) {
	doc := &document.Markdown{Source: []byte("# Hello\n\n![picture](local.png)\n\nlast paragraph"), Images: []document.MarkdownImage{{Alt: "picture", Image: image.NewRGBA(image.Rect(0, 0, 40, 20))}}}
	r := document.Result{Kind: "markdown", Markdown: doc}
	for _, size := range [][2]int{{40, 12}, {1, 1}} {
		v, err := PrintView(context.Background(), r, Options{}, size[0], size[1])
		if err != nil {
			t.Fatal(err)
		}
		if strings.Count(v, "\n") >= size[1] {
			t.Fatalf("too tall: %q", v)
		}
		for _, line := range strings.Split(v, "\n") {
			if ansi.StringWidth(line) > size[0] {
				t.Fatalf("too wide: %q", line)
			}
		}
		if size[0] == 40 && (!strings.Contains(v, "Hello") || !glyphs(v)) {
			t.Fatalf("missing Markdown or image: %q", v)
		}
	}
}
