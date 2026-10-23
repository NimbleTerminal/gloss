package app

import (
	"github.com/charmbracelet/x/ansi"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

var (
	tab      = tea.KeyPressMsg{Code: tea.KeyTab}
	shiftTab = tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
)

// renderedDocument is a Markdown document laid out in the text view.
func renderedDocument(t *testing.T, text string) *Model {
	t.Helper()
	m := searchDocument(t, text)
	m.markdown.raw = false
	m.layoutMarkdown()
	return m
}

// besideFiles writes doc.md and a small x.png into a folder and loads the
// document as the viewer would.
func besideFiles(t *testing.T, source string) (*Model, string) {
	t.Helper()
	dir := t.TempDir()
	f, err := os.Create(filepath.Join(dir, "x.png"))
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, image.NewRGBA(image.Rect(0, 0, 8, 4))); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if err := os.WriteFile(filepath.Join(dir, "doc.md"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	m := New(Options{Files: []string{filepath.Join(dir, "doc.md")}, Render: "glyph", Page: 1})
	t.Cleanup(func() { _ = m.Close() })
	m.Update(tea.WindowSizeMsg{Width: 60, Height: 30})
	_, cmd := m.Update(m.loader.Load(m.request(false)))
	drainAnimation(t, m, cmd)
	if m.markdown == nil {
		t.Fatalf("not shown as Markdown: %v", m.err)
	}
	return m, dir
}

func kinds(items []pageItem) []itemKind {
	var out []itemKind
	for _, it := range items {
		out = append(out, it.kind)
	}
	return out
}

func entryLabels(entries []menuEntry) []string {
	var out []string
	for _, e := range entries {
		out = append(out, e.label)
	}
	return out
}

func TestItemsInReadingOrder(t *testing.T) {
	m, dir := besideFiles(t, "![a](x.png)\n\n[l](https://e.org/)\n\n|h|i|\n|-|-|\n|1|2|\n\n[^1]\n\n[^1]: note")
	items := m.markdown.items()
	if got := kinds(items); len(got) != 3 || got[0] != itemPicture || got[1] != itemLink || got[2] != itemTable {
		t.Fatalf("kinds %v: %+v", got, items)
	}
	if want := filepath.Join(dir, "x.png"); items[0].picture.path != want {
		t.Fatalf("picture path %q, want %q", items[0].picture.path, want)
	}
	if items[1].url != "https://e.org/" {
		t.Fatalf("link %q", items[1].url)
	}
	if c := items[2].cells; len(c) != 2 || strings.Join(c[0], ",") != "h,i" || strings.Join(c[1], ",") != "1,2" {
		t.Fatalf("cells %q", c)
	}
	for _, it := range items {
		if it.rows[0] > it.rows[1] || it.rows[1] >= len(m.markdown.lines) {
			t.Fatalf("rows %v of %d", it.rows, len(m.markdown.lines))
		}
	}
}

func TestItemsMergeWrappedLinkAndLinkedPicture(t *testing.T) {
	m := renderedDocument(t, "[a link label that wraps](https://e.org/)")
	m.width = 20
	m.layoutMarkdown()
	items := m.markdown.items()
	if len(items) != 1 || items[0].kind != itemLink || items[0].rows[1]-items[0].rows[0] != 1 {
		t.Fatalf("wrapped link: %+v", items)
	}

	m, _ = besideFiles(t, "[![a](x.png)](https://e.org/)")
	items = m.markdown.items()
	if len(items) != 1 || items[0].kind != itemPicture || items[0].picture.link != "https://e.org/" {
		t.Fatalf("linked picture: %+v", items)
	}
	m.opts.OpenLink = func(string) error { return nil }
	got := entryLabels(m.itemEntries(items[0]))
	if len(got) < 2 || got[len(got)-2] != "Open link" || got[len(got)-1] != "Copy address" {
		t.Fatalf("entries %q", got)
	}
}

func TestTabMovesFocusNotFiles(t *testing.T) {
	m := renderedDocument(t, "[one](https://e.org/1)\n\n[two](https://e.org/2)")
	m.opts.Files = []string{"text.md", "other.md"}
	m.Update(tab)
	if m.index != 0 || m.markdown.focus == nil || m.markdown.focus.url != "https://e.org/1" {
		t.Fatalf("index %d focus %+v", m.index, m.markdown.focus)
	}
	m.Update(shiftTab)
	if m.index != 0 || m.markdown.focus == nil || m.markdown.focus.url != "https://e.org/2" {
		t.Fatalf("Shift-Tab did not wrap: index %d focus %+v", m.index, m.markdown.focus)
	}
	m.Update(press("]"))
	if m.index != 1 {
		t.Fatal("] did not change the file")
	}
}

func TestTabWithNoItems(t *testing.T) {
	m := renderedDocument(t, "Just a paragraph.")
	m.opts.Files = []string{"text.md", "other.md"}
	m.Update(tab)
	if m.index != 0 {
		t.Fatal("Tab changed the file")
	}
	if m.note != "No pictures, links or tables here · [/] files" {
		t.Fatalf("note %q", m.note)
	}
}

func TestTabScrollsIntoView(t *testing.T) {
	m := renderedDocument(t, strings.Repeat("paragraph\n\n", 100)+"[l](https://e.org/)")
	m.Update(tab)
	f, v := m.markdown.focus, m.markdown
	if f == nil {
		t.Fatal("no focus")
	}
	if f.rows[0] < v.offset || f.rows[0] >= v.offset+v.height {
		t.Fatalf("item row %d not in [%d,%d)", f.rows[0], v.offset, v.offset+v.height)
	}
}

func TestFocusDroppedOnReflow(t *testing.T) {
	m := renderedDocument(t, strings.Repeat("words to wrap ", 20)+"\n\n[l](https://e.org/)")
	m.Update(tab)
	if m.markdown.focus == nil {
		t.Fatal("no focus")
	}
	m.Update(tea.WindowSizeMsg{Width: 25, Height: 8})
	f := m.markdown.focus
	if f != nil {
		if f.url != "https://e.org/" || f.rows[1] >= len(m.markdown.lines) {
			t.Fatalf("stale focus %+v of %d rows", f, len(m.markdown.lines))
		}
		found := false
		for _, it := range m.markdown.items() {
			found = found || it.rows == f.rows
		}
		if !found {
			t.Fatalf("focus %+v is not an item after the reflow", f)
		}
	}
	m.Update(press("s"))
	if m.markdown.focus != nil {
		t.Fatal("focus kept in the source view")
	}
}

func TestRightClickOnLinkOpensMenu(t *testing.T) {
	m := renderedDocument(t, "[l](https://e.org/) and text\n\nplain")
	items := m.markdown.items()
	if len(items) != 1 {
		t.Fatalf("items %+v", items)
	}
	x, y := items[0].cols[0], items[0].rows[0]-m.markdown.offset
	for _, open := range []bool{true, false} {
		m.opts.OpenLink = nil
		want := "Copy address"
		if open {
			m.opts.OpenLink = func(string) error { return nil }
			want = "Open,Copy address"
		}
		m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseRight})
		if m.layer != layerActions || strings.Join(entryLabels(m.actions.entries), ",") != want {
			t.Fatalf("OpenLink %v: layer %v menu %+v", open, m.layer, m.actions)
		}
		m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	}
	blank := -1
	for row, line := range m.markdown.lines {
		if strings.Contains(line.text, "plain") {
			blank = row - m.markdown.offset
		}
	}
	if blank < 0 {
		t.Fatal("no plain row")
	}
	m.note = ""
	m.Update(tea.MouseClickMsg{X: 0, Y: blank, Button: tea.MouseRight})
	if m.layer == layerActions || m.note != "" {
		t.Fatalf("right-click on text: layer %v note %q", m.layer, m.note)
	}
}

