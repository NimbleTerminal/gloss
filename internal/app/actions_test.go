package app

import (
	"image"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// menuWith opens an action menu of two entries over a loaded picture and
// counts how often each ran.
func menuWith(t *testing.T, at image.Point) (m *Model, ran *[2]int) {
	t.Helper()
	m = loaded(t, "photos/landscape.png")
	ran = new([2]int)
	m.openActions("A picture", []menuEntry{
		{label: "First action", do: func(*Model) tea.Cmd { ran[0]++; return nil }},
		{label: "Second action", do: func(*Model) tea.Cmd { ran[1]++; return nil }},
	}, at)
	if m.layer != layerActions || m.actions == nil {
		t.Fatalf("the menu did not open: layer=%v", m.layer)
	}
	return m, ran
}

func TestActionMenuKeys(t *testing.T) {
	m, ran := menuWith(t, image.Pt(10, 5))
	m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if m.actions.cursor != 0 {
		t.Fatalf("up at the top moved the cursor to %d", m.actions.cursor)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if *ran != [2]int{0, 1} {
		t.Fatalf("ran=%v, want only the second entry once", *ran)
	}
	if m.layer != layerNone || m.actions != nil {
		t.Fatalf("the menu stayed open: layer=%v", m.layer)
	}

	m, ran = menuWith(t, image.Pt(10, 5))
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if *ran != [2]int{} || m.layer != layerNone || m.actions != nil {
		t.Fatalf("esc ran=%v layer=%v", *ran, m.layer)
	}
}

func TestActionMenuMouse(t *testing.T) {
	m, ran := menuWith(t, image.Pt(10, 5))
	rows(m) // The box is placed when it is drawn.
	box := m.actions.box
	if box.Empty() {
		t.Fatal("the menu was not placed")
	}
	// Border, title, then the entries.
	m.Update(tea.MouseClickMsg{X: box.Min.X + 3, Y: box.Min.Y + 3, Button: tea.MouseLeft})
	if *ran != [2]int{0, 1} || m.layer != layerNone {
		t.Fatalf("click on the second entry: ran=%v layer=%v", *ran, m.layer)
	}

	m, ran = menuWith(t, image.Pt(10, 5))
	rows(m)
	box = m.actions.box
	m.Update(tea.MouseClickMsg{X: box.Max.X + 2, Y: box.Max.Y + 2, Button: tea.MouseLeft})
	if *ran != [2]int{} || m.layer != layerNone || m.actions != nil {
		t.Fatalf("click outside: ran=%v layer=%v", *ran, m.layer)
	}

	// The title is not an entry: a click on it keeps the menu.
	m, ran = menuWith(t, image.Pt(10, 5))
	rows(m)
	box = m.actions.box
	m.Update(tea.MouseClickMsg{X: box.Min.X + 3, Y: box.Min.Y + 1, Button: tea.MouseLeft})
	if *ran != [2]int{} || m.layer != layerActions {
		t.Fatalf("click on the title: ran=%v layer=%v", *ran, m.layer)
	}
}

func TestActionMenuStaysOnScreen(t *testing.T) {
	m, _ := menuWith(t, image.Pt(99, 27))
	out := strings.Join(rows(m), "\n")
	for _, want := range []string{"A picture", "First action", "Second action"} {
		if !strings.Contains(out, want) {
			t.Fatalf("the menu is cut off at the corner, lacks %q:\n%s", want, out)
		}
	}
	if box := m.actions.box; box.Min.X < 0 || box.Min.Y < 0 || box.Max.X > 100 || box.Max.Y > m.bodyHeight() {
		t.Fatalf("box %v leaves the 100x%d body", box, m.bodyHeight())
	}

	m, _ = menuWith(t, image.Pt(10, 5))
	m.Update(tea.WindowSizeMsg{Width: 20, Height: 5})
	got := rows(m)
	if strings.Contains(strings.Join(got, "\n"), "Second action") {
		t.Fatalf("a box was drawn in a body too small for it:\n%s", strings.Join(got, "\n"))
	}
	if bar := got[len(got)-2]; !strings.Contains(bar, "enlarge terminal") {
		t.Fatalf("the bar does not say to enlarge the terminal: %q", bar)
	}
}

func TestActionMenuIgnoresClicksUnderLayers(t *testing.T) {
	right := tea.MouseClickMsg{X: 5, Y: 5, Button: tea.MouseRight}
	picture := func() *Model {
		m := loaded(t, "photos/landscape.png")
		m.opts.CopyImage = func([]byte) error { return nil }
		// Without a layer over it, this click opens the menu.
		if m.Update(right); m.layer != layerActions {
			t.Fatalf("a right-click on a picture opened no menu: layer=%v", m.layer)
		}
		m.closeLayer()
		return m
	}

	m := picture()
	m.Update(press("i"))
	m.Update(right)
	if m.layer != layerInfo || m.actions != nil {
		t.Fatalf("a right-click under the info box: layer=%v actions=%v", m.layer, m.actions)
	}

	m = picture()
	m.Update(press("?"))
	m.Update(right)
	if !m.help || m.layer != layerNone || m.actions != nil {
		t.Fatalf("a right-click under help: help=%v layer=%v actions=%v", m.help, m.layer, m.actions)
	}
}

func TestActionMenuStatus(t *testing.T) {
	m, _ := menuWith(t, image.Pt(10, 5))
	got := rows(m)
	if keys := strings.TrimRight(got[len(got)-1], " "); keys != " ↑/↓ choose · Enter do · Esc close" {
		t.Fatalf("keys line = %q", keys)
	}
}

func TestOverlayAtKeepsWhatTheBoxDoesNotCover(t *testing.T) {
	got := strings.Split(overlayAt("0123456789\nabcdefghij", "XY", 10, 3, 1), "\n")
	if got[0] != "0123456789" {
		t.Fatalf("a row above the box changed: %q", got[0])
	}
	if plain := ansi.Strip(got[1]); plain != "abcXYfghij" {
		t.Fatalf("row under the box = %q", plain)
	}
}

func TestActionMenuKeepsTheMouseOn(t *testing.T) {
	m, _ := menuWith(t, image.Pt(10, 5))
	if mode := m.View().MouseMode; mode != tea.MouseModeCellMotion {
		t.Fatalf("mouse mode with the menu open = %v", mode)
	}
}

// On a screen narrower than the widest box, the box narrows with it.
func TestActionMenuFitsANarrowScreen(t *testing.T) {
	m, _ := menuWith(t, image.Pt(3, 3))
	m.actions.title = strings.Repeat("A long title for a picture ", 3)
	m.Update(tea.WindowSizeMsg{Width: 28, Height: 20})
	for i, row := range rows(m) {
		if w := ansi.StringWidth(row); w > 28 {
			t.Fatalf("row %d is %d cells: %q", i, w, row)
		}
	}
	if box := m.actions.box; box.Empty() || box.Max.X > 28 {
		t.Fatalf("box %v", box)
	}
}
