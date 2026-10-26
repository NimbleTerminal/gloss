package app

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"image/color"
	"nimbleterminal.dev/gloss/internal/document"
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

// DocumentTheme is a resolved theme as the document loader takes it: gloss
// decides themes, and the loader only draws in them.
func DocumentTheme(name string) document.Theme {
	return document.Theme{Name: name, Dark: theme.Dark(name), Palette: theme.Typeset(name)}
}

// DocumentLocale is the user's language as the document loader takes it.
func DocumentLocale(tag string) document.Locale { return document.Locale{Tag: tag} }

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

// applyTheme recolors the document. While the theme picker is open, its
// diagrams keep their palette: they are drawn again once a theme is chosen.
func (m *Model) applyTheme(name string) tea.Cmd {
	return m.setTheme(name, m.themePicker == nil)
}

func (m *Model) setTheme(name string, redraw bool) tea.Cmd {
	if m.theme == name {
		return nil
	}
	m.theme = name
	var cmds []tea.Cmd
	if m.preview != nil {
		cmds = append(cmds, m.preview.setTheme(name, redraw))
	}
	if redraw {
		if load := m.redrawDiagrams(); load != nil {
			return tea.Batch(append(cmds, load)...)
		}
	}
	if m.markdown != nil {
		cmds = append(cmds, m.layoutMarkdown())
	}
	if m.typeset != nil {
		cmds = append(cmds, m.queueTypesetReflow())
	}
	return tea.Batch(cmds...)
}

// redrawDiagrams loads the document again, in place, when its Mermaid
// diagrams were drawn in the other palette (light or dark) from the theme's.
func (m *Model) redrawDiagrams() tea.Cmd {
	if m.diagrams() == 0 || theme.Dark(m.theme) == m.diagramsDark || m.suspended || m.screen != screenDocument {
		return nil
	}
	if m.markdown != nil {
		m.savedMarkdown = m.markdown
	}
	return m.load(false)
}

// settleDiagrams redraws the diagrams, here and in the preview, that a theme
// chosen in the picker left in the wrong palette.
func (m *Model) settleDiagrams() tea.Cmd {
	cmd := m.redrawDiagrams()
	if m.preview != nil {
		cmd = tea.Batch(cmd, m.preview.redrawDiagrams())
	}
	return cmd
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