func TestTableCopies(t *testing.T) {
	if got := tableTSV([][]string{{"h", "i"}, {"1", "2"}}); got != "h\ti\n1\t2\n" {
		t.Fatalf("TSV %q", got)
	}
	if got := tableMarkdown([][]string{{"a|b"}}); !strings.Contains(got, `a\|b`) {
		t.Fatalf("Markdown %q", got)
	}
	// A backslash before a pipe would otherwise escape the escape.
	if got := tableMarkdown([][]string{{`a\|b`}}); !strings.Contains(got, `a\\\|b`) {
		t.Fatalf("Markdown %q", got)
	}
}

func TestRawViewHasNoItems(t *testing.T) {
	m := renderedDocument(t, "[l](https://e.org/)")
	m.opts.Files = []string{"text.md", "other.md"}
	m.Update(press("s"))
	if !m.markdown.raw || m.markdown.items() != nil {
		t.Fatal("source view has items")
	}
	m.Update(tab)
	if m.index != 1 {
		t.Fatal("Tab did not switch files in the source view")
	}
}

// The highlight is drawn in reverse video on the focused link's cells.
func TestFocusHighlight(t *testing.T) {
	m := renderedDocument(t, "[l](https://e.org/)")
	if strings.Contains(m.markdown.view(), "\x1b[7m") {
		t.Fatal("highlight before focus")
	}
	m.Update(tab)
	if !strings.Contains(m.markdown.view(), "\x1b[7m") {
		t.Fatal("no highlight on the focused link")
	}
}

