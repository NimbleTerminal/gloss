package app

import (
	tea "charm.land/bubbletea/v2"
	"fmt"
	"github.com/charmbracelet/x/ansi"
	"nimbleterminal.dev/gloss/internal/openwith"
	"path/filepath"
	"strings"
)

type openWithPicker struct {
	target     openwith.Target
	choices    []openwith.Choice
	cursor     int
	busy       bool
	operation  int
	generation uint64
}
type openWithLoaded struct {
	operation  int
	generation uint64
	target     openwith.Target
	choices    []openwith.Choice
	err        error
}
type openWithFinished struct {
	operation        int
	generation       uint64
	name             string
	temporary, saved bool
	err              error
}

func (m *Model) requestOpenWith(path, kind string, directory, chooser bool) tea.Cmd {
	if m.isPreview {
		return nil
	}
	if m.opts.OpenWith == nil {
		m.note = "Open with is not available in the browser or this host"
		return nil
	}
	if path == "" {
		m.note = "No file selected"
		return nil
	}
	if m.openWithPicker != nil {
		return nil
	}
	m.openWithOperation++
	p := &openWithPicker{operation: m.openWithOperation, generation: m.generation, busy: true}
	m.openWithPicker = p
	if chooser {
		m.layer = layerOpenWith
	}
	host, op, generation := m.opts.OpenWith, p.operation, p.generation
	temporary := m.opts.IsStdin(path) || (m.opts.IsTemporary != nil && m.opts.IsTemporary(path))
	return func() tea.Msg {
		target, err := openWithTarget(path, kind, directory)
		target.Temporary = temporary
		if !chooser {
			name := ""
			if err == nil {
				name, err = host.Launch(target, "")
			}
			return openWithFinished{operation: op, generation: generation, name: name, temporary: temporary, err: err}
		}
		var choices []openwith.Choice
		if err == nil {
			choices, err = host.Choices(target)
		}
		if err == nil && !directory {
			choices = append(choices, openwith.Choice{ID: "copy", Name: "Copy path", Available: true})
		}
		return openWithLoaded{op, generation, target, choices, err}
	}
}
func (m *Model) openWithCurrent(chooser bool) tea.Cmd {
	index := m.index
	kind := m.kind
	if m.listing() {
		index = m.selection
		kind = ""
	}
	path := ""
	if index >= 0 && index < len(m.opts.Files) {
		path = m.opts.Files[index]
	}
	return m.requestOpenWith(path, kind, false, chooser)
}
func (m *Model) openWithLoaded(v openWithLoaded) tea.Cmd {
	p := m.openWithPicker
	if p == nil || p.operation != v.operation || p.generation != v.generation {
		return nil
	}
	if m.generation != v.generation {
		return m.closeOpenWith()
	}
	if v.err != nil {
		m.note = "Open with: " + safe(v.err.Error())
		return m.closeOpenWith()
	}
	p.target, p.choices, p.busy = v.target, v.choices, false
	for i, c := range p.choices {
		if c.Default {
			p.cursor = i
			break
		}
	}
	return nil
}
func (m *Model) openWithFinished(v openWithFinished) tea.Cmd {
	p := m.openWithPicker
	if p == nil || p.operation != v.operation || p.generation != v.generation {
		return nil
	}
	if m.generation != v.generation {
		return m.closeOpenWith()
	}
	p.busy = false
	if v.err != nil {
		m.note = "Open with: " + safe(v.err.Error())
		if m.layer != layerOpenWith {
			return m.closeOpenWith()
		}
		return nil
	}
	if v.saved {
		m.note = "Default saved for " + p.target.Kind
		for i := range p.choices {
			p.choices[i].Default = i == p.cursor
		}
		return nil
	}
	m.note = "Opened in " + safe(v.name)
	if v.temporary {
		m.note += " — temporary file; save a copy in the app to keep it"
	}
	return m.closeOpenWith()
}
func (m *Model) closeOpenWith() tea.Cmd {
	m.openWithPicker = nil
	if m.layer == layerOpenWith {
		m.layer = layerNone
	}
	return nil
}
func (m *Model) openWithKey(msg tea.KeyPressMsg) tea.Cmd {
	p := m.openWithPicker
	switch msg.String() {
	case "ctrl+c":
		return m.quit()
	case "esc", "q":
		return m.closeOpenWith()
	}
	if p.busy || m.width < 24 || m.bodyHeight() < 6 {
		return nil
	}
	switch msg.String() {
	case "up", "k":
		p.cursor = max(0, p.cursor-1)
	case "down", "j":
		p.cursor = min(len(p.choices)-1, p.cursor+1)
	case "enter", "d":
		if p.cursor < 0 || p.cursor >= len(p.choices) {
			return nil
		}
		c := p.choices[p.cursor]
		if !c.Available {
			m.note = "Application is unavailable"
			return nil
		}
		save := msg.String() == "d"
		if save && (c.ID == "copy" || c.ID == "reveal" || p.target.Kind == "") {
			m.note = "This action cannot be a default"
			return nil
		}
		if c.ID == "copy" {
			target := p.target.Path
			m.closeOpenWith()
			m.note = "Path copied"
			return tea.SetClipboard(target)
		}
		p.busy = true
		target, op, generation, host := p.target, p.operation, p.generation, m.opts.OpenWith
		return func() tea.Msg {
			name := ""
			var err error
			if save {
				err = host.SetDefault(target.Kind, c.ID)
			} else {
				name, err = host.Launch(target, c.ID)
			}
			return openWithFinished{operation: op, generation: generation, name: name, temporary: target.Temporary, saved: save, err: err}
		}
	}
	return nil
}
func (m *Model) openWithView(w, h int) string {
	p := m.openWithPicker
	if w < 24 || h < 6 {
		return "Open with: enlarge terminal"
	}
	header := "Open with — " + safe(filepath.Base(p.target.Path))
	if p.busy && len(p.choices) == 0 {
		return "Open with…"
	}
	rows := max(1, h-4)
	start := max(0, min(p.cursor-rows/2, len(p.choices)-rows))
	lines := []string{ansi.Truncate(header, w, "…"), ""}
	for i := start; i < min(len(p.choices), start+rows); i++ {
		c := p.choices[i]
		mark := "  "
		if i == p.cursor {
			mark = "› "
		}
		label := c.Name
		if c.Default {
			label += " (default)"
		}
		if !c.Available {
			label += " (unavailable)"
		}
		lines = append(lines, ansi.Truncate(mark+label, w, "…"))
	}
	if p.busy {
		lines = append(lines, "Working…")
	}
	return strings.Join(lines, "\n")
}

func (m *Model) openWithStatus(w int) (string, string) {
	return ansi.Truncate(fmt.Sprintf(" %s", m.note), w, "…"), ansi.Truncate(" ↑/↓ choose · Enter open · d default · Esc close", w, "")
}
