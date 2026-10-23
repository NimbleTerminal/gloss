package app

import (
	"image"
	"image/draw"
	"path/filepath"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"nimbleterminal.dev/gloss/internal/document"
	"nimbleterminal.dev/nttypeset"
)

type itemKind int

const (
	itemPicture itemKind = iota
	itemLink
	itemTable
)

// pageItem is a thing on a rendered Markdown page: where it is drawn, in
// rows of the text view or rectangles of the typeset page, and what it is.
type pageItem struct {
	kind    itemKind
	picture pictureItem       // itemPicture
	url     string            // itemLink
	cells   [][]string        // itemTable: plain text, header row first
	rows    [2]int            // Text view: the first and last line of m.markdown.lines.
	cols    [2]int            // Text view: a link's cells on its first row, a picture's on its rows.
	rects   []image.Rectangle // Typeset: page layout pixels.
}

const noItemsNote = "No pictures, links or tables here · [/] files"

// linkRun is a stretch of a row drawn as one link: its address and the
// cells it covers.
type linkRun struct {
	url    string
	cols   [2]int
	figure bool // It holds a picture's placeholder: the link is the picture's.
}

// linkRuns lists the OSC 8 links drawn on a row, left to right, measuring
// cells as oscLinkAt does. Glamour draws a link's label and then its
// address, both linked, with a space between: links to the same address
// with only blank cells between them are one run.
func linkRuns(text string) []linkRun {
	var runs []linkRun
	active, open, column := "", -1, 0
	blank := false // Only blank cells since the last run ended.
	var state byte
	for len(text) > 0 {
		seq, w, n, next := ansi.DecodeSequence(text, state, nil)
		if n == 0 {
			break
		}
		state, text = next, text[n:]
		if strings.HasPrefix(seq, "\x1b]8;") {
			value := strings.TrimSuffix(strings.TrimSuffix(seq[4:], "\a"), "\x1b\\")
			if _, address, ok := strings.Cut(value, ";"); ok {
				switch {
				case address == "":
					blank = open >= 0
					open = -1
				case address == active && open >= 0:
				case blank && runs[len(runs)-1].url == address:
					open = len(runs) - 1
				default:
					open = -1
				}
				active = address
			}
			continue
		}
		if w > 0 && active != "" {
			if open < 0 {
				runs = append(runs, linkRun{url: active, cols: [2]int{column, column}})
				open = len(runs) - 1
			}
			runs[open].cols[1] = column + w
			runs[open].figure = runs[open].figure || seq == "▧"
		} else if w > 0 && strings.TrimSpace(seq) != "" {
			blank = false
		}
		column += w
	}
	return runs
}

// lastCell is the column after a row's last drawn cell.
func lastCell(text string) int { return ansi.StringWidth(strings.TrimRight(ansi.Strip(text), " ")) }

// tables lists the content's tables in order: the table of a row tagged n
// is the n-1th.
func (v *markdownView) tables() []*textTable {
	var out []*textTable
	for _, block := range v.content.blocks {
		if block.table != nil {
			out = append(out, block.table)
		}
	}
	return out
}

