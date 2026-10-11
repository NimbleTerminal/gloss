package app

import (
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func lineModel(t *testing.T, path, text string) *Model {
	t.Helper()
	m := New(Options{Files: []string{path}, FilesFS: fstest.MapFS{path: {Data: []byte(text)}}, Render: "glyph", Page: 1})
	t.Cleanup(func() { _ = m.Close() })
	m.width, m.height = 120, 6
	drainAnimation(t, m, m.load(false))
	return m
}

func typeKeys(t *testing.T, m *Model, keys string) {
	t.Helper()
	for _, r := range keys {
		_, cmd := m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		drainAnimation(t, m, cmd)
	}
}

// # toggles the gutter, : asks for a line and Enter goes to it, and the
// status bar counts file lines once the rows are known to be file lines.
func TestLineNumberKeysAndGoTo(t *testing.T) {
	var lines []string
	for i := 1; i <= 40; i++ {
		lines = append(lines, strings.Repeat("x", i%7)+" line")
	}
	m := lineModel(t, "notes.txt", strings.Join(lines, "\n")+"\n")
	detail, hint := m.documentStatus(m.width, m.bodyHeight())
	if !strings.Contains(ansi.Strip(detail), "line 1/40") {
		t.Fatalf("status before numbers %q", ansi.Strip(detail))
	}
	if !strings.Contains(ansi.Strip(hint), ": line") || !strings.Contains(ansi.Strip(hint), "# numbers") {
		t.Fatalf("hint %q", ansi.Strip(hint))
	}
	cmd, handled := m.documentKey("#")
	drainAnimation(t, m, cmd)
	if !handled || !m.lineNumbers || m.markdown.gutter == 0 {
		t.Fatal("# did not show line numbers")
	}
	if rows := strings.Split(ansi.Strip(m.markdown.view()), "\n"); !strings.Contains(strings.Join(rows, "\n"), "1   x line") {
		t.Fatalf("gutter rows %q", rows)
	}
	// : takes a number; Enter goes there and the status says so.
	cmd, handled = m.documentKey(":")
	drainAnimation(t, m, cmd)
	if !handled || m.lineInput == nil {
		t.Fatal(": opened no prompt")
	}
	typeKeys(t, m, "25")
	_, hint = m.documentStatus(m.width, m.bodyHeight())
	if !strings.Contains(ansi.Strip(hint), ":25") {
		t.Fatalf("prompt not shown: %q", ansi.Strip(hint))
	}
	_, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	drainAnimation(t, m, cmd)
	if m.lineInput != nil || m.markdown.lines[m.markdown.offset].line != 25 {
		t.Fatalf("Enter did not go to line 25: offset %d line %d", m.markdown.offset, m.markdown.lines[m.markdown.offset].line)
	}
	detail, _ = m.documentStatus(m.width, m.bodyHeight())
	if !strings.Contains(ansi.Strip(detail), "line 25/40") {
		t.Fatalf("status after going: %q", ansi.Strip(detail))
	}
	// Esc cancels the prompt without moving; letters are not accepted.
	cmd, _ = m.documentKey(":")
	drainAnimation(t, m, cmd)
	typeKeys(t, m, "a1")
	if string(m.lineInput.draft) != "1" {
		t.Fatalf("prompt draft %q", string(m.lineInput.draft))
	}
	_, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	drainAnimation(t, m, cmd)
	if m.lineInput != nil || m.markdown.lines[m.markdown.offset].line != 25 {
		t.Fatal("Esc did not cancel in place")
	}
	// Past the end goes to the last line; # again hides the gutter.
	cmd, _ = m.documentKey(":")
	drainAnimation(t, m, cmd)
	typeKeys(t, m, "999")
	_, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	drainAnimation(t, m, cmd)
	if v := m.markdown; v.offset != max(0, len(v.lines)-v.height) {
		t.Fatalf("past the end stopped at offset %d of %d rows", v.offset, len(v.lines))
	}
	cmd, _ = m.documentKey("#")
	drainAnimation(t, m, cmd)
	if m.lineNumbers || m.markdown.gutter != 0 {
		t.Fatal("# did not hide line numbers")
	}
}

// Rendered Markdown prose has no file lines: # still arms numbers for the
// source view, and both keys say where the lines are.
func TestLineNumbersNeedLineRows(t *testing.T) {
	m := lineModel(t, "notes.md", "# Title\n\nSome *prose* here.\n\nMore.\n")
	cmd, _ := m.documentKey("#")
	drainAnimation(t, m, cmd)
	if !m.lineNumbers || m.markdown.gutter != 0 || !strings.Contains(m.note, "source") {
		t.Fatalf("# on prose: numbers %v gutter %d note %q", m.lineNumbers, m.markdown.gutter, m.note)
	}
	m.note = ""
	cmd, _ = m.documentKey(":")
	drainAnimation(t, m, cmd)
	if m.lineInput != nil || !strings.Contains(m.note, "source") {
		t.Fatalf(": on prose: prompt %v note %q", m.lineInput != nil, m.note)
	}
	// The source view has them.
	cmd, _ = m.documentKey("s")
	drainAnimation(t, m, cmd)
	if m.markdown.gutter == 0 || !m.markdown.numbered() {
		t.Fatal("source view not numbered")
	}
}

// --line-numbers starts with the gutter on.
func TestLineNumbersOption(t *testing.T) {
	m := New(Options{Files: []string{"a.txt"}, FilesFS: fstest.MapFS{"a.txt": {Data: []byte("one\ntwo\n")}}, Render: "glyph", Page: 1, LineNumbers: true})
	t.Cleanup(func() { _ = m.Close() })
	m.width, m.height = 40, 6
	drainAnimation(t, m, m.load(false))
	if !m.lineNumbers || m.markdown.gutter == 0 {
		t.Fatal("option did not turn numbers on")
	}
}

// In the typeset view # asks the loader for a gutter, : goes to a line
// through the loader, and the status bar reports the line at the top.
func TestTypesetLineNumberKeys(t *testing.T) {
	var lines []string
	for i := 1; i <= 200; i++ {
		lines = append(lines, "line "+strconv.Itoa(i))
	}
	files := fstest.MapFS{"long.txt": {Data: []byte(strings.Join(lines, "\n") + "\n")}, "page.md": {Data: []byte("# Title\n\nProse.\n")}}
	m := New(Options{Files: []string{"long.txt", "page.md"}, FilesFS: files, Render: "glyph", Typeset: true, DPI: 150, Page: 1})
	t.Cleanup(func() { _ = m.Close() })
	m.width, m.height = 160, 20
	drainAnimation(t, m, m.load(false))
	if m.typeset == nil || m.markdown != nil {
		t.Fatal("not a typeset page")
	}
	detail, hint := m.documentStatus(m.width, m.bodyHeight())
	if !strings.Contains(ansi.Strip(detail), "line 1/200") || !strings.Contains(ansi.Strip(hint), ": line") {
		t.Fatalf("status %q hint %q", ansi.Strip(detail), ansi.Strip(hint))
	}
	plain := m.source
	cmd, handled := m.documentKey("#")
	drainAnimation(t, m, cmd)
	if !handled || !m.lineNumbers || m.typesetShown == nil || !m.typesetShown.Numbers || m.source == plain {
		t.Fatal("# did not redraw the page with numbers")
	}
	cmd, _ = m.documentKey(":")
	drainAnimation(t, m, cmd)
	if m.lineInput == nil {
		t.Fatal(": opened no prompt")
	}
	typeKeys(t, m, "150")
	_, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	drainAnimation(t, m, cmd)
	if m.lineInput != nil || m.typesetShown.Line != 150 || m.typesetTop != m.typesetShown.Top || m.typesetTop == 0 {
		t.Fatalf("Enter did not go to line 150: shown %+v top %d", m.typesetShown, m.typesetTop)
	}
	detail, _ = m.documentStatus(m.width, m.bodyHeight())
	if !strings.Contains(ansi.Strip(detail), "line 150/200") {
		t.Fatalf("status after going: %q", ansi.Strip(detail))
	}
	// Scrolling keeps the gutter; the page stays where it is.
	cmd, _ = m.documentKey("down")
	drainAnimation(t, m, cmd)
	if !m.typesetShown.Numbers || m.typesetShown.Line < 150 {
		t.Fatalf("scrolled view %+v", m.typesetShown)
	}
	// A Markdown page has no lines: both keys say so; # still toggles the flag.
	drainAnimation(t, m, m.switchFile(1))
	if m.typeset == nil || m.typesetShown.Lines != 0 {
		t.Fatalf("markdown page %+v", m.typesetShown)
	}
	m.note = ""
	cmd, _ = m.documentKey(":")
	drainAnimation(t, m, cmd)
	if m.lineInput != nil || !strings.Contains(m.note, "line") {
		t.Fatalf(": on a Markdown page: prompt %v note %q", m.lineInput != nil, m.note)
	}
	m.note = ""
	cmd, _ = m.documentKey("#")
	drainAnimation(t, m, cmd)
	if m.lineNumbers || !strings.Contains(m.note, "line") {
		t.Fatalf("# on a Markdown page: numbers %v note %q", m.lineNumbers, m.note)
	}
}

func TestLineNumbersInHelp(t *testing.T) {
	if !strings.Contains(helpScreen(), "# line numbers") || !strings.Contains(helpScreen(), ": go to line") || !strings.Contains(helpScreen(), "S chop") {
		t.Fatal("help does not mention line numbers")
	}
}
