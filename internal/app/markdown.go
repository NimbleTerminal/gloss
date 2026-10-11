package app

import (
	"fmt"
	"image"
	"math"
	"strconv"
	"strings"
	"sync"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/NimbleMarkets/ntcharts/v2/picture"
	"github.com/NimbleTerminal/gloss/internal/animate"
	"github.com/NimbleTerminal/gloss/internal/document"
	"github.com/NimbleTerminal/gloss/internal/theme"
	"github.com/charmbracelet/x/ansi"
)

type markdownLine struct {
	spans []textSpan
	text  string
	image int // -1 denotes text
	row   int
	line  int // The file line the row shows, when the view knows it; 0 otherwise.
}

type markdownView struct {
	content, sourceText, renderedText *markdownContent
	highlights                        map[int][][2]int
	layoutVersion                     uint64
	doc                               *document.Markdown
	pictures                          []picture.Model
	animators                         []*animate.Animator
	lines                             []markdownLine
	width, height, offset             int
	raw                               bool
	theme                             string
	numbers                           bool // Show file line numbers in a gutter, where rows are file lines.
	fenced                            bool // The document is a file shown as one fenced code block: its rows are the file's lines.
	gutter                            int  // Columns the gutter takes, including its separating space; 0 for none.
	lastLine                          int  // The last file line the rows show; 0 when rows are not file lines.
	chop                              bool // Cut long lines at the right edge instead of wrapping them.
	left, widest                      int  // The columns scrolled off the left edge, and the widest row, when chopping.
}

func newMarkdownView(doc *document.Markdown, id int) *markdownView {
	m := &markdownView{doc: doc, theme: "dark"}
	for i := range doc.Images {
		m.pictures = append(m.pictures, picture.NewWithConfig(picture.Config{KittyID: id + i, KittyZ: -1, KittyFormat: picture.KittyFormatZlib}))
	}
	m.animators = make([]*animate.Animator, len(doc.Images))
	for i, asset := range doc.Images {
		if asset.Animation == nil {
			continue
		}
		a := animate.New(&m.pictures[i], animate.Config{NextID: nextKittyID, Background: pictureBackground})
		a.SetPlayer(asset.Animation)
		a.Transform = func(frame image.Image) image.Image {
			img, err := document.ExportImage(document.Result{Image: frame}, 1600)
			if err != nil {
				return frame
			}
			return img
		}
		m.animators[i] = a
	}
	return m
}

func (m *markdownView) close() tea.Cmd {
	var cmds []tea.Cmd
	for i := range m.pictures {
		if a := m.animators[i]; a != nil {
			cmds = append(cmds, a.Clear())
		} else {
			cmds = append(cmds, m.pictures[i].SetImage(nil))
		}
	}
	return tea.Batch(cmds...)
}

func (m *markdownView) update(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd
	for i := range m.pictures {
		cmd := m.pictures[i].Update(msg)
		if a := m.animators[i]; a != nil {
			cmd = a.Transmission(msg, cmd)
		}
		cmds = append(cmds, cmd)
	}
	return tea.Batch(cmds...)
}

func (m *markdownView) setKitty(kitty bool) tea.Cmd {
	var cmds []tea.Cmd
	for i := range m.pictures {
		if (m.pictures[i].Mode() == picture.PictureKitty) != kitty {
			cmds = append(cmds, m.pictures[i].Toggle())
			if a := m.animators[i]; a != nil {
				cmds = append(cmds, a.Show(a.Transform(a.Source.Image())))
			}
		}
	}
	return tea.Batch(cmds...)
}

