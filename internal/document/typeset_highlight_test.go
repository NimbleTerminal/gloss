package document

import (
	"bytes"
	"context"
	"image/color"
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
	"nimbleterminal.dev/gloss/internal/theme"
)

func TestHighlightCode(t *testing.T) {
	for _, tc := range []struct{ language, source string }{
		{"go", "// café\nfunc greeting() string {\n\treturn \"résumé\"\n}"},
		{"python", "# café\ndef greeting():\n    return \"résumé\"\n"},
		{"json", "{\"answer\": 42, \"ok\": true}"},
	} {
		t.Run(tc.language, func(t *testing.T) {
			spans := highlightCode(tc.source, tc.language, codeTheme)
			var reconstructed strings.Builder
			tokens := map[chroma.TokenType]bool{}
			for _, s := range spans {
				reconstructed.WriteString(s.text)
				tokens[s.token] = true
				if s.style&mono == 0 {
					t.Fatal("code lost monospace face")
				}
			}
			if reconstructed.String() != tc.source {
				t.Fatal("tokenization changed source")
			}
			if len(tokens) < 2 {
				t.Fatal("missing token types")
			}
		})
	}
	for _, language := range []string{"", "not-a-real-language"} {
		source := "  unknown\n\n\tcode café\n"
		spans := highlightCode(source, language, codeTheme)
		if len(spans) != 1 || spans[0].text != source || spans[0].ink.A != 0 || spans[0].token != 0 {
			t.Fatal("unknown/untagged code must stay plain")
		}
	}
}

func TestHighlightedRasterResizeAndThemeCache(t *testing.T) {
	source := "func greeting() string {\n    return \"Hello, café!\"\n}"
	p, err := prepareForTest(testInput(t, "```go\n"+source+"\n```\n"))
	if err != nil {
		t.Fatal(err)
	}
	e := testEngine(t)
	wide := e.layout(p, 800).viewport(context.Background(), 0, 300, paletteFor("light"))
	narrow := e.layout(p, 400).viewport(context.Background(), 0, 400, paletteFor("light"))
	fresh := testEngine(t).layout(p, 400).viewport(context.Background(), 0, 400, paletteFor("light"))
	if !bytes.Equal(narrow.Pix, fresh.Pix) {
		t.Fatal("highlight colors/spacing changed after resize")
	}
	colors := map[color.RGBA]bool{}
	for _, sp := range p.blocks[0].spans {
		if c := theme.Ink("light", sp.token); sp.token != 0 && c.R != c.G {
			colors[c] = false
		}
	}
	for y := 0; y < wide.Bounds().Dy(); y++ {
		for x := 0; x < wide.Bounds().Dx(); x++ {
			c := wide.RGBAAt(x, y)
			if _, ok := colors[c]; ok {
				colors[c] = true
			}
		}
	}
	found := 0
	for _, present := range colors {
		if present {
			found++
		}
	}
	if found < 2 {
		t.Fatal("token colors did not reach raster output")
	}
	a := e.shape(highlightCode(source, "go", codeTheme), 25)
	tinted := highlightCode(source, "go", codeTheme)
	tinted[0].ink = color.RGBA{1, 2, 3, 255}
	if b := e.shape(tinted, 25); a == b {
		t.Fatal("span colors collided in shape cache")
	}
}
