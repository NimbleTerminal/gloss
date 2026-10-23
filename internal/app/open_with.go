package app

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"errors"
	"fmt"
	"github.com/charmbracelet/x/ansi"
	"nimbleterminal.dev/gloss/internal/openwith"
	"nimbleterminal.dev/ntopenwith"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

type openWithPicker struct {
	managing   bool
	verb       openwith.Verb
	apps       []openwith.AppPreference
	target     openwith.Target
	choices    []openwith.Choice
	cursor     int
	busy       bool
	operation  int
	generation uint64
	table      bool // The file is a table's CSV: gloss opens it too, and its path is not worth copying.
}

// openWithLoaded lists the chooser's rows for verb. choose opens the chooser
// on them (a launch had no app to resolve); switching is Tab's reload, which
// keeps the chooser on its verb when it fails or lists nothing.
type openWithLoaded struct {
	operation         int
	generation        uint64
	verb              openwith.Verb
	target            openwith.Target
	choices           []openwith.Choice
	choose, switching bool
	err               error
}

// editorReady carries a terminal app's unstarted command, and the file it
// edits, to the editor handoff.
type editorReady struct {
	picker  *openWithPicker
	command *exec.Cmd
	target  openwith.Target
}
type openWithFinished struct {
	operation        int
	generation       uint64
	target           openwith.Target
	name             string
	temporary, saved bool
	err              error
}

// verbTitle is the chooser's name for verb: "Open with" or "Edit with".
func verbTitle(v openwith.Verb) string {
	if v == openwith.Edit {
		return "Edit with"
	}
	return "Open with"
}

// listed is the host's rows as the chooser shows them: for a table's CSV,
// gloss first and no Copy path.
func (p *openWithPicker) listed(choices []openwith.Choice) []openwith.Choice {
	if !p.table {
		return choices
	}
	choices = slices.DeleteFunc(slices.Clone(choices), func(c openwith.Choice) bool { return c.ID == "copy" })
	if p.verb == openwith.Open {
		choices = slices.Insert(choices, 0, openwith.Choice{ID: "gloss", Name: "gloss", Available: true})
	}
	return choices
}

// openWithChoices are the chooser's rows: the host's, and Copy path for a
// file being opened.
func openWithChoices(host openwith.Host, target openwith.Target, v openwith.Verb) ([]openwith.Choice, error) {
	choices, err := host.Choices(target, v)
	if err == nil && v == openwith.Open && !target.Directory {
		choices = append(choices, openwith.Choice{ID: "copy", Name: "Copy path", Available: true})
	}
	return choices, err
}

