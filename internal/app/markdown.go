package app

import (
	"github.com/NimbleTerminal/gloss/internal/animate"
	"image"
	"math"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/NimbleMarkets/ntcharts/v2/picture"
	"github.com/NimbleTerminal/gloss/internal/document"
	"github.com/charmbracelet/x/ansi"
)

type markdownLine struct {
	spans []textSpan
	text  string
	image int // -1 denotes text
	row   int
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
}

func newMarkdownView(doc *document.Markdown, id int) *markdownView {
	m := &markdownView{doc: doc}
	for i := range doc.Images {
		m.pictures = append(m.pictures, picture.NewWithConfig(picture.Config{KittyID: id + i, KittyZ: -1}))
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
			m.lines = append(m.lines, wrapTextUnit(m.content.units[block.unit], block.unit, m.width, m.raw)...)
		}
	}
	m.scroll(0)
	return tea.Batch(cmds...)
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
		m.offset = 0
	case "end", "G":
		m.scroll(len(m.lines))
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
			lines = append(lines, highlightRanges(line.text, m.highlights[i]))
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
