package document

import (
	"strings"
	"testing"
)

func TestMarkdownEmojiShortcodes(t *testing.T) {
	for _, tc := range []struct{ source, want string }{
		{":heart: :fire:", "❤️ 🔥"},
		{"**:fire:** [:heart:](https://example.org/:fire:)", "🔥 ❤️"},
		{"`:fire:`", ":fire:"},
		{`\:fire: :unknown_gloss_emoji: :shipit:`, ":fire: :unknown_gloss_emoji: :shipit:"},
	} {
		t.Run(tc.source, func(t *testing.T) {
			source := []byte(tc.source)
			root, err := readMarkdownTree(source)
			if err != nil {
				t.Fatal(err)
			}
			var got strings.Builder
			for _, s := range inline(root.FirstChild(), source, 0) {
				got.WriteString(s.text)
			}
			if got.String() != tc.want {
				t.Fatalf("got %q, want %q", got.String(), tc.want)
			}
			md, err := loadMarkdown("emoji.md", source, "")
			if err != nil || string(md.Source) != tc.source {
				t.Fatalf("source changed: %v", err)
			}
		})
	}
}