// items lists the pictures, links and tables on the page in reading order.
// The source view and a file shown as one code block have none.
func (v *markdownView) items() []pageItem {
	if v.raw || v.fenced || v.content == nil {
		return nil
	}
	var out []pageItem
	tables := v.tables()
	open, openURL := -1, "" // A link that reached the end of the row above, which this row may go on with.
	for i := 0; i < len(v.lines); i++ {
		line := v.lines[i]
		if line.image >= 0 {
			last := i
			for last+1 < len(v.lines) && v.lines[last+1].image == line.image {
				last++
			}
			out = append(out, v.pictureAt(line.image, i, last))
			i, open, openURL = last, -1, ""
			continue
		}
		if n := line.table; n > 0 && n <= len(tables) && (i == 0 || v.lines[i-1].table != n) {
			last := i
			for last+1 < len(v.lines) && v.lines[last+1].table == n {
				last++
			}
			var cells [][]string
			for _, row := range tables[n-1].rows {
				var texts []string
				for _, id := range row {
					texts = append(texts, strings.TrimSpace(v.content.units[id].plain))
				}
				cells = append(cells, texts)
			}
			out = append(out, pageItem{kind: itemTable, cells: cells, rows: [2]int{i, last}})
		}
		// A run that is no item (a footnote's, a picture's) is followed
		// onto the next row too, as -1.
		above, aboveURL, end := open, openURL, lastCell(line.text)
		open, openURL = -1, ""
		for k, r := range linkRuns(line.text) {
			_, note := document.FootnotePeer(r.url)
			goesOn := k == 0 && aboveURL == r.url
			at := -1
			switch {
			case goesOn && above >= 0:
				out[above].rows[1] = i
				at = above
			case goesOn || note || r.figure:
			default:
				out = append(out, pageItem{kind: itemLink, url: r.url, rows: [2]int{i, i}, cols: r.cols})
				at = len(out) - 1
			}
			if r.cols[1] >= end {
				open, openURL = at, r.url
			}
		}
	}
	return out
}

// pictureAt is the picture drawn on rows first to last, with its caption
// rows above them.
func (v *markdownView) pictureAt(index, first, last int) pageItem {
	asset := v.doc.Images[index]
	top := first
	if top > 0 && len(v.lines[top-1].spans) > 0 {
		unit := v.lines[top-1].spans[0].unit
		for top > 0 && v.lines[top-1].image < 0 && len(v.lines[top-1].spans) > 0 && v.lines[top-1].spans[0].unit == unit {
			top--
		}
	}
	name := asset.Alt
	switch {
	case asset.Path != "":
		name = filepath.Base(asset.Path)
	case name == "":
		name = "picture"
	}
	p := pictureItem{name: name, path: asset.Path, link: v.content.imageLinks[index], shown: func() image.Image {
		if a := v.animators[index]; a != nil {
			if shown := a.Status().Presented; shown != nil {
				return shown.Image()
			}
		}
		return v.doc.Images[index].Image
	}}
	inset := v.lines[first].inset
	return pageItem{kind: itemPicture, picture: p, rows: [2]int{top, last}, cols: [2]int{inset, inset + pictureColumns(asset, v.width, v.cw)}}
}

// sameItem reports whether two items are the one thing in the same place.
func sameItem(a, b pageItem) bool { return a.kind == b.kind && a.rows == b.rows && a.cols == b.cols }

// focusOrdinal says which item has the focus, by kind and ordinal among
// that kind; -1 for none.
func (v *markdownView) focusOrdinal() (itemKind, int) {
	if v.focus == nil {
		return 0, -1
	}
	n := 0
	for _, it := range v.items() {
		if it.kind != v.focus.kind {
			continue
		}
		if sameItem(it, *v.focus) {
			return it.kind, n
		}
		n++
	}
	return 0, -1
}

// refocus puts the focus back on the item of that kind and ordinal, if the
// new rows still have it.
func (v *markdownView) refocus(kind itemKind, ordinal int) {
	if ordinal < 0 {
		return
	}
	n := 0
	for _, it := range v.items() {
		if it.kind != kind {
			continue
		}
		if n == ordinal {
			v.focus = &it
			return
		}
		n++
	}
}

// showItem scrolls so the item is on screen: its end when it lies below,
// its start when above or when it is taller than the screen.
func (v *markdownView) showItem(it pageItem) {
	if it.rows[0] < v.offset {
		v.offset = it.rows[0]
	} else if it.rows[1] >= v.offset+v.height {
		v.offset = min(it.rows[0], it.rows[1]-v.height+1)
	}
	v.scroll(0)
}

