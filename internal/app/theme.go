package app

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/NimbleTerminal/gloss/internal/theme"
	"image/color"
)

// The Nimble brand colours (see the brand guide). They dress the chrome the
// pager draws itself: the status bar, the corner boxes and the banner. The
// document stays as it is, and terminals of fewer colours get the nearest.
const (
	colorNimblePurpleDark  = "#3F3080"
	colorNimblePurpleLight = "#655BA7"
	colorNimbleRed         = "#E24F36"
	colorNimbleTeal        = "#4495AA"
	colorNimbleMint        = "#A7D7B1"
	colorNimbleYellow      = "#FBF4A5"

	// Text on the dark purple: the guide's white, and its mint for the dim
	// lines (the brand's own light purple is too faint to read there).
	colorNimbleText = "#FFFFFF"
	colorNimbleDim  = colorNimbleMint
)

// Chrome styles.
var (
	frameColor  = lipgloss.Color(colorNimblePurpleLight)
	bannerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(colorNimbleRed)).Bold(true)
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