func (m *Model) requestOpenWith(path, kind string, directory, chooser bool, verb openwith.Verb) tea.Cmd {
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
	p := &openWithPicker{operation: m.openWithOperation, generation: m.generation, busy: true, verb: verb}
	m.openWithPicker = p
	if chooser {
		m.layer = layerOpenWith
	}
	host, op, generation := m.opts.OpenWith, p.operation, p.generation
	temporary := m.opts.IsStdin(path) || (m.opts.IsTemporary != nil && m.opts.IsTemporary(path))
	return func() tea.Msg {
		target, err := openWithTarget(path, kind, directory)
		target.Temporary = temporary
		if err != nil {
			return openWithLoaded{operation: op, generation: generation, verb: verb, err: err}
		}
		if !chooser {
			launched, err := host.Launch(target, verb, "")
			if err == nil && launched.Cmd != nil {
				return editorReady{p, launched.Cmd, target}
			}
			if !errors.Is(err, ntopenwith.ErrChoose) {
				return openWithFinished{operation: op, generation: generation, target: target, name: launched.Name, temporary: temporary, err: err}
			}
		}
		choices, err := openWithChoices(host, target, verb)
		return openWithLoaded{operation: op, generation: generation, verb: verb, target: target, choices: choices, choose: !chooser, err: err}
	}
}
func (m *Model) openWithCurrent(chooser bool, verb openwith.Verb) tea.Cmd {
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
	return m.requestOpenWith(path, kind, false, chooser, verb)
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
		m.note = verbTitle(v.verb) + ": " + safe(v.err.Error())
		if v.switching {
			p.busy = false
			return nil
		}
		return m.closeOpenWith()
	}
	if v.switching && len(v.choices) == 0 {
		p.busy = false
		m.note = "No app can edit " + safe(filepath.Base(p.target.Path))
		return nil
	}
	if v.choose {
		m.layer = layerOpenWith
	}
	p.verb, p.target, p.busy, p.cursor = v.verb, v.target, false, 0
	p.choices = p.listed(v.choices)
	for i, c := range p.choices {
		// A table's CSV is the viewer's own: gloss goes first.
		if (p.table && c.ID == "gloss") || (!p.table && c.Default) {
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
	if errors.Is(v.err, ntopenwith.ErrNoEditor) {
		m.note = "No app can edit " + safe(filepath.Base(v.target.Path))
		if m.layer != layerOpenWith {
			return m.closeOpenWith()
		}
		return nil
	}
	if v.err != nil {
		m.note = verbTitle(p.verb) + ": " + safe(v.err.Error())
		if m.layer != layerOpenWith {
			return m.closeOpenWith()
		}
		return nil
	}
	if v.saved {
		m.note = "Default saved for " + p.target.Kind + " (" + string(p.verb) + ")"
		for i := range p.choices {
			p.choices[i].Default = i == p.cursor
		}
		return nil
	}
	m.note = "Opened in " + safe(v.name)
	if p.verb == openwith.Edit {
		m.note = "Editing in " + safe(v.name)
	}
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
		if p.managing {
			if p.busy {
				return nil
			}
			return m.loadOpenWithApps(false, "", false)
		}
		return m.closeOpenWith()
	}
	if p.busy || m.width < 24 || m.bodyHeight() < 6 {
		return nil
	}
	if p.managing {
		switch msg.String() {
		case "up", "k":
			p.cursor = max(0, p.cursor-1)
		case "down", "j":
			p.cursor = min(len(p.apps)-1, p.cursor+1)
		case "enter":
			return m.loadOpenWithApps(false, "", false)
		case " ", "space":
			if p.cursor >= 0 && p.cursor < len(p.apps) {
				a := p.apps[p.cursor]
				return m.loadOpenWithApps(true, a.ID, !a.Enabled)
			}
		}
		return nil
	}
	switch msg.String() {
	case "m":
		if _, ok := m.opts.OpenWith.(openwith.Manager); ok {
			return m.loadOpenWithApps(true, "", false)
		}
	case "tab":
		return m.switchOpenWithVerb()
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
		if save && (c.ID == "copy" || c.ID == "reveal" || c.ID == "gloss" || p.target.Kind == "") {
			m.note = "This action cannot be a default"
			return nil
		}
		if c.ID == "copy" {
			target := p.target.Path
			m.closeOpenWith()
			m.note = "Path copied"
			return tea.SetClipboard(target)
		}
		if c.ID == "gloss" {
			path := p.target.Path
			m.closeOpenWith()
			return m.openTableInGloss(path)
		}
		return m.openWithLaunch(c.ID, save)
	}
	return nil
}

// switchOpenWithVerb lists the chooser's other verb for the same file.
func (m *Model) switchOpenWithVerb() tea.Cmd {
	p := m.openWithPicker
	if p.target.Directory {
		m.note = "Folders cannot be edited"
		return nil
	}
	verb := openwith.Edit
	if p.verb == openwith.Edit {
		verb = openwith.Open
	}
	p.busy = true
	m.openWithOperation++
	p.operation = m.openWithOperation
	op, generation, target, host := p.operation, p.generation, p.target, m.opts.OpenWith
	return func() tea.Msg {
		choices, err := openWithChoices(host, target, verb)
		return openWithLoaded{operation: op, generation: generation, verb: verb, target: target, choices: choices, switching: true, err: err}
	}
}

// openWithLaunch launches the chooser's app id, or saves it as the default.
// A terminal app's command goes to the editor handoff instead of finishing.
func (m *Model) openWithLaunch(id string, save bool) tea.Cmd {
	p := m.openWithPicker
	p.busy = true
	target, verb, op, generation, host := p.target, p.verb, p.operation, p.generation, m.opts.OpenWith
	return func() tea.Msg {
		var launched openwith.Launched
		var err error
		if save {
			err = host.SetDefault(target, verb, id)
		} else {
			launched, err = host.Launch(target, verb, id)
		}
		if err == nil && launched.Cmd != nil {
			return editorReady{p, launched.Cmd, target}
		}
		return openWithFinished{operation: op, generation: generation, target: target, name: launched.Name, temporary: target.Temporary, saved: save, err: err}
	}
}