// focusRanges are the byte ranges of row i's plain text the highlight
// covers: a link's cells, a picture's caption, a table's header.
func (v *markdownView) focusRanges(i int) [][2]int {
	f := v.focus
	if f == nil || i < f.rows[0] || i > f.rows[1] {
		return nil
	}
	line := v.lines[i]
	plain := ansi.Strip(line.text)
	whole := [][2]int{{0, len(strings.TrimRight(plain, " "))}}
	cell := func(col int) int { return len(ansi.Truncate(plain, col, "")) }
	switch f.kind {
	case itemLink:
		for _, r := range linkRuns(line.text) {
			if r.url == f.url && (i > f.rows[0] || r.cols == f.cols) {
				return [][2]int{{cell(r.cols[0]), cell(r.cols[1])}}
			}
		}
	case itemPicture:
		if line.image < 0 {
			return whole
		}
	case itemTable:
		tables := v.tables()
		if line.table < 1 || line.table > len(tables) || len(tables[line.table-1].rows) == 0 || len(line.spans) == 0 {
			return nil
		}
		header := map[int]bool{}
		for _, id := range tables[line.table-1].rows[0] {
			header[id] = true
		}
		for _, span := range line.spans {
			if !header[span.unit] {
				return nil
			}
		}
		return whole
	}
	return nil
}

// itemAt is the item drawn at cell x, y of the view: a link's cells, a
// picture or its caption, or anywhere on a table's rows.
func (v *markdownView) itemAt(x, y int) (pageItem, bool) {
	row := v.offset + y
	if x < 0 || y < 0 || y >= v.height || row >= len(v.lines) {
		return pageItem{}, false
	}
	line := v.lines[row]
	col := x - v.gutter + v.left
	var table *pageItem
	for _, it := range v.items() {
		if row < it.rows[0] || row > it.rows[1] {
			continue
		}
		switch it.kind {
		case itemTable:
			table = &it
		case itemPicture:
			if (line.image >= 0 && x >= it.cols[0] && x < it.cols[1]) || (line.image < 0 && col < lastCell(line.text)) {
				return it, true
			}
		case itemLink:
			for _, r := range linkRuns(line.text) {
				if r.url == it.url && col >= r.cols[0] && col < r.cols[1] {
					return it, true
				}
			}
		}
	}
	if table != nil {
		return *table, true // A link in a table goes first.
	}
	return pageItem{}, false
}

// tabItems reports whether Tab moves between the page's items rather than
// files: rendered Markdown, in the text view or typeset, on the document
// screen. Typeset text, source and JSON keep Tab for files.
func (m *Model) tabItems() bool {
	if m.screen != screenDocument || m.isPreview {
		return false
	}
	if v := m.markdown; v != nil {
		return !v.raw && !v.fenced
	}
	return m.typeset != nil && m.kind == "markdown"
}

// moveFocus moves the highlight step items on, wrapping, and scrolls the
// item into view. With no focus yet, Tab starts at the first item not
// above the screen and Shift-Tab at the last not below it.
func (m *Model) moveFocus(step int) tea.Cmd {
	v := m.markdown
	if v == nil {
		if m.typeset != nil {
			return m.moveTypesetFocus(step)
		}
		return nil
	}
	items := v.items()
	if len(items) == 0 {
		v.focus = nil
		m.note = noItemsNote
		return nil
	}
	at := -1
	if v.focus != nil {
		for i, it := range items {
			if sameItem(it, *v.focus) {
				at = i
				break
			}
		}
	}
	switch {
	case at >= 0:
		at = (at + step%len(items) + len(items)) % len(items)
	case step > 0:
		at = 0
		for i, it := range items {
			if it.rows[1] >= v.offset {
				at = i
				break
			}
		}
	default:
		at = len(items) - 1
		for i := len(items) - 1; i >= 0; i-- {
			if items[i].rows[0] < v.offset+v.height {
				at = i
				break
			}
		}
	}
	it := items[at]
	v.focus = &it
	v.showItem(it)
	return nil
}

// markdownItemActions opens the menu of the item under the pointer, or of
// the focused one from the keyboard, focusing the first when none is.
func (m *Model) markdownItemActions(at image.Point, pointer bool) tea.Cmd {
	v := m.markdown
	if v == nil {
		return m.typesetItemActions(at, pointer)
	}
	if pointer {
		it, ok := v.itemAt(at.X, at.Y)
		if !ok {
			return nil
		}
		v.focus = &it
	} else {
		if v.focus == nil {
			m.moveFocus(1)
		}
		if v.focus == nil {
			return nil
		}
		at = image.Pt(max(0, v.focus.cols[0]-v.left), v.focus.rows[0]-v.offset)
	}
	return m.openActions(itemTitle(*v.focus), m.itemEntries(*v.focus), at)
}

