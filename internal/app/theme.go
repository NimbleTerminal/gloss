package app

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/NimbleTerminal/gloss/internal/theme"
	"image/color"
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
	m.theme = name
	var cmds []tea.Cmd
	if m.markdown != nil {
		cmds = append(cmds, m.layoutMarkdown())
	}
	if m.typeset != nil {
		cmds = append(cmds, m.queueTypesetReflow())
	}
	if m.preview != nil {
		cmds = append(cmds, m.preview.applyTheme(name))
	}
	return tea.Batch(cmds...)
}
