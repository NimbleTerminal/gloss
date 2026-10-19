package app

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"nimbleterminal.dev/gloss/internal/theme"
)

type themePicker struct {
	choices          []theme.Choice
	cursor           int
	original, option string
}
type themeSaved struct{ err error }

func (m *Model) openThemePicker() tea.Cmd {
	choices := []theme.Choice{{Name: "auto", Ref: "auto"}}
	available := m.opts.ThemeChoices
	if len(available) == 0 {
		available = theme.Builtins()
	}
	seen := map[string]bool{"auto": true}
	for _, c := range available {
		if !seen[c.Ref] {
			choices = append(choices, c)
			seen[c.Ref] = true
		}
	}
	if !seen[m.theme] {
		choices = append(choices, theme.Choice{Name: theme.Name(m.theme), Ref: m.theme})
	}
	p := &themePicker{choices: choices, original: m.theme, option: m.opts.Theme}
	for i, c := range choices {
		if c.Ref == m.opts.Theme {
			p.cursor = i
		}
	}
	m.themePicker = p
	m.layer = layerTheme
	return nil
}
func (m *Model) themePickerKey(k string) tea.Cmd {
	p := m.themePicker
	if p == nil {
		return nil
	}
	switch k {
	case "esc", "q", "T":
		return m.closeLayer()
	case "ctrl+c":
		return m.quit()
	case "up", "k":
		p.cursor = max(0, p.cursor-1)
	case "down", "j":
		p.cursor = min(len(p.choices)-1, p.cursor+1)
	case "enter":
		ref := p.choices[p.cursor].Ref
		m.opts.Theme = ref
		m.themePicker = nil
		m.layer = layerNone
		cmd := m.settleDiagrams()
		if ref == "auto" {
			cmd = tea.Batch(cmd, m.themeRequest())
		}
		if save := m.opts.SaveTheme; save != nil {
			return tea.Batch(cmd, func() tea.Msg { return themeSaved{save(ref)} })
		}
		return cmd
	default:
		return nil
	}
	ref := p.choices[p.cursor].Ref
	// Auto previews the last detected terminal theme; confirming it requests a
	// fresh background reply through the normal event loop.
	if ref == "auto" {
		ref = m.detectedTheme
	}
	return m.applyTheme(ref)
}
func (m *Model) themePickerView(w, h int) string {
	p := m.themePicker
	if p == nil || w < 24 || h < 8 {
		return ""
	}
	width := max(1, min(46, w-4))
	rows := max(1, h-6)
	start := max(0, min(p.cursor-rows/2, len(p.choices)-rows))
	var b strings.Builder
	b.WriteString("Themes\n")
	for i := start; i < min(len(p.choices), start+rows); i++ {
		c := p.choices[i]
		label := "  " + c.Name
		if i == p.cursor {
			label = "› " + c.Name
		}
		row := ansi.Truncate(label, width, "…")
		style := lipgloss.NewStyle().Foreground(m.palette().Foreground)
		if i == p.cursor {
			style = style.Foreground(m.palette().Accent).Bold(true)
		}
		b.WriteString(style.Render(row) + "\n")
	}
	b.WriteString(ansi.Truncate(fmt.Sprintf("%d/%d · ↑/↓ preview · Enter keep", p.cursor+1, len(p.choices)), width, "…"))
	b.WriteString("\n" + ansi.Truncate("Esc restores previous theme", width, "…"))
	return lipgloss.NewStyle().Foreground(m.palette().Foreground).Background(m.palette().Background).Border(lipgloss.RoundedBorder()).BorderForeground(m.palette().Accent).Width(width + 2).Render(b.String())
}
