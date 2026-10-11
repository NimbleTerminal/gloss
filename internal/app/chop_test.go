package app

import (
	"strings"
	"testing"
	"testing/fstest"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// S chops long lines and wraps them again; chopped, the arrows scroll
// sideways, the status says which column is at the left, and a search match
// beyond the edge is scrolled into view.
func TestChopKeysAndSearch(t *testing.T) {
	long := "start " + strings.Repeat("filler ", 30) + "needle at the end"
	m := lineModel(t, "notes.txt", "first line\n"+long+"\nlast line\n")
	m.width = 40
	drainAnimation(t, m, m.layoutMarkdown())
	if len(m.markdown.lines) < 6 {
		t.Fatalf("long line did not wrap: %d rows", len(m.markdown.lines))
	}
	if hint := m.hints(); !strings.Contains(hint, "S chop") {
		t.Fatalf("hint %q", hint)
	}
	cmd, handled := m.documentKey("S")
	drainAnimation(t, m, cmd)
	if !handled || !m.chopLines || !m.markdown.chop {
		t.Fatal("S did not chop")
	}
	rows := strings.Split(ansi.Strip(m.markdown.view()), "\n")
	if len(m.markdown.lines) > 6 || ansi.StringWidth(rows[2]) > 40 {
		t.Fatalf("chopped rows %q", rows)
	}
	cmd, _ = m.documentKey("right")
	drainAnimation(t, m, cmd)
	if m.markdown.left != 10 {
		t.Fatalf("right scrolled to %d", m.markdown.left)
	}
	if detail := m.detail(m.width, m.bodyHeight()); !strings.Contains(detail, "col 11") || !strings.Contains(m.hints(), "S wrap") {
		t.Fatalf("status %q hint %q", detail, m.hints())
	}
	// A match past the right edge comes into view.
	m.Update(press("/"))
	for _, r := range "needle" {
		m.Update(press(string(r)))
	}
	_, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	drainAnimation(t, m, cmd)
	if m.search == nil || len(m.search.matches) != 1 {
		t.Fatalf("search found %v", m.search)
	}
	row := m.search.matches[0].row
	shown := ansi.Strip(strings.Split(m.markdown.view(), "\n")[row-m.markdown.offset])
	if !strings.Contains(shown, "needle") {
		t.Fatalf("match not scrolled into view: left %d row %q", m.markdown.left, shown)
	}
	// Wrapping again puts the left edge back.
	cmd, _ = m.documentKey("S")
	drainAnimation(t, m, cmd)
	if m.chopLines || m.markdown.chop || m.markdown.left != 0 || len(m.markdown.lines) < 6 {
		t.Fatal("S did not wrap again")
	}
}

// --chop-long-lines starts chopped.
func TestChopOption(t *testing.T) {
	m := New(Options{Files: []string{"a.txt"}, FilesFS: fstest.MapFS{"a.txt": {Data: []byte(strings.Repeat("word ", 40) + "\n")}}, Render: "glyph", Page: 1, ChopLines: true})
	t.Cleanup(func() { _ = m.Close() })
	m.width, m.height = 40, 6
	drainAnimation(t, m, m.load(false))
	rows := 0
	for _, line := range m.markdown.lines {
		if line.line > 0 {
			rows++
		}
	}
	if !m.markdown.chop || rows != 1 {
		t.Fatalf("option did not chop: %d rows for one line", rows)
	}
}

// In the typeset view S chops a text page, which then pans sideways as far
// as its widest line; a Markdown page says chopping is not for it.
func TestTypesetChopKey(t *testing.T) {
	long := "short\n" + strings.Repeat("a long line of words that will wrap at the measure ", 8) + "\nend\n"
	files := fstest.MapFS{"long.txt": {Data: []byte(long)}, "page.md": {Data: []byte("# Title\n\nProse.\n")}}
	m := New(Options{Files: []string{"long.txt", "page.md"}, FilesFS: files, Render: "glyph", Typeset: true, DPI: 150, Page: 1})
	t.Cleanup(func() { _ = m.Close() })
	m.width, m.height = 160, 20
	drainAnimation(t, m, m.load(false))
	if m.typeset == nil || !strings.Contains(m.hints(), "S chop") {
		t.Fatalf("typeset page hint %q", m.hints())
	}
	cmd, _ := m.documentKey("right")
	drainAnimation(t, m, cmd)
	if m.typesetShown.Left != 0 {
		t.Fatalf("wrapped page panned to %d", m.typesetShown.Left)
	}
	cmd, handled := m.documentKey("S")
	drainAnimation(t, m, cmd)
	if !handled || !m.chopLines || !m.typesetShown.Chop || !strings.Contains(m.hints(), "S wrap") {
		t.Fatalf("S did not chop: %+v hint %q", m.typesetShown, m.hints())
	}
	cmd, _ = m.documentKey("right")
	drainAnimation(t, m, cmd)
	if m.typesetShown.Left == 0 {
		t.Fatal("chopped page did not pan")
	}
	cmd, _ = m.documentKey("S")
	drainAnimation(t, m, cmd)
	if m.chopLines || m.typesetShown.Chop || m.typesetShown.Left != 0 {
		t.Fatalf("S did not wrap again: %+v", m.typesetShown)
	}
	drainAnimation(t, m, m.switchFile(1))
	m.note = ""
	cmd, _ = m.documentKey("S")
	drainAnimation(t, m, cmd)
	if !strings.Contains(m.note, "chop") || (m.typesetShown != nil && m.typesetShown.Chop) {
		t.Fatalf("S on a Markdown page: note %q shown %+v", m.note, m.typesetShown)
	}
}
