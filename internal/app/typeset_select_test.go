package app

import (
	"fmt"
	"image"
	"strings"
	"testing"
	"testing/fstest"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func selectModel(t *testing.T) *Model {
	t.Helper()
	files := fstest.MapFS{"page.md": &fstest.MapFile{Data: []byte("# A typeset page\n\n" + strings.Repeat("Café and a paragraph of words.\n\n", 40))}}
	m := New(Options{Files: []string{"page.md"}, FilesFS: files, MarkdownMode: "typeset", Render: "glyph", DPI: 150, Page: 1})
	t.Cleanup(func() { _ = m.Close() })
	m.width, m.height = 80, 30
	drainAnimation(t, m, m.load(false))
	if m.typeset == nil || m.source == nil {
		t.Fatal("not a typeset page")
	}
	return m
}

func drag(t *testing.T, m *Model, from, to image.Point) {
	t.Helper()
	_, cmd := m.Update(tea.MouseClickMsg{X: from.X, Y: from.Y, Button: tea.MouseLeft})
	drainAnimation(t, m, cmd)
	_, cmd = m.Update(tea.MouseMotionMsg{X: to.X, Y: to.Y, Button: tea.MouseLeft})
	drainAnimation(t, m, cmd)
	_, cmd = m.Update(tea.MouseReleaseMsg{X: to.X, Y: to.Y, Button: tea.MouseLeft})
	drainAnimation(t, m, cmd)
}

func TestTypesetSelectionDragCopyAndClear(t *testing.T) {
	m := selectModel(t)
	plain := m.source
	_, cmd := m.Update(tea.MouseClickMsg{X: 4, Y: 1, Button: tea.MouseLeft})
	drainAnimation(t, m, cmd)
	_, cmd = m.Update(tea.MouseMotionMsg{X: 40, Y: 12, Button: tea.MouseLeft})
	drainAnimation(t, m, cmd)
	if m.typesetSelected == "" || !strings.Contains(m.typesetSelected, "paragraph") {
		t.Fatalf("drag selected %q", m.typesetSelected)
	}
	if s := m.typesetSelect; s == nil || !s.Anchor.Placed || !s.Focus.Placed {
		t.Fatalf("selection not placed after the result: %+v", m.typesetSelect)
	}
	if m.source == plain {
		t.Fatal("highlight not drawn")
	}
	// The drag goes on after a result: the selection grows with the mouse.
	grown := m.typesetSelected
	_, cmd = m.Update(tea.MouseMotionMsg{X: 70, Y: 20, Button: tea.MouseLeft})
	if cmd == nil {
		t.Fatal("motion after a result asked for nothing")
	}
	drainAnimation(t, m, cmd)
	if len(m.typesetSelected) <= len(grown) || !m.typesetSelect.Focus.Placed {
		t.Fatalf("selection did not grow: %q then %q", grown, m.typesetSelected)
	}
	_, cmd = m.Update(tea.MouseReleaseMsg{X: 70, Y: 20, Button: tea.MouseLeft})
	drainAnimation(t, m, cmd)
	_, hint := m.documentStatus(m.width, m.bodyHeight())
	if !strings.Contains(ansi.Strip(hint), "y copy") {
		t.Fatalf("hint %q", ansi.Strip(hint))
	}
	// Scrolling keeps the selection by its places.
	before := m.typesetSelected
	cmd, _ = m.documentKey("down")
	drainAnimation(t, m, cmd)
	if m.typesetSelected != before {
		t.Fatalf("scroll changed the selection %q", m.typesetSelected)
	}
	// y copies through the terminal.
	cmd, handled := m.documentKey("y")
	if !handled || cmd == nil {
		t.Fatal("y did not copy")
	}
	if got := fmt.Sprint(cmd()); got != before {
		t.Fatalf("clipboard got %q", got)
	}
	if !strings.Contains(m.note, "copied") {
		t.Fatalf("note %q", m.note)
	}
	// Esc clears it and the highlight.
	cmd, _ = m.documentKey("esc")
	drainAnimation(t, m, cmd)
	if m.typesetSelect != nil || m.typesetSelected != "" {
		t.Fatal("esc kept the selection")
	}
	if m.screen != screenDocument || m.typeset == nil || m.source == plain {
		t.Fatal("esc left the document instead of clearing the selection")
	}
	cmd, _ = m.documentKey("y")
	if cmd != nil {
		t.Fatal("y copied nothing")
	}
	// A click without a drag selects nothing; Shift-drag is the terminal's.
	_, cmd = m.Update(tea.MouseClickMsg{X: 4, Y: 1, Button: tea.MouseLeft})
	drainAnimation(t, m, cmd)
	_, cmd = m.Update(tea.MouseReleaseMsg{X: 4, Y: 1, Button: tea.MouseLeft})
	drainAnimation(t, m, cmd)
	if m.typesetSelect != nil {
		t.Fatal("click selected")
	}
	_, cmd = m.Update(tea.MouseClickMsg{X: 4, Y: 1, Button: tea.MouseLeft, Mod: tea.ModShift})
	drainAnimation(t, m, cmd)
	_, cmd = m.Update(tea.MouseMotionMsg{X: 40, Y: 12, Button: tea.MouseLeft, Mod: tea.ModShift})
	drainAnimation(t, m, cmd)
	if m.typesetSelect != nil {
		t.Fatal("shift-drag selected")
	}
	// A new drag replaces the selection; a plain click clears it.
	drag(t, m, image.Pt(4, 1), image.Pt(40, 12))
	if m.typesetSelected == "" {
		t.Fatal("second drag")
	}
	_, cmd = m.Update(tea.MouseClickMsg{X: 60, Y: 20, Button: tea.MouseLeft})
	drainAnimation(t, m, cmd)
	_, cmd = m.Update(tea.MouseReleaseMsg{X: 60, Y: 20, Button: tea.MouseLeft})
	drainAnimation(t, m, cmd)
	if m.typesetSelect != nil || m.typesetSelected != "" {
		t.Fatal("click did not clear the selection")
	}
	// Leaving the file drops it.
	drag(t, m, image.Pt(4, 1), image.Pt(40, 12))
	drainAnimation(t, m, m.clearGraphics())
	if m.typesetSelect != nil || m.typesetSelected != "" || m.typesetDragging {
		t.Fatal("selection survived leaving the document")
	}
}

func TestTypesetSelectionExportFrameAndHelp(t *testing.T) {
	m := selectModel(t)
	if !strings.Contains(helpScreen(), "drag + y") || !strings.Contains(helpScreen(), "Shift") {
		t.Fatal("help does not say how to select")
	}
	if m.View().MouseMode == tea.MouseModeNone {
		t.Fatal("typeset view takes no mouse")
	}
}

// A drag is followed, not waited for: a job finished for an earlier point of
// the drag shows its page, since only the selection is behind, and asks for
// the current one; it does not become the selection.
func TestTypesetSelectionDragShowsSupersededFrames(t *testing.T) {
	m := selectModel(t)
	plain := m.source
	_, cmd := m.Update(tea.MouseClickMsg{X: 4, Y: 1, Button: tea.MouseLeft})
	drainAnimation(t, m, cmd)
	_, cmd = m.Update(tea.MouseMotionMsg{X: 20, Y: 6, Button: tea.MouseLeft})
	_, job := m.Update(cmd())
	if job == nil || m.typesetActive == 0 {
		t.Fatal("first motion started no job")
	}
	_, cmd = m.Update(tea.MouseMotionMsg{X: 40, Y: 12, Button: tea.MouseLeft})
	if _, none := m.Update(cmd()); none != nil || m.typesetCancel == nil {
		t.Fatal("second motion cancelled or doubled the job in flight")
	}
	second := *m.typesetSelect
	_, after := m.Update(job())
	if m.source == plain || m.typesetSelected == "" {
		t.Fatal("superseded frame not shown")
	}
	if *m.typesetSelect != second {
		t.Fatalf("superseded result became the selection: %+v", m.typesetSelect)
	}
	if after == nil {
		t.Fatal("no request for the current selection")
	}
	first := m.typesetSelected
	drainAnimation(t, m, after)
	if !m.typesetSelect.Focus.Placed || len(m.typesetSelected) <= len(first) {
		t.Fatalf("current selection not reached: %q then %q", first, m.typesetSelected)
	}
}