// openWithView is the chooser as a box to lay over the screen, w by h: at
// most 60 columns of text and as many rows as fit.
func (m *Model) openWithView(w, h int) string {
	p := m.openWithPicker
	title := verbTitle(p.verb)
	if w < 24 || h < 6 {
		return ansi.Truncate(title+": enlarge terminal", w, "")
	}
	inner := min(60, w-4)
	header := title + " — " + safe(filepath.Base(p.target.Path))
	var lines []string
	rows := max(1, h-6) // The border, the heading and the gap under it.
	switch {
	case p.busy && len(p.choices) == 0 && !p.managing:
		header, lines = title+"…", nil
	case p.managing:
		header = "Manage apps — all formats"
		start := max(0, min(p.cursor-rows/2, len(p.apps)-rows))
		for i := start; i < min(len(p.apps), start+rows); i++ {
			a := p.apps[i]
			mark, check := "  ", "[ ] "
			if i == p.cursor {
				mark = "› "
			}
			if a.Enabled {
				check = "[x] "
			}
			lines = append(lines, mark+check+safe(a.Name))
		}
		if len(p.apps) == 0 {
			lines = append(lines, "No installed apps to manage")
		}
	default:
		start := max(0, min(p.cursor-rows/2, len(p.choices)-rows))
		for i := start; i < min(len(p.choices), start+rows); i++ {
			c := p.choices[i]
			mark := "  "
			if i == p.cursor {
				mark = "› "
			}
			label := safe(c.Name)
			if c.Default {
				label += " (default)"
			}
			if c.Terminal {
				label += " (terminal)"
			}
			if !c.Available {
				label += " (unavailable)"
			}
			lines = append(lines, mark+label)
		}
	}
	if p.busy && (len(p.choices) > 0 || p.managing) {
		lines = append(lines, "Working…")
	}
	// As wide as the widest row, within the screen.
	width := ansi.StringWidth(header)
	for _, line := range lines {
		width = max(width, ansi.StringWidth(line))
	}
	inner = min(inner, width)
	text := m.boxText()
	box := []string{text.Bold(true).Foreground(m.palette().Accent).Render(ansi.Truncate(header, inner, "…"))}
	if len(lines) > 0 {
		box = append(box, text.Render(""))
	}
	for _, line := range lines {
		style := text
		if strings.HasPrefix(line, "› ") {
			style = text.Bold(true) // The row the cursor is on.
		}
		box = append(box, style.Render(ansi.Truncate(line, inner, "…")))
	}
	return themedBox(box, 1, m.palette())
}

// overlayOpenWith lays the chooser over the screen beneath, in the middle
// across and a third of the way down.
func (m *Model) overlayOpenWith(body string, w, h int) string {
	box := m.openWithView(w, h)
	bw, bh := lipgloss.Width(box), lipgloss.Height(box)
	return overlayAt(body, box, w, max(0, (w-bw)/2), max(0, (h-bh)/3))
}

func (m *Model) openWithStatus(w int) (string, string) {
	hint := " ↑/↓ choose · Enter open · Tab edit with · d default"
	if m.openWithPicker != nil && m.openWithPicker.verb == openwith.Edit {
		hint = " ↑/↓ choose · Enter edit · Tab open with · d default"
	}
	if _, ok := m.opts.OpenWith.(openwith.Manager); ok {
		hint += " · m manage"
	}
	hint += " · Esc close"
	if m.openWithPicker != nil && m.openWithPicker.managing {
		hint = " ↑/↓ choose · Space toggle (saved) · Enter/Esc back"
	}
	return ansi.Truncate(fmt.Sprintf(" %s", m.note), w, "…"), ansi.Truncate(hint, w, "")
}

// App management uses the existing host boundary and event loop; it never launches.
type openWithAppsLoaded struct {
	operation  int
	generation uint64
	managing   bool
	apps       []openwith.AppPreference
	choices    []openwith.Choice
	err        error
}

func (m *Model) loadOpenWithApps(managing bool, id string, enabled bool) tea.Cmd {
	p := m.openWithPicker
	host := m.opts.OpenWith
	manager, ok := host.(openwith.Manager)
	if !ok {
		return nil
	}
	p.busy = true
	m.openWithOperation++
	p.operation = m.openWithOperation
	op, generation, target, verb := p.operation, p.generation, p.target, p.verb
	return func() tea.Msg {
		v := openWithAppsLoaded{operation: op, generation: generation, managing: managing}
		if id != "" {
			v.err = manager.SetEnabled(id, enabled)
		}
		if managing {
			v.apps = manager.Apps()
		} else {
			v.choices, v.err = openWithChoices(host, target, verb)
		}
		return v
	}
}

func (m *Model) openWithAppsLoaded(v openWithAppsLoaded) tea.Cmd {
	p := m.openWithPicker
	if p == nil || p.operation != v.operation || p.generation != v.generation {
		return nil
	}
	if m.generation != v.generation {
		return m.closeOpenWith()
	}
	p.busy = false
	if v.err != nil {
		m.note = "Manage apps: " + safe(v.err.Error())
	} else {
		m.note = ""
	}
	if p.managing != v.managing {
		p.cursor = 0
	}
	p.managing = v.managing
	if v.managing {
		p.apps = v.apps
	} else if v.err == nil {
		p.choices = p.listed(v.choices)
	}
	return nil
}