func (m *markdownView) layout(width, height, cw, ch int) tea.Cmd {
	m.width, m.height = max(1, width), max(1, height)
	m.lines = nil
	m.layoutVersion++
	m.highlights = nil
	if m.raw {
		if m.sourceText == nil {
			m.sourceText = sourceContent(m.doc.Source)
		}
		m.content = m.sourceText
	} else {
		if m.renderedText == nil {
			m.renderedText = renderedContent(m.doc, m.theme)
		}
		m.content = m.renderedText
	}
	lineOf, last := m.lineMap()
	m.gutter, m.lastLine = 0, 0
	if lineOf != nil {
		m.lastLine = last
	}
	if m.numbers && lineOf != nil {
		m.gutter = min(m.width-1, len(strconv.Itoa(last))+1)
	}
	wrap := m.width - m.gutter
	if m.chop {
		wrap = 1 << 20
	}
	var cmds []tea.Cmd
	for _, block := range m.content.blocks {
		switch {
		case block.table != nil:
			prefix := block.prefix
			if ansi.StringWidth(prefix) >= m.width {
				prefix = ""
			}
			for _, line := range layoutTextTable(block.table, m.content.units, m.width-ansi.StringWidth(prefix)) {
				line.text = prefix + line.text
				for i := range line.spans {
					line.spans[i].offset += len(prefix)
				}
				m.lines = append(m.lines, line)
			}
		case block.image >= 0:
			i := block.image
			asset := m.doc.Images[i]
			if asset.Image == nil {
				continue
			}
			b := asset.Image.Bounds()
			rows := int(math.Ceil(float64(b.Dy()) * float64(m.width*max(1, cw)) / float64(b.Dx()*max(1, ch))))
			rows = max(1, min(rows, max(3, min(20, m.height-2))))
			if a := m.animators[i]; a != nil {
				oldCW, oldCH := m.pictures[i].CellPixelSize()
				w, h := a.Size()
				cmds = append(cmds, m.pictures[i].SetCellPixelSize(cw, ch))
				if !a.HasImage() || w != m.width || h != rows || oldCW != cw || oldCH != ch {
					a.CancelPending()
					a.SetSize(m.width, rows)
					cmds = append(cmds, a.Show(a.Transform(a.Source.Image())))
				}
			} else {
				cmds = append(cmds, m.pictures[i].SetCellPixelSize(cw, ch), m.pictures[i].SetSize(m.width, rows), m.pictures[i].SetImage(asset.Image))
			}
			for row := 0; row < rows; row++ {
				m.lines = append(m.lines, markdownLine{image: i, row: row})
			}
		default:
			rows := wrapTextUnit(m.content.units[block.unit], block.unit, wrap, m.raw)
			if lineOf != nil {
				for i := range rows {
					rows[i].line = lineOf(block.unit)
				}
			}
			m.lines = append(m.lines, rows...)
		}
	}
	m.widest = 0
	for _, line := range m.lines {
		if line.image < 0 {
			m.widest = max(m.widest, ansi.StringWidth(line.text))
		}
	}
	if !m.chop {
		m.left = 0
	}
	m.scroll(0)
	m.scrollX(0)
	return tea.Batch(cmds...)
}

// scrollX moves the chopped rows sideways, no further than the end of the
// widest one.
func (m *markdownView) scrollX(delta int) {
	m.left = max(0, min(m.widest-max(1, m.width-m.gutter), m.left+delta))
}

// lineMap says which file line a unit of the content shows, and the last
// line, or nil where rows are not file lines. In the source view every unit
// is one line. A file shown as one fenced code block has its lines as the
// units between Glamour's margin rows, which the probe measures; the rows
// are checked against the file before they are numbered.
func (m *markdownView) lineMap() (func(unit int) int, int) {
	if m.raw {
		return func(unit int) int { return unit + 1 }, len(m.content.units)
	}
	if !m.fenced {
		return nil, 0
	}
	lines := strings.Split(strings.TrimSuffix(string(m.doc.Source), "\n"), "\n")
	if len(lines) < 3 || !strings.HasPrefix(lines[0], "```") {
		return nil, 0
	}
	fence := lines[0][:strings.LastIndex(lines[0], "`")+1]
	end := -1
	for i := len(lines) - 1; i > 0; i-- {
		if lines[i] == fence {
			end = i
			break
		}
	}
	if end < 0 {
		return nil, 0
	}
	content := lines[1:end]
	top := fenceMargin(m.theme)
	if top < 0 || top+len(content) > len(m.content.units) {
		return nil, 0
	}
	for i, line := range content {
		if squeeze(m.content.units[top+i].plain) != squeeze(line) {
			return nil, 0
		}
	}
	return func(unit int) int {
		if unit < top || unit >= top+len(content) {
			return 0
		}
		return unit - top + 1
	}, len(content)
}

func squeeze(s string) string { return strings.Join(strings.Fields(s), " ") }

var (
	fenceMargins   = map[string]int{}
	fenceMarginsMu sync.Mutex
)

// fenceMargin is how many rows Glamour puts above a fenced code block in a
// theme, measured once by rendering a block of one known line.
func fenceMargin(themeName string) int {
	fenceMarginsMu.Lock()
	defer fenceMarginsMu.Unlock()
	if top, ok := fenceMargins[themeName]; ok {
		return top
	}
	top := -1
	probe := renderedContent(&document.Markdown{Source: []byte("```text\nX\n```\n")}, themeName)
	for i, unit := range probe.units {
		if strings.TrimSpace(unit.plain) == "X" {
			top = i
			break
		}
	}
	fenceMargins[themeName] = top
	return top
}

// numbered reports whether the rows are file lines the view can number and
// go to.
func (m *markdownView) numbered() bool { return m.lastLine > 0 }

// goLine scrolls so that file line n starts at the top row, as far as the
// scroll range allows; a line past the end is the last one.
func (m *markdownView) goLine(n int) bool {
	if !m.numbered() {
		return false
	}
	at := len(m.lines) - 1
	for i, line := range m.lines {
		if line.line >= n {
			at = i
			break
		}
	}
	m.offset = at
	m.scroll(0)
	return true
}

var gutterStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

// gutterFor is the row's gutter: the line number on a line's first row,
// blank on the rows it wraps onto.
func (m *markdownView) gutterFor(i int) string {
	if m.gutter == 0 {
		return ""
	}
	line := m.lines[i]
	if line.line == 0 || (i > 0 && m.lines[i-1].line == line.line) {
		return strings.Repeat(" ", m.gutter)
	}
	return gutterStyle.Render(fmt.Sprintf("%*d", m.gutter-1, line.line)) + " "
}

func (m *markdownView) scroll(delta int) {
	m.offset = max(0, min(max(0, len(m.lines)-m.height), m.offset+delta))
}

func (m *markdownView) key(k string) bool {
	switch k {
	case "j", "down":
		m.scroll(1)
	case "k", "up":
		m.scroll(-1)
	case "space", "pgdown":
		m.scroll(max(1, m.height-1))
	case "b", "pgup":
		m.scroll(-max(1, m.height-1))
	case "ctrl+d":
		m.scroll(max(1, m.height/2))
	case "ctrl+u":
		m.scroll(-max(1, m.height/2))
	case "home", "0":
		m.offset, m.left = 0, 0
	case "end", "G":
		m.scroll(len(m.lines))
	case "left", "h", "right", "l":
		if !m.chop {
			return false
		}
		step := max(1, m.width/4)
		if k == "left" || k == "h" {
			step = -step
		}
		m.scrollX(step)
	default:
		return false
	}
	return true
}

func (m *markdownView) view() string {
	cache := map[int][]string{}
	var lines []string
	for i := m.offset; i < min(len(m.lines), m.offset+m.height); i++ {
		line := m.lines[i]
		if line.image < 0 {
			p := theme.Get(m.theme)
			style := lipgloss.NewStyle().Foreground(p.Foreground).Background(p.Background).Width(m.width)
			text := highlightRanges(line.text, m.highlights[i])
			if m.chop {
				text = ansi.Cut(text, m.left, m.left+max(1, m.width-m.gutter))
			}
			lines = append(lines, style.Render(m.gutterFor(i)+text))
			continue
		}
		rows, ok := cache[line.image]
		if !ok {
			view := m.pictures[line.image].View().Content
			if a := m.animators[line.image]; a != nil {
				view = a.View()
			}
			rows = strings.Split(view, "\n")
			cache[line.image] = rows
		}
		if line.row < len(rows) {
			lines = append(lines, rows[line.row])
		} else {
			lines = append(lines, "")
		}
	}
	return strings.Join(lines, "\n")
}

func (m *markdownView) setTheme(name string) {
	if m.theme == name {
		return
	}
	m.theme = name
	m.renderedText = nil
}

func (m *markdownView) animationMessage(msg tea.Msg) (tea.Cmd, bool) {
	for _, a := range m.animators {
		if a != nil {
			if cmd, ok := a.Update(msg); ok {
				return cmd, true
			}
		}
	}
	return nil, false
}
func (m *markdownView) visibleImages() map[int]bool {
	visible := map[int]bool{}
	if m.raw {
		return visible
	}
	for i := m.offset; i < min(len(m.lines), m.offset+m.height); i++ {
		if m.lines[i].image >= 0 {
			visible[m.lines[i].image] = true
		}
	}
	return visible
}
func (m *markdownView) scheduleAnimations(shown bool) tea.Cmd {
	visible := m.visibleImages()
	var cmds []tea.Cmd
	for i, a := range m.animators {
		if a != nil {
			active := shown && visible[i]
			if !active && a.HasImage() {
				cmds = append(cmds, a.Hide())
			}
			if active && !a.HasImage() {
				cmds = append(cmds, a.Show(a.Transform(a.Source.Image())))
			}
			cmds = append(cmds, a.Schedule(active))
		}
	}
	return tea.Batch(cmds...)
}
func (m *markdownView) animatedFrame() image.Image {
	if m.raw {
		return nil
	}
	for i := m.offset; i < min(len(m.lines), m.offset+m.height); i++ {
		index := m.lines[i].image
		if index >= 0 && m.animators[index] != nil {
			a := m.animators[index]
			if a.Front != nil {
				return a.Front.Image
			}
			return a.Source.Image()
		}
	}
	return nil
}

// keep freezes the displayed pictures for main-screen quit. Removing the
// animators also prevents the update defer from hiding these static placements.
func (m *markdownView) keep() tea.Cmd {
	var cmds []tea.Cmd
	for i, a := range m.animators {
		if a != nil {
			cmds = append(cmds, a.Keep())
			m.animators[i] = nil
		}
	}
	return tea.Batch(cmds...)
}
