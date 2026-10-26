package app

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"nimbleterminal.dev/gloss/internal/i18n"
)

// langChoice is a row of the language picker: Tag "" is System, the
// language gloss starts in without a saved choice.
type langChoice struct{ Tag, Label string }

type langPicker struct {
	choices  []langChoice
	cursor   int
	original string // opts.Lang when the picker opened
}
type langSaved struct{ err error }

// langChoices are System, then each catalog by its own name, with the
// English one beside it.
func langChoices() []langChoice {
	choices := []langChoice{{"", i18n.T("app.lang.system", "System (from LANG)")}}
	for _, l := range i18n.Languages() {
		label := l.Name
		if l.English != l.Name {
			label += " (" + l.English + ")"
		}
		choices = append(choices, langChoice{l.Tag, label})
	}
	return choices
}

// setLang makes tag gloss's language and the viewer's.
func (m *Model) setLang(tag string) {
	m.opts.Lang = tag
	i18n.SetLanguage(tag)
	if m.preview != nil {
		m.preview.opts.Lang = tag
	}
}

// langTag is what choosing c sets opts.Lang to.
func (m *Model) langTag(c langChoice) string {
	if c.Tag == "" {
		return m.opts.SystemLang
	}
	return c.Tag
}

func (m *Model) openLangPicker() tea.Cmd {
	p := &langPicker{choices: langChoices(), original: m.opts.Lang}
	if current := i18n.Match(m.opts.Lang); current != i18n.Match(m.opts.SystemLang) {
		for i, c := range p.choices {
			if c.Tag == current {
				p.cursor = i
			}
		}
	}
	m.langPicker = p
	m.layer = layerLang
	return nil
}

func (m *Model) langPickerKey(k string) tea.Cmd {
	p := m.langPicker
	if p == nil {
		return nil
	}
	switch k {
	case "esc", "q", "f2":
		return m.closeLayer()
	case "ctrl+c":
		return m.quit()
	case "up", "k":
		p.cursor = max(0, p.cursor-1)
	case "down", "j":
		p.cursor = min(len(p.choices)-1, p.cursor+1)
	case "home":
		p.cursor = 0
	case "end":
		p.cursor = len(p.choices) - 1
	case "enter":
		choice := p.choices[p.cursor]
		m.setLang(m.langTag(choice))
		m.langPicker = nil
		m.layer = layerNone
		var cmd tea.Cmd
		if i18n.Match(m.opts.Lang) != i18n.Match(p.original) {
			cmd = m.reloadForLang()
		}
		if save := m.opts.SaveLang; save != nil {
			return tea.Batch(cmd, func() tea.Msg { return langSaved{save(choice.Tag)} })
		}
		return cmd
	default:
		return nil
	}
	m.setLang(m.langTag(p.choices[p.cursor]))
	return nil
}

// reloadForLang loads the document again, in place, so that what the loader
// wrote (the info box's numbers) follows the new language.
func (m *Model) reloadForLang() tea.Cmd {
	if m.suspended || m.screen != screenDocument {
		return nil
	}
	if m.markdown != nil {
		m.savedMarkdown = m.markdown
	}
	return m.load(false)
}

func (m *Model) langPickerView(w, h int) string {
	p := m.langPicker
	if p == nil || w < 24 || h < 8 {
		return ""
	}
	// The labels are made again, so System is in the language previewed.
	p.choices = langChoices()
	width := max(1, min(46, w-4))
	rows := max(1, h-6)
	start := max(0, min(p.cursor-rows/2, len(p.choices)-rows))
	var b strings.Builder
	b.WriteString(ansi.Truncate(i18n.T("app.lang.title", "Languages"), width, "…") + "\n")
	for i := start; i < min(len(p.choices), start+rows); i++ {
		label := "  " + p.choices[i].Label
		style := lipgloss.NewStyle().Foreground(m.palette().Foreground)
		if i == p.cursor {
			label = "› " + p.choices[i].Label
			style = style.Foreground(m.palette().Accent).Bold(true)
		}
		b.WriteString(style.Render(ansi.Truncate(label, width, "…")) + "\n")
	}
	b.WriteString(ansi.Truncate(i18n.Tf("app.lang.hint", "%d/%d · ↑/↓ preview · Enter keep", p.cursor+1, len(p.choices)), width, "…"))
	b.WriteString("\n" + ansi.Truncate(i18n.T("app.lang.restore", "Esc restores previous language"), width, "…"))
	return lipgloss.NewStyle().Foreground(m.palette().Foreground).Background(m.palette().Background).Border(lipgloss.RoundedBorder()).BorderForeground(m.palette().Accent).Width(width + 2).Render(b.String())
}
