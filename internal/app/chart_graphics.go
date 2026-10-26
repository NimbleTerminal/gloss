package app

import (
	"errors"
	"strings"

	tea "charm.land/bubbletea/v2"
	"nimbleterminal.dev/gloss/internal/document"
	"nimbleterminal.dev/gloss/internal/i18n"
	ntpicture "nimbleterminal.dev/ntpicture"
)

// toggleChartGraphics switches charts between text and graphics, for this
// chart and the ones after it, and redraws the pane.
func (m *Model) toggleChartGraphics() tea.Cmd {
	m.chartGraphics = !m.chartGraphics
	if p := m.chartPane; p != nil && !m.chartGraphics {
		p.img, p.imgErr = nil, nil
	}
	if m.chartGraphics {
		m.note = i18n.T("app.chart.graphicsOn", "chart graphics: g switches Kitty / half-blocks")
	} else {
		m.note = i18n.T("app.chart.text", "chart text")
	}
	return m.layoutChartPane()
}

// chartCellPixels is a cell's size in pixels, for drawing a chart that
// fills the pane: the terminal's when it says, else the export cell's.
func (m *Model) chartCellPixels() (int, int) {
	cw, ch := m.cellPixelSize()
	if cw <= 0 || ch <= 0 {
		return document.ChartCellWidth, document.ChartCellHeight
	}
	return cw, ch
}

// syncChartPicture keeps the chart's picture in step with the pane: given
// the graphics drawn and the pane's size, or closed when there are none.
func (m *Model) syncChartPicture(msg tea.Msg) tea.Cmd {
	p := m.chartPane
	if p == nil || !m.chartGraphics || p.img == nil {
		return m.clearChartPicture()
	}
	if m.chartPic == nil {
		m.chartPic = ntpicture.New(ntpicture.Options{
			Session:      m.terminal,
			HostGeometry: true,
			Render:       ntpicture.Config{KittyZ: -1, Background: m.palette().Background, KittyFormat: ntpicture.KittyFormatZlib},
		})
	}
	cw, ch := m.cellPixelSize()
	cmds := []tea.Cmd{m.chartPic.SetTerminal(m.kitty, cw, ch), m.chartPic.SetSize(max(1, p.width), max(1, p.height))}
	if m.chartPicImg != p.img {
		m.chartPicImg = p.img
		cmds = append(cmds, m.chartPic.SetImageImmutable(p.img))
	}
	return tea.Batch(append(cmds, m.chartPic.Update(msg))...)
}

// chartPicView is the chart's picture as cells, or "" while it has none to
// show: the text chart stands in until then.
func (m *Model) chartPicView() string {
	if m.chartPic == nil || !m.chartGraphics || m.chartPane == nil || m.chartPane.img == nil || m.chartPicImg != m.chartPane.img {
		return ""
	}
	return m.chartPic.View()
}

func (m *Model) clearChartPicture() tea.Cmd {
	if m.chartPic == nil {
		return nil
	}
	pic := m.chartPic
	m.chartPic, m.chartPicImg = nil, nil
	return pic.Close()
}

// rasterHint is how the chart compiler says a chart needs its raster
// renderer, which gloss offers as ctrl+g.
const rasterHint = `To draw other chart types as an image, add "renderer": "raster" to the input.`

// chartError is a chart's error as the pane says it, in the language in
// use: gloss's own reasons translated, and in the compiler's text, which is
// left as it is, the raster hint becomes the key that gives it.
func chartError(err error) string {
	switch {
	case errors.Is(err, errChartTooSmall):
		return i18n.T("app.chart.tooSmall", "the terminal is too small for a chart")
	case errors.Is(err, errNothingToChart):
		return i18n.T("app.chart.nothing", "nothing to chart: show a date or category column and a number column")
	case errors.Is(err, errChartNoRows):
		return i18n.T("app.chart.noRows", "the chart has no rows and no table is open")
	case errors.Is(err, errNoTableToChart):
		return i18n.T("app.chart.noTable", "no table is open to chart")
	}
	return strings.Replace(err.Error(), rasterHint, i18n.T("app.chart.rasterHint", "ctrl+g draws it as graphics."), 1)
}
