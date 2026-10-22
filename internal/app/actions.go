package app

import (
	"image"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// menuEntry is one thing the action menu can do.
type menuEntry struct {
	label string
	do    func(m *Model) tea.Cmd
}

// actionMenu is the small menu over the document that lists what can be
// done with the thing it was opened on.
type actionMenu struct {
	title   string
	entries []menuEntry
	cursor  int
	at      image.Point     // The cell the box is placed beside: an item's corner or the pointer.
	box     image.Rectangle // Where it was last drawn, for clicks.
}

// actionsMinW and actionsMinH are the body below which no menu is drawn.
const (
	actionsMinW = 24
	actionsMinH = 6
	actionsMaxW = 40 // The widest the box gets, border included.
)

// openActions raises a menu of entries beside at. With none there is no
// menu, and no note.
func (m *Model) openActions(title string, entries []menuEntry, at image.Point) tea.Cmd {
	if len(entries) == 0 {
		return nil
	}
	cmd := m.closeLayer()
	m.actions = &actionMenu{title: title, entries: entries, at: at}
	m.layer = layerActions
	return cmd
}

// actionsTooSmall says the body has no room for the menu.
func (m *Model) actionsTooSmall() bool {
	return m.width < actionsMinW || m.bodyHeight() < actionsMinH
}

// window is how many entries show at once, and the first of them: the
// cursor stays in view when the body is shorter than the list.
func (a *actionMenu) window(h int) (first, count int) {
	count = max(1, min(len(a.entries), h-3)) // Border twice, and the title.
	first = max(0, min(a.cursor-count/2, len(a.entries)-count))
	return first, count
}

// actionsLayout places the box in a w×h body, noting where for clicks, and
// returns its text. It is empty where the body is too small.
func (m *Model) actionsLayout(w, h int) string {
	a := m.actions
	if a == nil || w < actionsMinW || h < actionsMinH {
		if a != nil {
			a.box = image.Rectangle{}
		}
		return ""
	}
	inner := 0
	for _, e := range a.entries {
		inner = max(inner, 2+ansi.StringWidth(safe(e.label)))
	}
	title := safe(a.title)
	inner = min(actionsMaxW-4, max(inner, ansi.StringWidth(title)))
	first, count := a.window(h)

	text := m.boxText()
	heading := text.Bold(true).Foreground(m.palette().Accent)
	lines := []string{heading.Render(ansi.Truncate(title, inner, "…"))}
	for i := first; i < first+count; i++ {
		mark, style := "  ", text
		if i == a.cursor {
			mark, style = "› ", text.Bold(true)
		}
		lines = append(lines, style.Render(ansi.Truncate(mark+safe(a.entries[i].label), inner, "…")))
	}
	box := themedBox(lines, 1, m.palette())

	// Right of and below the cell, or left of and above it where that runs off.
	bw, bh := inner+4, len(lines)+2
	x, y := a.at.X+1, a.at.Y+1
	if x+bw > w {
		x = a.at.X - bw
	}
	if y+bh > h {
		y = a.at.Y - bh
	}
	x, y = max(0, min(x, w-bw)), max(0, min(y, h-bh))
	a.box = image.Rect(x, y, x+bw, y+bh)
	return box
}

// actionsView is the menu's box for a w×h body, empty where it will not fit.
func (m *Model) actionsView(w, h int) string { return m.actionsLayout(w, h) }

// actionsKey gives the open menu its keys. It takes all of them: nothing
// reaches the document beneath.
func (m *Model) actionsKey(k string) (tea.Cmd, bool) {
	a := m.actions
	if a == nil {
		m.layer = layerNone
		return nil, false
	}
	switch k {
	case "esc":
		return m.closeLayer(), true
	case "up", "k":
		a.cursor = max(0, a.cursor-1)
	case "down", "j":
		a.cursor = min(len(a.entries)-1, a.cursor+1)
	case "enter":
		if !m.actionsTooSmall() {
			return m.runAction(a.cursor), true
		}
	}
	return nil, true
}

// runAction closes the menu, then does the entry.
func (m *Model) runAction(i int) tea.Cmd {
	entry := m.actions.entries[i]
	return tea.Batch(m.closeLayer(), entry.do(m))
}

// actionsMouse takes every mouse message while the menu is open: a click
// on an entry does it, a click outside closes the menu, and the document
// beneath sees none of them. msg is in the body's cells.
func (m *Model) actionsMouse(msg tea.Msg) (tea.Cmd, bool) {
	a := m.actions
	if _, ok := msg.(tea.MouseMsg); !ok {
		return nil, false
	}
	if a == nil {
		m.layer = layerNone
		return nil, false
	}
	click, ok := msg.(tea.MouseClickMsg)
	if !ok {
		return nil, true
	}
	m.actionsLayout(m.width, m.bodyHeight())
	at := image.Pt(click.X, click.Y)
	if !at.In(a.box) {
		return m.closeLayer(), true
	}
	first, count := a.window(m.bodyHeight())
	if i := click.Y - a.box.Min.Y - 2; click.Button == tea.MouseLeft && i >= 0 && i < count {
		a.cursor = first + i
		return m.runAction(first + i), true
	}
	return nil, true
}

// actionsNote is what the status bar says while the menu has no room to
// be drawn.
func (m *Model) actionsNote() string {
	if m.layer == layerActions && m.actionsTooSmall() {
		return "enlarge terminal"
	}
	return ""
}