// itemTitle heads an item's menu.
func itemTitle(it pageItem) string {
	switch it.kind {
	case itemPicture:
		return it.picture.name
	case itemLink:
		return it.url
	}
	return "Table"
}

// rectsEqual says two lists of rectangles are the same, as a focus.
func rectsEqual(a, b []image.Rectangle) bool {
	return slices.Equal(a, b)
}

func (m *Model) clearTypesetFocus() {
	m.typesetFocus, m.typesetFocusIndex, m.typesetTabPending = nil, 0, 0
}

// typesetFocused is the index of the focused item on the typeset page; -1
// for none.
func (m *Model) typesetFocused() int {
	if m.typesetFocus == nil {
		return -1
	}
	for i, it := range m.typesetItems {
		if rectsEqual(it.Rects, m.typesetFocus) {
			return i
		}
	}
	return -1
}

// moveTypesetFocus is moveFocus on the typeset page. Its items are those
// laid so far: when the next one is further down than that, the page goes a
// screen down and the step is taken again on the view that comes back.
func (m *Model) moveTypesetFocus(step int) tea.Cmd {
	shown := m.typesetShown
	if shown == nil {
		return nil
	}
	items := m.typesetItems
	top, bottom := shown.Top, shown.Top+shown.Height
	at := m.typesetFocused()
	switch {
	case at >= 0:
		at += step
	case step > 0:
		at = len(items)
		for i, it := range items {
			if it.Rects[len(it.Rects)-1].Max.Y > top {
				at = i
				break
			}
		}
	default:
		at = len(items) - 1
		for i := len(items) - 1; i >= 0; i-- {
			if items[i].Rects[0].Min.Y < bottom {
				at = i
				break
			}
		}
	}
	if (len(items) == 0 || at >= len(items)) && !m.typesetDone {
		m.typesetTabPending = step
		m.typesetGoTop = -1
		m.typesetTop += max(1, m.typesetViewport().Height>>m.zoom)
		return m.queueTypesetReflow()
	}
	if len(items) == 0 {
		m.clearTypesetFocus()
		m.note = noItemsNote
		return m.queueTypesetReflow()
	}
	at = (at%len(items) + len(items)) % len(items)
	m.typesetFocus, m.typesetFocusIndex = items[at].Rects, at
	if r := items[at].Rects[0]; r.Min.Y < top || r.Max.Y > bottom {
		m.typesetGoTop = max(0, r.Min.Y-20)
	}
	return m.queueTypesetReflow()
}

// followTypesetFocus takes a Tab that waited for more of the page, and keeps
// the focus on its item when the page is laid out anew, as at another width.
func (m *Model) followTypesetFocus() tea.Cmd {
	if step := m.typesetTabPending; step != 0 {
		m.typesetTabPending = 0
		return m.moveFocus(step)
	}
	if m.typesetFocus == nil {
		return nil
	}
	i := m.typesetFocusIndex
	switch {
	case i < len(m.typesetItems) && rectsEqual(m.typesetItems[i].Rects, m.typesetFocus):
		return nil
	case i < len(m.typesetItems):
		m.typesetFocus = m.typesetItems[i].Rects
	default:
		m.clearTypesetFocus()
	}
	return m.queueTypesetReflow()
}

// typesetRaster maps page rectangles into the shown page picture's pixels.
func (m *Model) typesetRaster(r image.Rectangle) image.Rectangle {
	shown, b := m.typesetShown, m.source.Bounds()
	view := nttypeset.View{Top: shown.Top, Left: shown.Left, Scale: float64(b.Dx()) / float64(shown.Width)}
	return view.Project(r).Add(b.Min)
}

