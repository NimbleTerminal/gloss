package app

import (
	"os"
	"strings"
	"testing"

	"nimbleterminal.dev/gloss/internal/document"
)

func TestMarkdownExtensionTextParity(t *testing.T) {
	source, err := os.ReadFile("../../examples/markdown-extensions.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, theme := range []string{"light", "dark"} {
		content := renderedContent(&document.Markdown{Source: source}, theme)
		var text strings.Builder
		for _, u := range content.units {
			text.WriteString(u.plain)
			text.WriteByte('\n')
		}
		for _, want := range []string{"[1]", "[2]", "Note", "Tip", "Important", "Warning", "Caution", "Footnotes", "Footnotes support", "second paragraph", "Another explanation", "Lists stay inside"} {
			if !strings.Contains(text.String(), want) {
				t.Errorf("%s missing %q: %s", theme, want, text.String())
			}
		}
	}
}
