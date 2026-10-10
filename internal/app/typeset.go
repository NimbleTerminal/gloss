package app

import (
	"image"

	"context"
	"github.com/NimbleMarkets/ntcharts/v2/picture"
	"math"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/NimbleTerminal/gloss/internal/document"
)

const typesetResizeDelay = 60 * time.Millisecond

type typesetResize struct {
	owner    int
	revision uint64
}
type typesetReflowResult struct {
	owner    int
	revision uint64
	result   document.Result
}

func (m *Model) stopTypeset() {
	if m.typesetCancel != nil {
		m.typesetCancel()
		m.typesetCancel = nil
	}
}
func (m *Model) toggleTypeset() tea.Cmd {
	m.opts.Typeset = !m.opts.typesetEnabled()
	m.opts.MarkdownMode = "text"
	m.clearSearch()
	m.typesetRevision++
	m.typesetActive = 0
	m.page, m.pages = 1, 1
	m.source, m.zoom, m.panX, m.panY = nil, 0, 0, 0
	m.savedMarkdown = nil
	return tea.Sequence(m.clearGraphics(), m.load(false))
}
func (m *Model) typesetMeasure() int {
	if !m.opts.typesetEnabled() || m.isPreview || m.width < 1 {
		return 0
	}
	cw, _ := m.pic.CellPixelSize()
	return max(192, min(960, int(math.Round(float64(m.width*max(1, cw))*72/150))))
}
func (m *Model) typesetViewport() document.TypesetViewport {
	cw, ch := m.pic.CellPixelSize()
	width := float64(m.typesetMeasure()) * 150 / 72
	h := int(math.Round(width * float64(m.bodyHeight()*max(1, ch)) / float64(max(1, m.width)*max(1, cw))))
	v := document.TypesetViewport{Top: m.typesetTop, Height: max(1, min(4096, h)), Left: m.typesetLeft, Zoom: m.zoom, End: m.typesetEnd}
	if m.pic.Mode() == picture.PictureKitty {
		// The picture fills the body's cell area; a raster of exactly that
		// size is shown as it is instead of being resampled.
		v.Pixels = image.Pt(m.width*max(1, cw), m.bodyHeight()*max(1, ch))
	}
	return v
}
func (m *Model) typesetMatches() bool {
	v := m.typesetViewport()
	shown := m.typesetShown
	return shown != nil && m.typesetTheme == m.theme && !v.End && m.typesetMeasure() == m.typesetWidth && v.Top == shown.Top && v.Left == shown.Left && v.Zoom == shown.Zoom && max(1, v.Height>>v.Zoom) == shown.Height && v.Pixels == shown.Pixels
}
func (m *Model) queueTypesetReflow() tea.Cmd {
	if m.typeset == nil || m.isPreview || m.loading {
		return nil
	}
	if m.typesetMatches() && m.typesetActive == 0 {
		return nil
	}
	m.presentation.CancelPending()
	m.stopTypeset()
	m.typesetRevision++
	msg := typesetResize{m.kittyID, m.typesetRevision}
	if m.typesetWidth == m.typesetMeasure() {
		return func() tea.Msg { return msg }
	}
	return tea.Tick(typesetResizeDelay, func(time.Time) tea.Msg { return msg })
}
func (m *Model) reflowTypeset(msg typesetResize) tea.Cmd {
	if msg.owner != m.kittyID || msg.revision != m.typesetRevision || m.typeset == nil || m.loading || m.typesetActive != 0 || m.typesetMatches() {
		return nil
	}
	m.generation++
	m.typesetActive = m.generation
	q, loader := m.request(false), m.loader
	q.Context, m.typesetCancel = context.WithCancel(context.Background())
	return func() tea.Msg { return typesetReflowResult{msg.owner, msg.revision, loader.Load(q)} }
}
func (m *Model) typesetReflowed(msg typesetReflowResult) tea.Cmd {
	if msg.owner != m.kittyID {
		return nil
	}
	v := msg.result
	if m.typesetActive == v.Generation {
		m.typesetActive = 0
	}
	if v.Generation != m.generation || msg.revision != m.typesetRevision || m.typeset == nil {
		return m.queueTypesetReflow()
	}
	if v.Err != nil {
		m.note = "typeset: " + safe(v.Err.Error())
		return nil
	}
	m.note = ""
	m.source, m.fields = v.Image, v.Info
	m.typeset, m.typesetWidth = v.Typeset, v.TypesetWidth
	m.acceptTypesetView(v)
	if m.markdown != nil {
		return nil
	}
	return m.showPicture(m.source)
}
func (m *Model) acceptTypesetView(v document.Result) {
	m.typesetLinks = v.Links
	m.typesetPlayback = v.Playback
	if v.Playback != nil {
		m.presentation.SetSource(v.Playback)
		m.presentation.Paused = false
		m.presentation.SetSpeed(0)
	}
	m.typesetShown = v.TypesetView
	m.typesetTheme = v.Theme
	if v.TypesetView != nil {
		m.typesetTop, m.typesetLeft = v.TypesetView.Top, v.TypesetView.Left
		m.typesetEnd = false
	}
}
func (m *Model) typesetKey(k string) (tea.Cmd, bool) {
	step := max(1, 75>>m.zoom)
	page := max(1, m.typesetViewport().Height>>m.zoom)
	switch k {
	case "up", "k":
		m.typesetTop = max(0, m.typesetTop-step)
	case "down", "j":
		m.typesetTop += step
	case "space", "pgdown", "n":
		m.typesetTop += page
	case "b", "pgup", "p":
		m.typesetTop = max(0, m.typesetTop-page)
	case "home", "0", "f":
		m.typesetTop, m.typesetLeft, m.zoom = 0, 0, 0
	case "end", "G":
		m.typesetEnd = true
	case "left", "h":
		m.typesetLeft = max(0, m.typesetLeft-step)
	case "right", "l":
		m.typesetLeft += step
	case "+", "=":
		m.zoom = min(4, m.zoom+1)
	case "-", "_":
		m.zoom = max(0, m.zoom-1)
	default:
		return nil, false
	}
	return m.queueTypesetReflow(), true
}

func (o Options) typesetEnabled() bool { return o.Typeset || o.MarkdownMode == "typeset" }

func (m *Model) canTypeset() bool {
	return m.kind == "markdown" || m.kind == "text" || m.kind == "json" || m.typeset != nil
}