// typesetItemIndex is the index of the item under body cell x, y of the
// typeset page, -1 for none; a link in a table goes before the table.
func (m *Model) typesetItemIndex(x, y int) int {
	if m.typeset == nil || m.source == nil || m.typesetShown == nil || m.typesetShown.Width <= 0 || !m.typesetViewMatches() {
		return -1
	}
	at, ok := m.typesetPixel(x, y, false)
	if !ok {
		return -1
	}
	table := -1
	for i, it := range m.typesetItems {
		for _, r := range it.Rects {
			if !at.In(m.typesetRaster(r)) {
				continue
			}
			if it.Kind != document.TypesetTable {
				return i
			}
			table = i
		}
	}
	return table
}

// typesetItemAt is the item under body cell x, y of the typeset page.
func (m *Model) typesetItemAt(x, y int) (pageItem, bool) {
	if i := m.typesetItemIndex(x, y); i >= 0 {
		return m.typesetPageItem(m.typesetItems[i]), true
	}
	return pageItem{}, false
}

// typesetPageItem is a typeset item as the menus take it.
func (m *Model) typesetPageItem(it document.TypesetItem) pageItem {
	switch it.Kind {
	case document.TypesetPicture:
		name := "picture"
		if it.Path != "" {
			name = filepath.Base(it.Path)
		}
		p := pictureItem{name: name, path: it.Path, link: it.URL, shown: func() image.Image { return m.typesetPictureShown(it) }}
		return pageItem{kind: itemPicture, picture: p, rects: it.Rects}
	case document.TypesetLinkItem:
		return pageItem{kind: itemLink, url: it.URL, rects: it.Rects}
	}
	return pageItem{kind: itemTable, cells: it.Cells, rects: it.Rects}
}

// typesetPictureShown is a picture on the typeset page as it is on screen,
// cut from the page picture, playing frame and all; its still when it is
// not all on screen.
func (m *Model) typesetPictureShown(it document.TypesetItem) image.Image {
	frame := m.typesetExportFrame()
	if frame == nil || m.source == nil || m.typesetShown == nil || m.typesetShown.Width <= 0 || frame.Bounds() != m.source.Bounds() {
		return it.Image
	}
	r := m.typesetRaster(it.Rects[0])
	if r.Empty() || !r.In(frame.Bounds()) {
		return it.Image
	}
	out := image.NewRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	draw.Draw(out, out.Bounds(), frame, r.Min, draw.Src)
	return out
}

// typesetItemActions is markdownItemActions on the typeset page. The item
// the menu is for takes the focus.
func (m *Model) typesetItemActions(at image.Point, pointer bool) tea.Cmd {
	var i int
	if pointer {
		if i = m.typesetItemIndex(at.X, at.Y); i < 0 {
			return nil
		}
	} else {
		if m.source == nil || m.typesetShown == nil || m.typesetShown.Width <= 0 {
			return nil
		}
		if i = m.typesetFocused(); i < 0 {
			// The first item on screen, if there is one: a menu from the
			// keyboard does not scroll.
			for k, item := range m.typesetItems {
				if m.typesetRaster(item.Rects[0]).Overlaps(m.source.Bounds()) {
					i = k
					break
				}
			}
		}
		if i < 0 {
			return nil
		}
		at = m.typesetCellAt(m.typesetRaster(m.typesetItems[i].Rects[0]).Min)
	}
	it := m.typesetPageItem(m.typesetItems[i])
	m.typesetFocus, m.typesetFocusIndex = it.rects, i
	return tea.Batch(m.queueTypesetReflow(), m.openActions(itemTitle(it), m.itemEntries(it), at))
}

// itemEntries lists what can be done with an item in this host.
func (m *Model) itemEntries(it pageItem) []menuEntry {
	switch it.kind {
	case itemPicture:
		return m.pictureEntries(it.picture)
	case itemLink:
		return linkEntries(m, it.url, "Open")
	case itemTable:
		cells := it.cells
		return []menuEntry{
			{"Copy as TSV", func(m *Model) tea.Cmd {
				m.note = "Table copied"
				return tea.SetClipboard(tableTSV(cells))
			}},
			{"Copy as Markdown", func(m *Model) tea.Cmd {
				m.note = "Table copied"
				return tea.SetClipboard(tableMarkdown(cells))
			}},
		}
	}
	return nil
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
