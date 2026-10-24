package app

import (
	"fmt"
	"image"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"nimbleterminal.dev/gloss/internal/document"
)

// addressSheet has an address in B1 and plain cells around it.
func addressSheet(t *testing.T) *Model {
	t.Helper()
	sheet := &document.Sheet{Name: "Data", Columns: 3, Rows: [][]string{
		{"one", "https://e.org/", "three\tx"},
		{"four", "five", "six"},
	}}
	m := tabulated(t, sheet, 1, 1)
	m.sheet.at = [2]int{0, 1}
	return m
}

func TestCellEntries(t *testing.T) {
	m := addressSheet(t)
	m.opts.Fetch = func(string) (string, error) { return "", nil }
	m.opts.OpenLink = func(string) error { return nil }
	if got := strings.Join(labels(m.cellEntries()), ", "); got != "Copy value, Copy row, Chart these columns, Open in gloss, Save linked file, Open in browser, Show QR" {
		t.Fatalf("all hosts: %s", got)
	}

	m.opts.Fetch, m.opts.OpenLink = nil, nil
	if got := strings.Join(labels(m.cellEntries()), ", "); got != "Copy value, Copy row, Chart these columns, Show QR" {
		t.Fatalf("no hosts: %s", got)
	}

	m.sheet.at = [2]int{1, 1}
	if got := strings.Join(labels(m.cellEntries()), ", "); got != "Copy value, Copy row, Chart these columns" {
		t.Fatalf("a plain cell: %s", got)
	}
}

func TestCellEntriesCopyWhatIsShown(t *testing.T) {
	m := addressSheet(t)
	m.sheet.hide(0)
	m.sheet.at = [2]int{0, 1} // Now C1, "three\tx".
	// run gives the entry's note and what it puts on the clipboard.
	run := func(label string) (string, string) {
		for _, e := range m.cellEntries() {
			if e.label == label {
				m.note = ""
				cmd := e.do(m)
				if cmd == nil {
					t.Fatalf("%q copies nothing", label)
				}
				return m.note, fmt.Sprint(cmd())
			}
		}
		t.Fatalf("no %q entry", label)
		return "", ""
	}
	if note, copied := run("Copy value"); note != "Value copied" || copied != "three\tx" {
		t.Fatalf("note %q, copied %q", note, copied)
	}
	if note, copied := run("Copy row"); note != "Row copied" || copied != "https://e.org/\tthree x" {
		t.Fatalf("note %q, copied %q", note, copied)
	}
	if got := m.sheet.rowText(); got != "https://e.org/\tthree x" {
		t.Fatalf("a row is the shown columns on one line: %q", got)
	}
}

func TestRightClickSelectsCell(t *testing.T) {
	m := addressSheet(t)
	s := m.sheet
	// Column places start after the row numbers; each is its width and a space.
	x := s.gutter() + 1
	for _, c := range s.cols[:2] {
		x += s.widths[c] + 1
	}
	send(m, tea.MouseClickMsg{X: x, Y: 2, Button: tea.MouseRight}) // The third column, second row.
	if s.at != [2]int{1, 2} || m.layer != layerActions || m.actions.title != "C2 · six" {
		t.Fatalf("at=%v layer=%v actions=%+v", s.at, m.layer, m.actions)
	}
	if m.actions.at.X != x || m.actions.at.Y != 2 {
		t.Fatalf("the menu is not at the pointer: %v", m.actions.at)
	}
	m.closeLayer()

	// Past the last column, on the header and under the rows: nothing, and
	// the cursor stays.
	before := s.at
	for _, at := range [][2]int{{m.width - 1, 2}, {x, 0}, {x, 5}} {
		send(m, tea.MouseClickMsg{X: at[0], Y: at[1], Button: tea.MouseRight})
		if m.layer != layerNone || s.at != before || m.note != "" {
			t.Fatalf("a right-click at %v: layer=%v at=%v note=%q", at, m.layer, s.at, m.note)
		}
	}

	// A left click only selects.
	send(m, tea.MouseClickMsg{X: s.gutter() + 2, Y: 1, Button: tea.MouseLeft})
	if m.layer != layerNone || s.at != [2]int{0, 0} {
		t.Fatalf("left click: layer=%v at=%v", m.layer, s.at)
	}
}

func TestSheetKeysOpenTheMenu(t *testing.T) {
	m := addressSheet(t)
	send(m, press("a"))
	s := m.sheet
	// Beside the cursor's cell, B1: past the row numbers and column A.
	if want := image.Pt(s.gutter()+1+s.widths[s.cols[0]], 1); m.layer != layerActions || m.actions.title != "B1 · https://e.org/" || m.actions.at != want {
		t.Fatalf("a: layer=%v actions=%+v, want at %v", m.layer, m.actions, want)
	}
	m.closeLayer()
	send(m, tea.KeyPressMsg{Code: tea.KeyF10, Mod: tea.ModShift})
	if m.layer != layerActions {
		t.Fatalf("Shift-F10: layer=%v", m.layer)
	}
	m.closeLayer()
	if !strings.Contains(m.hints(), "a actions") {
		t.Fatalf("hints: %s", m.hints())
	}
}

func TestSheetTabStillSwitchesFiles(t *testing.T) {
	m := viewing(t, Options{Files: []string{"a.xlsx", "b.xlsx"}}, "")
	m.Update(document.Result{Generation: m.generation, Kind: "xlsx", Page: 1, Pages: 1, Sheet: table(3, 3)})
	send(m, tea.KeyPressMsg{Code: tea.KeyTab})
	if m.index != 1 || m.layer != layerNone {
		t.Fatalf("Tab: index=%d layer=%v", m.index, m.layer)
	}
}

// A cell's value heads its menu on one line, safe, and cut short.
func TestCellMenuTitle(t *testing.T) {
	sheet := &document.Sheet{Name: "Data", Columns: 2, Rows: [][]string{
		{"two\nlines\x1b]2;x\a", strings.Repeat("long ", 20)},
	}}
	m := tabulated(t, sheet, 1, 1)
	m.sheet.at = [2]int{0, 0}
	send(m, press("a"))
	if title := m.actions.title; strings.ContainsAny(title, "\n\x1b\a") || !strings.HasPrefix(title, "A1 · two lines") {
		t.Fatalf("title %q", title)
	}
	m.closeLayer()
	m.sheet.at = [2]int{0, 1}
	send(m, press("a"))
	if title := m.actions.title; !strings.HasSuffix(title, "…") || ansi.StringWidth(title) > actionsMaxW-4 {
		t.Fatalf("title %q", title)
	}
}
