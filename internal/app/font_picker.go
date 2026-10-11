package app

import (
	"context"
	"image"
	"image/draw"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/NimbleMarkets/ntcharts/v2/picture"
	"github.com/charmbracelet/x/ansi"
	"nimbleterminal.dev/gloss/internal/document"
)

type fontPicker struct {
	palette               string
	pic                   picture.Model
	owner, revision       int
	width, height, cw, ch int
	cursor                int
	font, mono            string
	base                  image.Image
	cancel                context.CancelFunc
	err                   error
}
type fontPreviewResult struct {
	owner, revision int
	image           image.Image
	err             error
}
type fontSaved struct{ err error }

func (m *Model) setFont(name, mono string) tea.Cmd {
	if document.FontKey(m.opts.Font, m.opts.Mono) == document.FontKey(name, mono) {
		return nil
	}
	m.opts.Font, m.opts.Mono = name, mono
	return m.queueTypesetReflow()
}
func (m *Model) openFontPicker() tea.Cmd {
	id := nextKittyID()
	cw, ch := m.pic.CellPixelSize()
	p := &fontPicker{owner: id, font: m.opts.Font, mono: m.opts.Mono, pic: picture.NewWithConfig(picture.Config{KittyID: id, KittyZ: -1, Background: m.palette().Background, CellPixelWidth: cw, CellPixelHeight: ch})}
	for i, l := range document.FontLooks() {
		if l.Name == m.opts.Font {
			p.cursor = i
		}
	}
	m.fontPicker = p
	m.layer = layerFont
	return m.layoutFontPicker()
}
func (m *Model) layoutFontPicker() tea.Cmd {
	p := m.fontPicker
	if p == nil {
		return nil
	}
	cw, ch := m.pic.CellPixelSize()
	w, h := max(1, min(64, m.width-4)), max(1, min(24, m.bodyHeight()-5))
	var mode tea.Cmd
	if p.pic.Mode() != m.pic.Mode() {
		mode = p.pic.Toggle()
	}
	if p.width == w && p.height == h && p.cw == cw && p.ch == ch && p.palette == m.theme {
		return mode
	}
	if p.cancel != nil {
		p.cancel()
	}
	p.width, p.height, p.cw, p.ch = w, h, cw, ch
	p.palette = m.theme
	p.revision++
	cmds := []tea.Cmd{mode, p.pic.SetSize(w, h), p.pic.SetCellPixelSize(cw, ch)}
	if p.pic.Mode() != m.pic.Mode() {
		cmds = append(cmds, p.pic.Toggle())
	}
	if w < 24 || h < 10 {
		p.base = nil
		return tea.Batch(append(cmds, p.pic.SetImage(nil))...)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	p.cancel = cancel
	owner, revision := p.owner, p.revision
	source := m.typeset.Source
	palette, mono := m.theme, m.opts.Mono
	cmds = append(cmds, func() tea.Msg {
		defer cancel()
		im, err := document.FontPreview(ctx, source, palette, mono, w*max(1, cw), h*max(1, ch))
		return fontPreviewResult{owner, revision, im, err}
	})
	return tea.Batch(cmds...)
}
func (m *Model) showFontPreview() tea.Cmd {
	p := m.fontPicker
	if p == nil || p.base == nil {
		return nil
	}
	im := image.NewRGBA(p.base.Bounds())
	draw.Draw(im, im.Bounds(), p.base, im.Bounds().Min, draw.Src)
	row := im.Bounds().Dy() / 4
	y := row * p.cursor
	accent := image.NewUniform(m.palette().Accent)
	draw.Draw(im, image.Rect(0, y, 3, y+row), accent, image.Point{}, draw.Src)
	return p.pic.SetImage(im)
}
func (m *Model) clearFontPicker() tea.Cmd {
	p := m.fontPicker
	if p == nil {
		return nil
	}
	m.fontPicker = nil
	if m.layer == layerFont {
		m.layer = layerNone
	}
	if p.cancel != nil {
		p.cancel()
	}
	return p.pic.SetImage(nil)
}
func (m *Model) fontPickerKey(k string) tea.Cmd {
	p := m.fontPicker
	if p == nil {
		return nil
	}
	switch k {
	case "esc", "f":
		return m.closeLayer()
	case "Q":
		m.opts.KeepScreen = true
		return m.quit()
	case "ctrl+c", "q":
		return m.quit()
	case "+", "=", "-":
		cmd, _ := m.typesetKey(k)
		return cmd
	case "up", "k":
		p.cursor = max(0, p.cursor-1)
	case "down", "j":
		p.cursor = min(3, p.cursor+1)
	case "enter":
		name, mono := m.opts.Font, m.opts.Mono
		cleanup := m.clearFontPicker()
		m.layer = layerNone
		if save := m.opts.SaveFont; save != nil {
			return tea.Batch(cleanup, func() tea.Msg { return fontSaved{save(name, mono)} })
		}
		return cleanup
	default:
		return nil
	}
	return tea.Batch(m.setFont(document.FontLooks()[p.cursor].Name, m.opts.Mono), m.showFontPreview())
}
func (m *Model) fontPickerView() string {
	p := m.fontPicker
	if p == nil {
		return ""
	}
	var body string
	if p.base != nil && p.err == nil {
		body = p.pic.View().Content
	} else {
		var lines []string
		for i, l := range document.FontLooks() {
			prefix := "  "
			if i == p.cursor {
				prefix = "› "
			}
			lines = append(lines, prefix+strings.ToUpper(l.Name[:1])+l.Name[1:])
		}
		body = strings.Join(lines, "\n")
		if p.err != nil {
			body += "\n" + ansi.Truncate("Preview unavailable: "+safe(p.err.Error()), p.width, "…")
		}
	}
	title := "Fonts · " + document.FontLooks()[p.cursor].Name
	return ansi.Truncate(title, p.width, "…") + "\n" + body + "\n" + ansi.Truncate("↑/↓ preview · Enter keep · Esc cancel", p.width, "…")
}
