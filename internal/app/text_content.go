package app

import (
	"strings"

	"github.com/alecthomas/chroma/v2/quick"
	"github.com/charmbracelet/x/ansi"
	"nimbleterminal.dev/gloss/internal/document"
	"nimbleterminal.dev/gloss/internal/theme"
)

// fencedFileContent bypasses Markdown presentation for a text/source file
// represented by one fence. In particular, a large pipe need not pass every
// character through Glamour's indentation and styling writers before opening.
// JSON records, which start with record headings, keep the Markdown renderer.
func fencedFileContent(doc *document.Markdown, themeName string) *markdownContent {
	source := string(doc.Source)
	header, rest, ok := strings.Cut(source, "\n")
	if !ok || !strings.HasPrefix(header, "```") {
		return nil
	}
	ticks := len(header) - len(strings.TrimLeft(header, "`"))
	fence := header[:ticks]
	end := strings.Index("\n"+rest, "\n"+fence+"\n")
	if end < 0 {
		return nil
	}
	code := rest[:end]
	tail := rest[end+len(fence)+1:]
	language := strings.TrimSpace(header[ticks:])
	code = strings.ReplaceAll(code, "\t", "    ")
	styled := code
	if language != "" && language != "text" && language != "plaintext" {
		var out strings.Builder
		if err := quick.Highlight(&out, code, language, "terminal16m", theme.Syntax(themeName)); err == nil {
			styled = out.String()
		}
	}
	c := &markdownContent{}
	for line := range strings.SplitSeq(strings.TrimSuffix(styled, "\n"), "\n") {
		c.line(line)
	}
	c.fileLines = len(c.units)
	if strings.TrimSpace(tail) != "" {
		note := renderedContent(&document.Markdown{Source: []byte(tail)}, themeName)
		for _, unit := range note.units {
			c.line(unit.styled)
		}
	}
	return c
}

func wrapFileUnit(unit textUnit, id, width int) []markdownLine {
	// Most source lines fit. Avoid cutting, wrapping, and scanning their ANSI
	// escapes repeatedly; the one span still supports search and line numbers.
	if ansi.StringWidth(unit.plain) <= width {
		return []markdownLine{{text: unit.styled, image: -1, spans: []textSpan{{unit: id, end: len(unit.plain)}}}}
	}
	return wrapTextUnit(unit, id, width, true)
}