// A right-click on a picture's rows, and a on a table, open their menus;
// the table's header row is highlighted.
func TestPictureAndTableMenus(t *testing.T) {
	m, _ := besideFiles(t, "![a](x.png)\n\n|h|i|\n|-|-|\n|1|2|")
	items := m.markdown.items()
	if len(items) != 2 {
		t.Fatalf("items %+v", items)
	}
	pic := items[0]
	y := pic.rows[1] - m.markdown.offset
	m.Update(tea.MouseClickMsg{X: pic.cols[0], Y: y, Button: tea.MouseRight})
	if m.layer != layerActions || m.actions.title != "x.png" || !strings.Contains(strings.Join(entryLabels(m.actions.entries), ","), "Copy path") {
		t.Fatalf("picture menu %+v", m.actions)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m.Update(tab)
	m.Update(press("a"))
	if m.layer != layerActions || strings.Join(entryLabels(m.actions.entries), ",") != "Copy as TSV,Copy as Markdown" {
		t.Fatalf("table menu %+v", m.actions)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	header := items[1].rows[0]
	if len(m.markdown.focusRanges(header)) == 0 || len(m.markdown.focusRanges(items[1].rows[1])) != 0 {
		t.Fatal("the highlight is not on the header row alone")
	}
}

// A link to an address the row above also links to is its own item when
// text comes before it; two links to one address on a row are two items,
// each hit where it is drawn.
func TestSameAddressLinksStayApart(t *testing.T) {
	m := renderedDocument(t, "[first](https://e.org/)\nthen [again](https://e.org/) and [more](https://e.org/)")
	var links []pageItem
	for _, it := range m.markdown.items() {
		if it.kind == itemLink {
			links = append(links, it)
		}
	}
	if len(links) < 3 {
		t.Fatalf("links merged: %+v", links)
	}
	last := links[len(links)-1]
	got, ok := m.markdown.itemAt(last.cols[0]-m.markdown.left+m.markdown.gutter, last.rows[0]-m.markdown.offset)
	if !ok || !sameItem(got, last) {
		t.Fatalf("hit %+v, want %+v", got, last)
	}
}

// A tall picture held to fewer rows is narrower than its columns, and a
// right-click beside it is on nothing.
func TestTallPictureHitsOnlyWhereDrawn(t *testing.T) {
	m, dir := besideFiles(t, "![tall](tall.png)\n")
	f, err := os.Create(filepath.Join(dir, "tall.png"))
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, image.NewRGBA(image.Rect(0, 0, 40, 400))); err != nil {
		t.Fatal(err)
	}
	f.Close()
	_, cmd := m.Update(m.loader.Load(m.request(true)))
	drainAnimation(t, m, cmd)
	var pic pageItem
	for _, it := range m.markdown.items() {
		if it.kind == itemPicture {
			pic = it
		}
	}
	if pic.kind != itemPicture || pic.cols[1]-pic.cols[0] >= m.width/2 {
		t.Fatalf("picture %+v at width %d", pic, m.width)
	}
	row := pic.rows[1] - m.markdown.offset
	if _, ok := m.markdown.itemAt(0, row); ok && pic.cols[0] > 0 {
		t.Fatal("the margin beside the picture is part of it")
	}
	if got, ok := m.markdown.itemAt(pic.cols[0], row); !ok || got.kind != itemPicture {
		t.Fatal("the picture itself is not hit")
	}
}

func TestBareURLAt(t *testing.T) {
	for _, c := range []struct {
		text string
		at   int
		want string
	}{
		{"see https://e.org/x.", 6, "https://e.org/x"},
		{"see https://e.org/x.", 19, ""}, // The full stop.
		{"(https://e.org/a_(b))", 3, "https://e.org/a_(b)"},
		{"(https://e.org/)", 3, "https://e.org/"},
		{"ftp://e.org/", 3, ""},
		{"no address here", 3, ""},
	} {
		if got := bareURLAt(c.text, c.at); got != c.want {
			t.Errorf("%q at %d: %q, want %q", c.text, c.at, got, c.want)
		}
	}
}

// cellOf finds where a string is drawn in the view, as x, y.
func cellOf(t *testing.T, m *Model, s string) (int, int) {
	t.Helper()
	for y, line := range rows(m) {
		if i := strings.Index(line, s); i >= 0 {
			return ansi.StringWidth(line[:i]), y
		}
	}
	t.Fatalf("%q is not on screen:\n%s", s, strings.Join(rows(m), "\n"))
	return 0, 0
}

// A web address written out in text, not a link, takes a right-click too:
// whole, though it wraps, in the source view as in rendered Markdown.
func TestRightClickBareAddress(t *testing.T) {
	const address = "https://example.org/a/long/path/that/wraps/onto/the/next/row"
	m := searchDocument(t, "Read "+address+" today.")
	m.Update(tea.WindowSizeMsg{Width: 40, Height: 20})
	x, y := cellOf(t, m, "onto")
	send(m, tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseRight})
	if m.layer != layerActions || m.actions.title != address {
		t.Fatalf("source view: layer %v, menu %+v", m.layer, m.actions)
	}
	if got := strings.Join(entryLabels(m.actions.entries), ", "); got != "Copy address" {
		t.Fatalf("entries without a browser: %s", got)
	}

	m = renderedDocument(t, "Read "+address+" today.")
	m.opts.OpenLink = func(string) error { return nil }
	x, y = cellOf(t, m, "example.org")
	send(m, tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseRight})
	if m.layer != layerActions || m.actions.title != address {
		t.Fatalf("rendered: layer %v, menu %+v", m.layer, m.actions)
	}
	if got := strings.Join(entryLabels(m.actions.entries), ", "); got != "Open, Copy address" {
		t.Fatalf("entries with a browser: %s", got)
	}

	m = renderedDocument(t, "Read "+address+" today.")
	x, y = cellOf(t, m, "Read")
	send(m, tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseRight})
	if m.layer != layerNone || m.note != "" {
		t.Fatalf("a right-click on words: layer %v, note %q", m.layer, m.note)
	}
}

