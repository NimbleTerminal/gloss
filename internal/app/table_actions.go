package app

import (
	"bytes"
	"encoding/csv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"nimbleterminal.dev/gloss/internal/openwith"
)

// tableSaved is the answer of the command that writes a table for another
// app, or for gloss.
type tableSaved struct {
	generation uint64
	path       string
	chooser    bool // Ask which app; otherwise open it in gloss.
	err        error
}

// tableEntries lists what can be done with a table in this host. Opening it
// needs a place to write the CSV; which app, an Open with host.
func (m *Model) tableEntries(cells [][]string) []menuEntry {
	var entries []menuEntry
	switch {
	case m.opts.Scratch != nil && m.opts.OpenWith != nil:
		entries = append(entries, menuEntry{"Open table with…", func(m *Model) tea.Cmd { return m.openTable(cells, true) }})
	case m.opts.Scratch != nil:
		entries = append(entries, menuEntry{"Open table in gloss", func(m *Model) tea.Cmd { return m.openTable(cells, false) }})
	}
	return append(entries,
		menuEntry{"Copy as TSV", func(m *Model) tea.Cmd {
			m.note = "Table copied"
			return tea.SetClipboard(tableTSV(cells))
		}},
		menuEntry{"Copy as Markdown", func(m *Model) tea.Cmd {
			m.note = "Table copied"
			return tea.SetClipboard(tableMarkdown(cells))
		}})
}

// openTable writes the table as table.csv in the scratch folder, then asks
// which app opens it (chooser) or shows it in gloss as a sheet.
func (m *Model) openTable(cells [][]string, chooser bool) tea.Cmd {
	scratch := m.opts.Scratch
	if scratch == nil {
		return nil
	}
	data, generation := tableCSV(cells), m.generation
	return func() tea.Msg {
		path, err := scratch("table.csv", data)
		return tableSaved{generation: generation, path: path, chooser: chooser, err: err}
	}
}

func (m *Model) tableSaved(v tableSaved) tea.Cmd {
	if v.generation != m.generation {
		return nil
	}
	if v.err != nil {
		m.note = "open table: " + safe(v.err.Error())
		return nil
	}
	if !v.chooser {
		return m.openTableInGloss(v.path)
	}
	before := m.openWithPicker
	cmd := m.requestOpenWith(v.path, "csv", false, true, openwith.Open)
	if p := m.openWithPicker; p != before {
		p.table = true
	}
	return cmd
}

// openTableInGloss shows the table's CSV as a sheet beside the document,
// which Esc returns to as it is: where it was scrolled, with the table
// focused.
func (m *Model) openTableInGloss(path string) tea.Cmd {
	f := fetchedDoc{path: path, from: m.index, typesetTop: m.typesetTop, focus: m.focusedItem() + 1}
	if v := m.markdown; v != nil {
		f.markdown = &markdownView{raw: v.raw, offset: v.offset, left: v.left}
	}
	m.fetched = append(m.fetched, f)
	cmd := m.addDropped(dropResult{files: []string{path}})
	m.note = "opened table"
	return cmd
}

// focusedItem is the place of the focused item among the page's, -1 for none.
func (m *Model) focusedItem() int {
	v := m.markdown
	if v == nil {
		return m.typesetFocused()
	}
	if v.focus != nil {
		for i, it := range v.items() {
			if sameItem(it, *v.focus) {
				return i
			}
		}
	}
	return -1
}

// tableCSV is the table as CSV, every row as long as the longest.
func tableCSV(cells [][]string) []byte {
	cols := 0
	for _, row := range cells {
		cols = max(cols, len(row))
	}
	var b bytes.Buffer
	w := csv.NewWriter(&b)
	for _, row := range cells {
		record := make([]string, cols)
		copy(record, row)
		_ = w.Write(record) // To memory: no error to meet.
	}
	w.Flush()
	return b.Bytes()
}

// oneLine keeps a cell on one line of a copied table.
func oneLine(cell string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(cell, "\t", " ")), " ")
}

// tableTSV is the table as tab-separated rows, for spreadsheets.
func tableTSV(cells [][]string) string {
	var b strings.Builder
	for _, row := range cells {
		for i, cell := range row {
			if i > 0 {
				b.WriteByte('\t')
			}
			b.WriteString(oneLine(cell))
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// tableMarkdown is the table as a Markdown pipe table, the first row its
// header; pipes in cells are escaped.
func tableMarkdown(cells [][]string) string {
	cols := 0
	for _, row := range cells {
		cols = max(cols, len(row))
	}
	if cols == 0 {
		return ""
	}
	var b strings.Builder
	line := func(row []string) {
		b.WriteString("|")
		for i := range cols {
			cell := ""
			if i < len(row) {
				cell = strings.ReplaceAll(oneLine(row[i]), "|", `\|`)
			}
			b.WriteString(" " + cell + " |")
		}
		b.WriteByte('\n')
	}
	line(cells[0])
	b.WriteString("|" + strings.Repeat(" --- |", cols) + "\n")
	for _, row := range cells[1:] {
		line(row)
	}
	return b.String()
}
