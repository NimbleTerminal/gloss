package app

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"image/color"
	"nimbleterminal.dev/gloss/internal/theme"
)

// Detection belongs to the application's event loop, never to a renderer or
// headless loader. Missing replies keep the interactive dark fallback.
func (m *Model) themeRequest() tea.Cmd {
	if m.isPreview || m.opts.Theme != "" && m.opts.Theme != "auto" {
		return nil
	}
	return tea.RequestBackgroundColor
}

func (m *Model) palette() theme.Palette { return theme.Get(m.theme) }

func (m *Model) pictureBackground() color.Color {
	if m.typeset != nil {
		return m.palette().Background
	}
	return pictureBackground
}

func (m *Model) statusStyle() lipgloss.Style {
	p := m.palette()
	return lipgloss.NewStyle().Foreground(p.Accent).Background(p.Header)
}

func (m *Model) applyTheme(name string) tea.Cmd {
	if m.theme == name {
		return nil
	}
	redraw := theme.Dark(m.theme) != theme.Dark(name) && m.diagrams() > 0
	m.theme = name
	var cmds []tea.Cmd
	if m.preview != nil {
		cmds = append(cmds, m.preview.applyTheme(name))
	}
	if redraw && !m.suspended && m.screen == screenDocument {
		// Mermaid diagrams are drawn in a light or a dark palette when the
		// document loads, so crossing between them loads it again, in place.
		if m.markdown != nil {
			m.savedMarkdown = m.markdown
		}
		return tea.Batch(append(cmds, m.load(false))...)
	}
	if m.markdown != nil {
		cmds = append(cmds, m.layoutMarkdown())
	}
	if m.typeset != nil {
		cmds = append(cmds, m.queueTypesetReflow())
	}
	return tea.Batch(cmds...)
}

// diagrams counts the Mermaid diagrams of the document shown.
func (m *Model) diagrams() int {
	switch {
	case m.markdown != nil:
		return m.markdown.doc.Diagrams
	case m.typeset != nil:
		return m.typeset.Diagrams
	case m.kind == "mermaid":
		return 1
	}
	return 0
}
