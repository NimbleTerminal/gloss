package app

import (
	"image"

	"context"
	"math"
	"slices"
	"time"

	tea "charm.land/bubbletea/v2"
	"nimbleterminal.dev/gloss/internal/document"
	"nimbleterminal.dev/gloss/internal/i18n"
)

// typesetResizeDelay coalesces a burst of width changes before reflowing. It
// was 60 ms when a reflow cost ten times what it does now; at 30 ms a resize
// feels live and still folds a drag's events together.
const typesetResizeDelay = 30 * time.Millisecond

// typesetSettleDelay is how long a page view stays unchanged before its
// animated regions get their overlays. A scroll or resize burst then costs
// the page alone, with each GIF standing at its current frame.
const typesetSettleDelay = 120 * time.Millisecond

type typesetSettled struct {
	owner    int
	revision uint64
}

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

// typesetCell is a body cell's shape as the page picture is drawn into it:
// the terminal's cell in pixels with Kitty graphics. Glyphs draw every
// cell as one pixel by two, whatever the terminal's: here two by four, so
// that a cell has a middle.
func (m *Model) typesetCell() (int, int) {
	if m.kitty {
		return m.cellPixelSize()
	}
	return 2, 4
}

func (m *Model) typesetMeasure() int {
	if !m.opts.typesetEnabled() || m.isPreview || m.width < 1 {
		return 0
	}
	cw, _ := m.cellPixelSize()
	return max(192, min(960, int(math.Round(float64(m.width*max(1, cw))*72/150))))
}
func (m *Model) typesetViewport() document.TypesetViewport {
	cw, ch := m.typesetCell()
	width := float64(m.typesetMeasure()) * 150 / 72
	h := int(math.Round(width * float64(m.bodyHeight()*max(1, ch)) / float64(max(1, m.width)*max(1, cw))))
	v := document.TypesetViewport{Top: m.typesetTop, Height: max(1, min(4096, h)), Left: m.typesetLeft, Zoom: m.zoom, End: m.typesetEnd, Line: m.typesetGoLine}
	// A Tab lays on to the next item; a Shift-Tab, round to the last.
	v.Link, v.Ahead, v.Last = m.typesetGoLink, m.typesetAhead && m.typesetTabPending > 0, m.typesetAhead && m.typesetTabPending < 0
	if m.typesetGoTop >= 0 {
		v.Top = m.typesetGoTop
	}
	if m.kitty {
		// The picture fills the body's cell area; a raster of exactly that
		// size is shown as it is instead of being resampled.
		v.Pixels = image.Pt(m.width*max(1, cw), m.bodyHeight()*max(1, ch))
	}
	return v
}

// typesetMatches says the shown picture is the one the viewer's state asks
// for; typesetViewMatches, only that it shows the same part of the page,
// which is what mapping a cell onto the page needs.
func (m *Model) typesetMatches() bool {
	return m.typesetViewMatches() && selectionsEqual(m.typesetSelect, m.typesetSelectShown) && rectsEqual(m.typesetFocus, m.typesetFocusShown) && slices.Equal(m.typesetHover, m.typesetHoverShown)
}
func (m *Model) typesetViewMatches() bool {
	return m.typesetFont == document.FontKey(m.opts.Font, m.opts.Mono) && m.viewportAsked(m.typesetShown, m.typesetWidth, m.typesetTheme)
}

// viewportAsked says a rendered part of the page, at a measure and in a
// theme, is the one the viewer's state asks for now.
func (m *Model) viewportAsked(shown *document.TypesetViewport, width int, theme string) bool {
	v := m.typesetViewport()
	return shown != nil && theme == m.theme && !v.End && !v.Ahead && !v.Last && v.Line == 0 && v.Link == "" && shown.Numbers == m.lineNumbers && shown.Chop == m.chopLines && m.typesetMeasure() == width && v.Top == shown.Top && v.Left == shown.Left && v.Zoom == shown.Zoom && max(1, v.Height>>v.Zoom) == shown.Height && v.Pixels == shown.Pixels
}

