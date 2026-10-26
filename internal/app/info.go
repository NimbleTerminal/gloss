package app

import (
	"os"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"nimbleterminal.dev/gloss/internal/document"
	"nimbleterminal.dev/gloss/internal/i18n"
	"nimbleterminal.dev/gloss/internal/theme"
	ntpicture "nimbleterminal.dev/ntpicture"
)

// infoBox draws the details of the current file, no larger than w by h cells.
// It returns "" when the terminal has no room for a box.
func (m *Model) infoBox(w, h int) string {
	inner, rows := min(56, w-8), h-2
	if inner < 16 || rows < 2 {
		return ""
	}
	fields := append(append([]document.Field(nil), m.fields...), m.terminalFields()...)
	labels := 0
	for _, f := range fields {
		if f.Value != "" {
			labels = max(labels, lipgloss.Width(safe(infoLabel(f.Label))))
		}
	}
	text := m.boxText()
	heading, label := text.Bold(true).Foreground(m.palette().Accent), text.Foreground(m.palette().Muted)
	var lines []string
	for _, f := range fields {
		if f.Value == "" {
			lines = append(lines, heading.Render(ansi.Truncate(safe(infoLabel(f.Label)), inner, "…")))
			continue
		}
		name, value := safe(infoLabel(f.Label)), safe(f.Value)
		room := max(1, inner-labels-3)
		if over := ansi.StringWidth(value) - room; over > 0 && f.Label == "Path" {
			// The name at the end of a path says more than the root at its start.
			value = "…" + ansi.TruncateLeft(value, over+1, "")
		}
		value = ansi.Truncate(value, room, "…")
		lines = append(lines, label.Render(" "+name+strings.Repeat(" ", labels-ansi.StringWidth(name)+2))+text.Render(value))
	}
	if len(lines) == 0 {
		lines = []string{text.Render(i18n.T("app.info.none", "No details yet."))}
	}
	if len(lines) > rows {
		lines = append(lines[:rows-1], text.Render("…"))
	}
	return themedBox(lines, 1, m.palette())
}

// terminalFields describes the terminal and how gloss draws on it: what
// the box says when there is no file, and what it ends with when there is.
func (m *Model) terminalFields() []document.Field {
	size := i18n.Tf("app.info.cells", "%d × %d cells", m.width, m.height)
	if cw, ch := m.cellPixelSize(); cw > 1 && ch > 1 {
		size += i18n.Tf("app.info.cellPixels", ", %d × %d pixels each", cw, ch)
	}
	pictures := i18n.T("app.info.glyphs", "glyphs")
	if m.kitty {
		pictures = i18n.T("app.info.kitty", "Kitty graphics")
	}
	if m.opts.Render == "glyph" {
		pictures += i18n.T("app.info.askedFor", " (asked for)")
	}
	kitty := map[ntpicture.KittyCapability]string{
		ntpicture.KittyCapabilitySupported:   i18n.T("app.info.supported", "supported"),
		ntpicture.KittyCapabilityUnsupported: i18n.T("app.info.unsupported", "not supported"),
	}[m.terminal.Snapshot().Kitty]
	if kitty == "" {
		kitty = i18n.T("app.info.unknown", "not yet known")
	}
	meshes := map[string]string{
		"software":  i18n.T("app.info.software", "software (asked for)"),
		"wireframe": i18n.T("app.info.wireframe", "wireframe (asked for)"),
	}[m.opts.Render3D]
	if meshes == "" {
		meshes = i18n.T("app.info.webgpu", "WebGPU, or software without it")
	}
	if m.chart != nil {
		meshes = m.chart.RenderMode().String()
	}
	program := os.Getenv("TERM_PROGRAM")
	if program != "" {
		program = strings.TrimSpace(program + " " + os.Getenv("TERM_PROGRAM_VERSION"))
	} else {
		program = os.Getenv("TERM")
	}
	multiplexer := ""
	if os.Getenv("TMUX") != "" {
		multiplexer = "tmux"
	}
	screen := i18n.T("app.info.alternate", "alternate")
	if m.opts.KeepScreen {
		screen = i18n.T("app.info.mainScreen", "main, kept after quitting")
	}
	return document.Section("Terminal",
		document.Field{Label: "Size", Value: size},
		document.Field{Label: "Text theme", Value: theme.Name(m.theme)},
		document.Field{Label: "Pictures", Value: pictures},
		document.Field{Label: "Kitty graphics", Value: kitty},
		document.Field{Label: "Meshes", Value: meshes},
		document.Field{Label: "Program", Value: program},
		document.Field{Label: "Multiplexer", Value: multiplexer},
		document.Field{Label: "Screen", Value: screen},
	)
}

func (m *Model) boxText() lipgloss.Style {
	return lipgloss.NewStyle().Background(m.palette().Header).Foreground(m.palette().Foreground)
}

func themedBox(lines []string, padding int, p theme.Palette) string {
	shade := p.Header
	text := lipgloss.NewStyle().Background(shade).Foreground(p.Foreground)
	width := 0
	for _, line := range lines {
		width = max(width, lipgloss.Width(line))
	}
	for i, line := range lines {
		lines[i] = line + text.Render(strings.Repeat(" ", width-lipgloss.Width(line)))
	}
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(p.Border).BorderBackground(shade).
		Background(shade).Padding(0, padding).Render(strings.Join(lines, "\n"))
}

// overlay lays box over the right of body, which is w cells wide, top rows
// below its first.
// Rows are cut between cells, so a Kitty image keeps the placeholders that
// remain visible and gives up only those the box covers.
func overlay(body, box string, w, top int) string {
	return overlayRows(body, box, w, top, func(row int) int { return w - row })
}

// overlayAt lays box over body, which is w cells wide, with its left edge x
// cells in and top rows below the first; what the box does not cover stays.
func overlayAt(body, box string, w, x, top int) string {
	return overlayRows(body, box, w, top, func(int) int { return x })
}

// overlayRows lays each row of box over body at the cell where, given the
// row's width, the row is to start.
func overlayRows(body, box string, w, top int, start func(width int) int) string {
	if box == "" {
		return body
	}
	lines := strings.Split(body, "\n")
	for i, row := range strings.Split(box, "\n") {
		i += top
		if i >= len(lines) {
			break
		}
		rowWidth := ansi.StringWidth(row)
		x := max(0, start(rowWidth))
		left := ansi.Truncate(lines[i], x, "")
		line := left + "\x1b[m" + strings.Repeat(" ", max(0, x-ansi.StringWidth(left))) + row
		if end := x + rowWidth; end < w {
			line += "\x1b[m" + ansi.TruncateLeft(ansi.Truncate(lines[i], w, ""), end, "")
		}
		lines[i] = line
	}
	return strings.Join(lines, "\n")
}
