//go:build !js

package app

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"nimbleterminal.dev/gloss/internal/document"
)

func TestTableCSV(t *testing.T) {
	cells := [][]string{{"name", "note"}, {"a, b", `say "hi"`}, {"short"}}
	rows, err := csv.NewReader(strings.NewReader(string(tableCSV(cells)))).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{{"name", "note"}, {"a, b", `say "hi"`}, {"short", ""}}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("rows %q, want %q", rows, want)
	}
}

// longTable is a Markdown page with a table far enough down to scroll to.
func longTable() string {
	return strings.Repeat("A line of prose.\n\n", 40) + "|h|i|\n|-|-|\n|1|2|\n\n" + strings.Repeat("More prose.\n\n", 40)
}

func TestOpenTableInGloss(t *testing.T) {
	m, _ := besideFiles(t, longTable())
	scratch := t.TempDir()
	m.opts.Scratch = func(name string, data []byte) (string, error) {
		path := filepath.Join(scratch, name)
		return path, os.WriteFile(path, data, 0o600)
	}
	m.Update(tab)
	items := m.markdown.items()
	if m.markdown.focus == nil || m.markdown.focus.kind != itemTable || m.markdown.offset == 0 {
		t.Fatalf("focus %+v at %d of %d items", m.markdown.focus, m.markdown.offset, len(items))
	}
	offset := m.markdown.offset
	m.Update(press("a"))
	if got := entryLabels(m.actions.entries); got[0] != "Open table in gloss" {
		t.Fatalf("entries %q", got)
	}
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	drainAnimation(t, m, cmd)
	if m.sheet == nil || !m.isFetched() {
		t.Fatalf("not a fetched sheet: sheet %v err %v", m.sheet, m.err)
	}
	csvFile := m.opts.Files[m.index]
	if _, err := os.Stat(csvFile); err != nil {
		t.Fatal(err)
	}
	_, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	drainAnimation(t, m, cmd)
	if m.markdown == nil || m.markdown.offset != offset {
		t.Fatalf("back on %v at %d, want %d", m.markdown != nil, m.markdown.offset, offset)
	}
	if f := m.markdown.focus; f == nil || f.kind != itemTable {
		t.Fatalf("focus %+v after Esc", f)
	}
	if _, err := os.Stat(csvFile); !os.IsNotExist(err) {
		t.Fatalf("the CSV is still there: %v", err)
	}
}

func TestOpenTableWithListsGlossFirst(t *testing.T) {
	m, _ := besideFiles(t, longTable())
	host, _ := pictureHosts(m, t.TempDir())
	m.Update(tab)
	m.Update(press("a"))
	if got := entryLabels(m.actions.entries); got[0] != "Open table with…" {
		t.Fatalf("entries %q", got)
	}
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	drainAnimation(t, m, cmd)
	p := m.openWithPicker
	if p == nil || m.layer != layerOpenWith || len(p.choices) == 0 {
		t.Fatalf("no chooser: %+v", p)
	}
	var names []string
	for _, c := range p.choices {
		names = append(names, c.Name)
	}
	if names[0] != "gloss" || p.choices[0].ID != "gloss" {
		t.Fatalf("rows %q", names)
	}
	for _, n := range names {
		if n == "Copy path" {
			t.Fatalf("Copy path is listed: %q", names)
		}
	}
	// The cursor starts on the host's default; gloss is above it. d on
	// gloss is no default; Enter opens the sheet.
	m.Update(press("k"))
	m.Update(press("d"))
	if m.note != "This action cannot be a default" {
		t.Fatalf("note %q", m.note)
	}
	_, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	drainAnimation(t, m, cmd)
	if m.sheet == nil || !m.isFetched() || m.openWithPicker != nil || len(host.launched) != 0 {
		t.Fatalf("sheet %v fetched %v picker %v launched %d", m.sheet != nil, m.isFetched(), m.openWithPicker, len(host.launched))
	}
}

// A typeset page comes back at the same top, with the table outlined.
func TestOpenTableFromTypesetPage(t *testing.T) {
	dir := t.TempDir()
	doc := filepath.Join(dir, "doc.md")
	if err := os.WriteFile(doc, []byte(longTable()), 0o600); err != nil {
		t.Fatal(err)
	}
	m := New(Options{Files: []string{doc}, MarkdownMode: "typeset", Render: "glyph", DPI: 150, Page: 1,
		Scratch: func(name string, data []byte) (string, error) {
			path := filepath.Join(dir, name)
			return path, os.WriteFile(path, data, 0o600)
		}})
	t.Cleanup(func() { _ = m.Close() })
	m.width, m.height = 80, 30
	drainAnimation(t, m, m.load(false))
	if m.typeset == nil || m.markdown != nil {
		t.Fatalf("not a typeset page: %v", m.err)
	}
	pressDrain(t, m, tab)
	at := focused(m)
	if at < 0 || m.typesetItems[at].Kind != document.TypesetTable || m.typesetTop == 0 {
		t.Fatalf("focus on %d, top %d", at, m.typesetTop)
	}
	top := m.typesetTop
	pressDrain(t, m, press("a"))
	if got := entryLabels(m.actions.entries); got[0] != "Open table in gloss" {
		t.Fatalf("entries %q", got)
	}
	pressDrain(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.sheet == nil || !m.isFetched() {
		t.Fatalf("not a fetched sheet: %v", m.err)
	}
	pressDrain(t, m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.typeset == nil || m.typesetTop != top || focused(m) != at {
		t.Fatalf("back at top %d (want %d), focus %d (want %d)", m.typesetTop, top, focused(m), at)
	}
}