// queueTypesetReflow asks for the page as the viewer's state now wants it,
// giving up a job in flight for an older state. queueTypesetSelection does
// the same for a change of selection alone, but lets the job finish: its
// page is the right part of the document with an earlier selection, which
// is shown on the way, so a drag is followed rather than waited for.
func (m *Model) queueTypesetReflow() tea.Cmd    { return m.queueTypeset(true) }
func (m *Model) queueTypesetSelection() tea.Cmd { return m.queueTypeset(false) }
func (m *Model) queueTypeset(cancel bool) tea.Cmd {
	if m.typeset == nil || m.isPreview || m.loading {
		return nil
	}
	if m.typesetMatches() && m.typesetActive == 0 {
		return nil
	}
	if cancel {
		m.stopTypeset()
	}
	m.typesetRevision++
	msg := typesetResize{m.ownerID, m.typesetRevision}
	if m.typesetWidth == m.typesetMeasure() {
		return func() tea.Msg { return msg }
	}
	return tea.Tick(typesetResizeDelay, func(time.Time) tea.Msg { return msg })
}
func (m *Model) reflowTypeset(msg typesetResize) tea.Cmd {
	if msg.owner != m.ownerID || msg.revision != m.typesetRevision || m.typeset == nil || m.loading || m.typesetActive != 0 || m.typesetMatches() {
		return nil
	}
	m.generation++
	m.typesetActive = m.generation
	q, loader := m.request(false), m.loader
	q.Context, m.typesetCancel = context.WithCancel(context.Background())
	return func() tea.Msg { return typesetReflowResult{msg.owner, msg.revision, loader.Load(q)} }
}
func (m *Model) typesetReflowed(msg typesetReflowResult) tea.Cmd {
	if msg.owner != m.ownerID {
		return nil
	}
	v := msg.result
	if m.typesetActive == v.Generation {
		m.typesetActive = 0
	}
	stale := v.Generation != m.generation || msg.revision != m.typesetRevision
	if m.typeset == nil || (stale && (v.Err != nil || v.FontKey != document.FontKey(m.opts.Font, m.opts.Mono) || !m.viewportAsked(v.TypesetView, v.TypesetWidth, v.Theme))) {
		return m.queueTypesetReflow()
	}
	if v.Err != nil {
		m.note = i18n.Tf("app.note.typesetFailed", "typeset: %s", safe(v.Err.Error()))
		m.typesetTabPending, m.typesetGoTop, m.typesetAhead = 0, -1, false
		return nil
	}
	m.note = ""
	m.source, m.fields = v.Image, v.Info
	m.typeset, m.typesetWidth = v.Typeset, v.TypesetWidth
	overlays := m.acceptTypesetView(v)
	if stale {
		// The part of the page is right; only the selection is behind.
		overlays = tea.Batch(overlays, m.queueTypesetReflow())
	}
	if m.markdown != nil {
		return overlays
	}
	return tea.Batch(m.showPicture(m.source), overlays)
}

// acceptTypesetView takes a page view over, returning the overlays' commands.
// Under Kitty graphics the animated regions play in overlays and the page
// picture stays still; otherwise the composite page animates as a whole.
func (m *Model) acceptTypesetView(v document.Result) tea.Cmd {
	m.typesetLinks = v.Links
	m.typesetItems, m.typesetFocusShown, m.typesetHoverShown = v.TypesetItems, v.TypesetFocus, v.TypesetMarks
	m.typesetSelectShown, m.typesetSelected = v.TypesetSelection, v.TypesetSelected
	// When the result answers the selection as it is now, that becomes the
	// placed one, both ends in the text, or nothing when the text has no
	// such place. A drag that has closed on nothing is no selection.
	if selectionsEqual(v.TypesetAsked, m.typesetSelect) {
		m.typesetSelect = nil
		if v.TypesetSelection != nil {
			// A copy: the shown one must stay what the picture has while a
			// drag moves this one on.
			placed := *v.TypesetSelection
			m.typesetSelect = &placed
		}
		if m.typesetSelected == "" && !m.typesetDragging {
			m.typesetSelect, m.typesetSelectShown = nil, nil
		}
	}
	m.typesetPlayback = v.Playback
	var overlays tea.Cmd
	m.typesetPending = nil
	if v.Playback != nil && !m.isPreview {
		if m.kitty && len(v.Animated) > 0 && v.TypesetView != nil {
			pending := v
			m.typesetPending = &pending
			m.typesetSettle++
			owner, revision := m.ownerID, m.typesetSettle
			overlays = tea.Tick(typesetSettleDelay, func(time.Time) tea.Msg { return typesetSettled{owner, revision} })
		}
		overlays = tea.Batch(overlays, m.showTypesetLayers(v.Playback))
	} else {
		overlays = m.clearTypesetLayers()
	}
	m.typesetShown = v.TypesetView
	m.typesetTheme = v.Theme
	m.typesetFont = v.FontKey
	if v.TypesetView != nil {
		m.typesetTop, m.typesetLeft = v.TypesetView.Top, v.TypesetView.Left
		m.typesetEnd, m.typesetGoLine = false, 0
		m.typesetGoLink, m.typesetGoTop, m.typesetAhead = "", -1, false
		m.typesetDone = v.TypesetDone || v.TypesetItemsAll
		overlays = tea.Batch(overlays, m.followTypesetFocus())
	}
	return overlays
}

// typesetSettled gives the last page view its overlays, unless it has moved on.
func (m *Model) typesetSettled(msg typesetSettled) tea.Cmd {
	if msg.owner != m.ownerID || msg.revision != m.typesetSettle || m.typesetPending == nil || m.typesetActive != 0 || !m.typesetMatches() {
		return nil
	}
	m.typesetPending = nil
	if m.typesetLayers != nil {
		return m.typesetLayers.SetActive(true)
	}
	return nil
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
