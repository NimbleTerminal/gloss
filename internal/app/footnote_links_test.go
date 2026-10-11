package app

import (
	"strings"
	"testing"
	"testing/fstest"

	tea "charm.land/bubbletea/v2"
)

func clickFootnote(t *testing.T, m *Model, address string, drag bool) {
	t.Helper()
	for y := 0; y < m.bodyHeight(); y++ {
		for x := 0; x < m.width; x++ {
			if m.linkAt(x, y) != address {
				continue
			}
			_, cmd := m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
			drainAnimation(t, m, cmd)
			if drag {
				m.Update(tea.MouseMotionMsg{X: x + 1, Y: y, Button: tea.MouseLeft})
			}
			_, cmd = m.Update(tea.MouseReleaseMsg{X: x, Y: y, Button: tea.MouseLeft})
			drainAnimation(t, m, cmd)
			return
		}
	}
	t.Fatalf("no hit area for %s (typeset=%v): %q", address, m.typeset != nil, m.View().Content)
}

func TestFootnoteClickRoundTrip(t *testing.T) {
	source := "First[^a].\n\nSecond[^a].\n\n" + strings.Repeat("Filler paragraph.\n\n", 30) + "[^a]: The note body.\n"
	for _, typeset := range []bool{false, true} {
		for _, width := range []int{45, 80} {
			m := New(Options{Files: []string{"notes.md"}, FilesFS: fstest.MapFS{"notes.md": {Data: []byte(source)}}, Typeset: typeset, Render: "glyph", Page: 1, DPI: 150})
			t.Cleanup(func() { m.Close() })
			m.width, m.height = width, 20
			drainAnimation(t, m, m.load(false))
			clickFootnote(t, m, "gloss:fn:1:0", true)
			if m.typesetTop != 0 || m.markdown != nil && m.markdown.offset != 0 {
				t.Fatal("drag jumped")
			}
			clickFootnote(t, m, "gloss:fn:1:0", false)
			if typeset && m.typesetTop == 0 || !typeset && m.markdown.offset == 0 {
				t.Fatal("reference did not jump")
			}
			_, cmd := m.Update(tea.WindowSizeMsg{Width: width + 5, Height: 20})
			drainAnimation(t, m, cmd)
			clickFootnote(t, m, "gloss:ref:1:1", false)
			clickFootnote(t, m, "gloss:fn:1:1", false)
			clickFootnote(t, m, "gloss:ref:1:0", false)
			clickFootnote(t, m, "gloss:fn:1:0", false)
		}
	}
}