// With --fetch, a web link can be opened in gloss or saved; saving keeps
// the download in the working folder, numbered beside a file of that name,
// and leaves no download behind.
func TestSaveLinkedFile(t *testing.T) {
	const address = "https://e.org/notes.txt"
	m := renderedDocument(t, "[notes]("+address+")")
	if got := strings.Join(entryLabels(linkEntries(m, address, "Open")), ", "); got != "Copy address" {
		t.Fatalf("without --fetch: %s", got)
	}
	downloads, kept := t.TempDir(), t.TempDir()
	var fetched string
	m.opts.Fetch = func(a string) (string, error) {
		fetched = filepath.Join(downloads, "notes.txt")
		return fetched, os.WriteFile(fetched, []byte("hello"), 0600)
	}
	if err := os.WriteFile(filepath.Join(kept, "notes.txt"), []byte("mine"), 0600); err != nil {
		t.Fatal(err)
	}
	m.opts.Save = func(name string, data []byte) (string, error) { return saveFile(kept, name, data) }
	entries := linkEntries(m, address, "Open")
	if got := strings.Join(entryLabels(entries), ", "); got != "Open in gloss, Save linked file, Copy address" {
		t.Fatalf("with --fetch: %s", got)
	}
	if got := entryLabels(linkEntries(m, "mailto:a@e.org", "Open")); len(got) != 1 {
		t.Fatalf("a mail address is fetched: %v", got)
	}
	drainAnimation(t, m, entries[1].do(m))
	if !strings.HasPrefix(m.note, "saved notes") || m.note == "saved notes.txt" {
		t.Fatalf("note %q", m.note)
	}
	mine, err := os.ReadFile(filepath.Join(kept, "notes.txt"))
	if err != nil || string(mine) != "mine" {
		t.Fatalf("the file there was overwritten: %q %v", mine, err)
	}
	saved, err := os.ReadFile(filepath.Join(kept, strings.TrimPrefix(m.note, "saved ")))
	if err != nil || string(saved) != "hello" {
		t.Fatalf("saved %q, %v", saved, err)
	}
	if _, err := os.Stat(fetched); !os.IsNotExist(err) {
		t.Fatal("the download was left behind")
	}
	if m.fetching {
		t.Fatal("still fetching")
	}
}
