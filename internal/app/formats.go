package app

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
	"nimbleterminal.dev/gloss/internal/document"
	"nimbleterminal.dev/gloss/internal/i18n"
)

// formatLabel is a group of formats' label in the language in use.
func formatLabel(g document.FormatGroup) string {
	switch g.Label {
	case "Images":
		return i18n.T("app.formats.images", "Images")
	case "Doc":
		return i18n.T("app.formats.doc", "Doc")
	case "Text":
		return i18n.T("app.formats.text", "Text")
	case "3D":
		return i18n.T("app.formats.3d", "3D")
	case "Maps":
		return i18n.T("app.formats.maps", "Maps")
	}
	return g.Label
}

// formatNames lists a group's formats with the words the language puts
// around them.
func formatNames(g document.FormatGroup) string {
	names := strings.Join(g.Names, i18n.T("app.formats.comma", ", "))
	if len(g.Names) < 2 {
		return names
	}
	head, last := strings.Join(g.Names[:len(g.Names)-1], i18n.T("app.formats.comma", ", ")), g.Names[len(g.Names)-1]
	switch g.Label {
	case "Text":
		return i18n.Tf("app.formats.textNames", "%s, plain text and source", names)
	case "3D":
		return i18n.Tf("app.formats.meshes", "%s and %s meshes", head, last)
	case "Maps":
		return i18n.Tf("app.formats.mapNames", "%s, and raster %s", head, last)
	}
	return names
}

// formatLines says what gloss opens, a labelled line a group, with the
// names lined up after the widest label.
func formatLines() []string {
	labels := make([]string, len(document.FormatGroups))
	widest := 0
	for i, g := range document.FormatGroups {
		labels[i] = i18n.Tf("app.formats.label", "%s:", formatLabel(g))
		widest = max(widest, ansi.StringWidth(labels[i]))
	}
	// A full-width colon carries its own space; any other label gets one.
	gap := 1
	if strings.HasSuffix(labels[0], "：") {
		gap = 0
	}
	lines := make([]string, len(labels))
	for i, g := range document.FormatGroups {
		lines[i] = labels[i] + strings.Repeat(" ", widest+gap-ansi.StringWidth(labels[i])) + formatNames(g)
	}
	return lines
}

// formatsShort says it in one line, for a screen with no room for the
// groups.
func formatsShort() string {
	return strings.Join(shortFormats(), i18n.T("app.formats.comma", ", "))
}

// shortFormats are the items of the short line: images, the documents by
// name, the text formats, 3D and maps.
func shortFormats() []string {
	var items []string
	for _, g := range document.FormatGroups {
		switch g.Label {
		case "Images", "3D":
			items = append(items, formatLabel(g))
		case "Text":
			// Plain text goes after the formats with markup.
			items = append(items, g.Names[:2]...)
			items = append(items, i18n.T("app.formats.plainText", "text"))
			items = append(items, g.Names[2:]...)
		case "Maps":
			items = append(items, i18n.T("app.formats.someMaps", "maps"))
		default:
			items = append(items, g.Names...)
		}
	}
	return items
}

// wrapList lays items joined by sep on lines of at most w cells, breaking
// only between items: after the comma, as a word wrap does at a space, even
// where the language's comma has no space after it.
func wrapList(items []string, sep string, w int) string {
	mark, gap := strings.TrimRight(sep, " "), ""
	if mark != sep {
		gap = " "
	}
	var lines []string
	line := ""
	for i, item := range items {
		if i < len(items)-1 {
			item += mark
		}
		switch {
		case line == "":
			line = item
		case ansi.StringWidth(line)+ansi.StringWidth(gap)+ansi.StringWidth(item) <= w:
			line += gap + item
		default:
			lines, line = append(lines, line), item
		}
	}
	return strings.Join(append(lines, line), "\n")
}
