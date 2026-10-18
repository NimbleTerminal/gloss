package document

import (
	"context"
	"image/color"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

// A light code theme matching the original comparison fixture.
var codeTheme = styles.Get("github")

// Highlight once during preparation, not on each resize or repaint. Unknown or
// absent language tags stay plain; never guess a language or emit ANSI escapes.
func highlightCode(source, language string, theme *chroma.Style) []span {
	return highlightCodeContext(context.Background(), source, language, theme)
}
func highlightCodeContext(ctx context.Context, source, language string, theme *chroma.Style) (out []span) {
	plain := []span{{text: source, style: mono}}
	defer func() {
		if recover() != nil {
			out = plain
		}
	}()
	lexer := lexers.Get(language)
	if language == "" || lexer == nil || theme == nil {
		return plain
	}
	iterator, err := chroma.Coalesce(lexer).Tokenise(&chroma.TokeniseOptions{State: "root", Nested: true}, source)
	if err != nil {
		return plain
	}
	var reconstructed, pending strings.Builder
	var current span
	count := 0
	flush := func() {
		if pending.Len() > 0 {
			current.text = pending.String()
			out = append(out, current)
			pending.Reset()
		}
	}
	for token := iterator(); token != chroma.EOF; token = iterator() {
		count++
		if count > 32768 || (ctx != nil && ctx.Err() != nil) {
			return plain
		}
		if token.Value == "" {
			continue
		}
		reconstructed.WriteString(token.Value)
		entry := theme.Get(token.Type)
		sp := span{text: token.Value, style: mono}
		sp.token = token.Type
		if entry.Bold == chroma.Yes {
			sp.style |= bold
		}
		if entry.Italic == chroma.Yes {
			sp.style |= italic
		}
		// The shipped code theme is resolved from the token at paint time; any
		// other style carries its own ink.
		if theme != codeTheme && entry.Colour.IsSet() {
			sp.ink = color.RGBA{entry.Colour.Red(), entry.Colour.Green(), entry.Colour.Blue(), 255}
		}
		// Buffer adjacent equal styles: repeated string concatenation would
		// make punctuation-heavy untrusted code quadratic.
		if pending.Len() > 0 && (current.token != sp.token || current.style != sp.style || current.ink != sp.ink) {
			flush()
		}
		current = sp
		pending.WriteString(sp.text)
	}
	flush()

	// A lexer must never add/remove code, indentation or line breaks.
	if reconstructed.String() != source {
		return plain
	}
	return out
}
