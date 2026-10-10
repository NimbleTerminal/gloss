package main

import (
	"image/color"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

// Match the existing typeset PDF backend's light code theme.
var codeTheme = styles.Get("github")

// Highlight once during preparation, not on each resize or repaint. Unknown or
// absent language tags stay plain; never guess a language or emit ANSI escapes.
func highlightCode(source, language string, theme *chroma.Style) (out []span) {
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
	var reconstructed strings.Builder
	for token := iterator(); token != chroma.EOF; token = iterator() {
		if token.Value == "" {
			continue
		}
		reconstructed.WriteString(token.Value)
		entry := theme.Get(token.Type)
		sp := span{text: token.Value, style: mono}
		if entry.Bold == chroma.Yes {
			sp.style |= bold
		}
		if entry.Italic == chroma.Yes {
			sp.style |= italic
		}
		if entry.Colour.IsSet() {
			sp.ink = color.RGBA{entry.Colour.Red(), entry.Colour.Green(), entry.Colour.Blue(), 255}
		}
		// Merge identical visual styles to avoid unnecessary shaping/raster runs.
		if n := len(out); n > 0 && out[n-1].style == sp.style && out[n-1].ink == sp.ink {
			out[n-1].text += sp.text
		} else {
			out = append(out, sp)
		}
	}
	// A lexer must never add/remove code, indentation or line breaks.
	if reconstructed.String() != source {
		return plain
	}
	return out
}
