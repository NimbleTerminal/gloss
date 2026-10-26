package app

import (
	"context"
	"fmt"
	"image"
	"io"
	"os"
	"strings"
	"sync/atomic"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"nimbleterminal.dev/gloss/internal/chart"
	"nimbleterminal.dev/gloss/internal/document"
)

// chartSplitWidth is the terminal width from which the chart stands beside
// the grid rather than in its place.
const chartSplitWidth = 120

// chartPane draws a chart of the sheet's visible columns, or a chart
// document, compiled off the event loop.
type chartPane struct {
	shape         chart.Shape
	templates     []string
	template      int
	doc           *chart.Document // A chart document page; nil for an inferred chart.
	source        *chart.Document // A document drawn over the sheet, bound to its rows; nil infers.
	bound         []chart.Note    // What binding the source set aside.
	boundRows     int             // The rows binding the source kept.
	series        int             // The series the last good compile drew.
	compiled      *chart.Document // The document the view was compiled from, for the browser page.
	cols          []int           // The sheet columns the shape was read from.
	revision      int
	width, height int
	view          string      // The last good chart.
	img           image.Image // The chart drawn as graphics, while ctrl+g asks for them.
	imgErr        error       // Why graphics could not draw it; the text chart stands in.
	drawn         bool        // Whether the last compile drew graphics.
	warnings      []chart.Warning
	err           error
	cancel        context.CancelFunc
	reply         chan<- ChartReply // Who pushed the source, until the first compile answers.
	claim         *PushClaim        // The push's claim, while reply waits.
}

type chartResult struct {
	pane     *chartPane
	revision int
	doc      *chart.Document // What was compiled, for the browser page once the view is drawn.
	view     string
	img      image.Image // Graphics, when they were asked for and drew.
	imgErr   error
	kind     string // The Spec's chart type.
	series   int
	warnings []chart.Warning
	err      error
}

// ChartPush is a chart document handed to the viewer from outside, with a
// channel for what became of it.
type ChartPush struct {
	Doc   chart.Document
	Reply chan<- ChartReply
	Claim *PushClaim // Nil when the pusher always waits for the reply.
}

// PushClaim settles a push once: the viewer keeps the chart when it is
// drawn, or the pusher gives up on it first (a timeout, a request gone),
// and then the viewer drops it, so a push reported lost is never shown.
type PushClaim struct{ state atomic.Int32 }

const (
	claimOpen int32 = iota
	claimKept
	claimAbandoned
)

// Abandon gives the push up, unless the viewer has kept it; it reports
// whether it did, and if not the reply is on its way.
func (c *PushClaim) Abandon() bool { return c.state.CompareAndSwap(claimOpen, claimAbandoned) }

func (c *PushClaim) keep() bool {
	return c == nil || c.state.CompareAndSwap(claimOpen, claimKept)
}

func (c *PushClaim) abandoned() bool { return c != nil && c.state.Load() == claimAbandoned }

// ChartReply says what the viewer drew from a push, or why it did not.
type ChartReply struct {
	Type     string          `json:"type,omitempty"`
	Series   int             `json:"series,omitempty"`
	Rows     int             `json:"rows,omitempty"`
	Warnings []chart.Warning `json:"warnings"`
	Error    string          `json:"error,omitempty"`
}

type chartPushMsg ChartPush

// awaitCharts waits for the next chart from outside; each arrival renews it.
func (m *Model) awaitCharts() tea.Cmd {
	pushes := m.opts.Charts
	if pushes == nil {
		return nil
	}
	return func() tea.Msg {
		push, ok := <-pushes
		if !ok {
			return nil
		}
		return chartPushMsg(push)
	}
}

// pushed draws a pushed chart and arranges to answer when it is compiled,
// or answers at once with what stood in the way.
func (m *Model) pushed(push chartPushMsg) tea.Cmd {
	if push.Claim.abandoned() {
		return nil
	}
	doc := push.Doc
	cmd := m.applyChart(&doc)
	if cmd == nil {
		reason := m.note
		if p := m.chartPane; p != nil && p.err != nil {
			reason = p.err.Error()
		}
		if reason == "" {
			reason = "the chart could not be drawn"
		}
		answer(push.Reply, ChartReply{Error: reason})
		return nil
	}
	m.chartPane.reply, m.chartPane.claim = push.Reply, push.Claim
	return cmd
}

// answer sends a reply to whoever pushed, if anyone still waits.
func answer(reply chan<- ChartReply, r ChartReply) {
	if reply == nil {
		return
	}
	if r.Warnings == nil {
		r.Warnings = []chart.Warning{}
	}
	select {
	case reply <- r:
	default:
	}
}

// toggleChartPane opens the chart beside the sheet, or closes it.
func (m *Model) toggleChartPane() tea.Cmd {
	if m.chartPane != nil && m.chartPane.doc == nil {
		m.closeChartPane()
		m.chartWanted = false
		return nil
	}
	if m.sheet == nil || m.chartPane != nil {
		return nil
	}
	m.chartWanted = true
	m.chartPane = &chartPane{source: m.opts.Chart}
	return m.layoutChartPane()
}

// rereadChart reads the --chart file again, for a reload; a file that
// cannot be read leaves the chart as it was and says so.
func (m *Model) rereadChart() {
	if m.opts.ChartPath == "" {
		return
	}
	f, err := os.Open(m.opts.ChartPath)
	if err != nil {
		m.note = "--chart: " + safe(err.Error())
		return
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, chart.MaxDocumentBytes+1))
	if err == nil {
		var doc chart.Document
		if doc, err = chart.Sniff(data); err == nil {
			m.opts.Chart = &doc
			return
		}
	}
	m.note = "--chart: " + safe(err.Error())
}

// applyChart draws doc over the sheet, or as a page when no sheet is open.
func (m *Model) applyChart(doc *chart.Document) tea.Cmd {
	switch {
	case m.sheet != nil && !m.suspended:
		m.chartWanted = true
		m.closeChartPane()
		m.chartPane = &chartPane{source: doc}
		return m.layoutChartPane()
	case doc.Unbound:
		m.note = "the chart has no rows and no table is open"
		return nil
	case m.chartPane == nil || m.chartPane.doc == nil || m.suspended:
		// Over a picture, a mesh or a text the chart would not be seen:
		// say so rather than report it drawn.
		m.note = "no table is open to chart"
		return nil
	}
	m.closeChartPane()
	m.chartPane = &chartPane{doc: doc}
	return m.layoutChartPane()
}

func (m *Model) closeChartPane() {
	if p := m.chartPane; p != nil {
		if p.cancel != nil {
			p.cancel()
		}
		answer(p.reply, ChartReply{Error: "the chart was replaced"})
		p.reply = nil
		if m.opts.Charted != nil {
			m.opts.Charted(nil)
		}
	}
	m.chartPane = nil
}

// chartSplit divides the body between the grid and the pane.
func (m *Model) chartSplit() (grid, pane int) {
	switch {
	case m.chartPane == nil:
		return m.width, 0
	case m.chartPane.doc != nil || m.width < chartSplitWidth:
		return 0, m.width
	}
	grid = m.width / 2
	return grid, m.width - grid - 1
}

// sheetWidth is what the grid may use, beside the chart or alone.
func (m *Model) sheetWidth() int {
	grid, _ := m.chartSplit()
	if grid == 0 {
		return m.width
	}
	return grid
}

// layoutChartPane sizes the pane and compiles when the size, the columns
// or the template changed since the last chart.
func (m *Model) layoutChartPane() tea.Cmd {
	p := m.chartPane
	if p == nil {
		return nil
	}
	_, w := m.chartSplit()
	h := m.bodyHeight() - 1 // The pane's title line.
	if p.doc == nil {
		if m.sheet == nil {
			m.closeChartPane()
			return nil
		}
		cols := append([]int(nil), m.sheet.cols...)
		if !equalInts(cols, p.cols) {
			rows := m.sheet.sheet.Rows
			var header []string
			if len(rows) > 0 {
				header, rows = rows[0], rows[1:]
			}
			p.shape = chart.Analyze(header, rows, cols)
			p.templates, p.template, p.cols = p.shape.Templates(), 0, cols
			p.view, p.img = "", nil
		}
	}
	if p.width == w && p.height == h && p.shown() && p.err == nil && p.drawn == m.chartGraphics {
		return nil
	}
	p.width, p.height, p.drawn = w, h, m.chartGraphics
	if p.cancel != nil {
		p.cancel()
	}
	p.revision++ // Whatever was compiling answers to the old revision now.
	if w < 10 || h < 4 {
		p.err = fmt.Errorf("the terminal is too small for a chart")
		return nil
	}
	var doc chart.Document
	switch {
	case p.doc != nil:
		doc = *p.doc
	case p.source != nil:
		rows := m.sheet.sheet.Rows
		var header []string
		if len(rows) > 0 {
			header, rows = rows[0], rows[1:]
		}
		bound, notes, err := chart.Bind(*p.source, header, rows, p.cols)
		if err != nil {
			p.view, p.err = "", err
			m.reportChart()
			return nil
		}
		doc, p.bound, p.boundRows = bound, notes, min(len(rows), chart.MaxRows)
	case len(p.templates) == 0:
		p.view, p.err = "", fmt.Errorf("nothing to chart: show a date or category column and a number column")
		m.reportChart()
		return nil
	default:
		var err error
		if doc, err = p.shape.Document(p.templates[p.template]); err != nil {
			p.err = err
			return nil
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	rev, graphics := p.revision, p.drawn
	cw, ch := m.chartCellPixels()
	return func() tea.Msg {
		defer cancel()
		r := chartResult{pane: p, revision: rev, doc: &doc}
		res, err := chart.Default.Compile(ctx, doc, w, h)
		if err == nil {
			r.kind, r.series, r.warnings = string(res.Spec.Type), len(res.Spec.Data.Series), res.Warnings
			r.view, err = chart.RenderText(res.Spec)
		}
		r.err = err
		if graphics {
			// The text chart stands in when graphics cannot draw; graphics
			// draw what text cannot, such as a pie.
			r.img, r.imgErr = chart.Default.Raster(ctx, doc, w*cw, h*ch)
			if err != nil && r.img != nil {
				r.view, r.kind, r.err = "", "image", nil
			}
		}
		if r.err != nil {
			r.doc = nil
		}
		return r
	}
}

// reportChart tells the host what the pane draws: the document behind the
// view, or nothing while the pane is blank.
func (m *Model) reportChart() {
	if m.opts.Charted == nil {
		return
	}
	if p := m.chartPane; p != nil && p.shown() {
		m.opts.Charted(p.compiled)
		return
	}
	m.opts.Charted(nil)
}

// syncChartPane recompiles when the grid's columns changed under the pane.
func (m *Model) syncChartPane() tea.Cmd {
	if m.chartPane == nil || m.chartPane.doc != nil || m.sheet == nil || equalInts(m.chartPane.cols, m.sheet.cols) {
		return nil
	}
	return m.layoutChartPane()
}

// nextChartTemplate draws the same columns as the next template that fits them.
func (m *Model) nextChartTemplate() tea.Cmd {
	p := m.chartPane
	if p == nil || p.doc != nil || p.source != nil || len(p.templates) < 2 {
		return nil
	}
	p.template = (p.template + 1) % len(p.templates)
	p.view, p.img = "", nil
	return m.layoutChartPane()
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// chartCompiled takes a compile result, unless a newer compile is in flight.
func (m *Model) chartCompiled(r chartResult) {
	p := m.chartPane
	if p == nil || r.pane != p || r.revision != p.revision {
		return
	}
	if p.reply != nil && !p.claim.keep() {
		// The pusher gave up before this was drawn and told its caller so.
		p.reply = nil
		m.closeChartPane()
		return
	}
	p.err, p.warnings = r.err, r.warnings
	if r.err == nil {
		p.view, p.compiled, p.series = r.view, r.doc, r.series
		p.img, p.imgErr = r.img, r.imgErr
		m.reportChart()
	}
	if p.reply != nil {
		out := ChartReply{Type: r.kind, Series: r.series, Rows: p.rows(), Warnings: r.warnings}
		if r.err != nil {
			out.Error = r.err.Error()
		}
		answer(p.reply, out)
		p.reply, p.claim = nil, nil
	}
}

// shown reports whether the pane draws a chart: text, graphics or both.
func (p *chartPane) shown() bool { return p.view != "" || p.img != nil }

// rows is how many of the sheet's rows the chart draws: those binding kept
// for a document over the sheet, else those the inferred chart took, after
// MaxRows and MaxValues.
func (p *chartPane) rows() int {
	if p.source != nil {
		return p.boundRows
	}
	return p.shape.Rows
}

// chartTitle names what the pane draws.
func (m *Model) chartTitle() string {
	p := m.chartPane
	switch {
	case p == nil:
		return ""
	case p.doc != nil:
		return "chart · " + p.doc.Kind.String()
	case p.source != nil:
		return "chart · " + p.source.Kind.String()
	case len(p.templates) > 0:
		return strings.ToLower(p.templates[p.template])
	}
	return "chart"
}

// chartPaneView draws the pane: a title line, then the chart or what
// stands in its way.
func (m *Model) chartPaneView(w, h int) string {
	p := m.chartPane
	title := m.chartTitle()
	if p.doc == nil && p.source == nil && len(p.templates) > 1 {
		title += " · t next type"
	}
	body := p.view
	if pic := m.chartPicView(); pic != "" {
		title += " · graphics"
		body = pic
	}
	if body == "" && p.err != nil {
		body = ansi.Wordwrap(safe(chartError(p.err)), max(1, w), "")
		for _, n := range p.shape.Notes {
			body += "\n" + ansi.Wordwrap(safe(string(n)), max(1, w), "")
		}
	}
	lines := append([]string{ansi.Truncate(title, w, "…")}, strings.Split(body, "\n")...)
	for i := range lines {
		lines[i] = ansi.Truncate(lines[i], w, "")
	}
	return strings.Join(lines[:min(len(lines), h)], "\n")
}

// chartStatus is the pane's part of the status bar.
func (m *Model) chartStatus() string {
	p := m.chartPane
	if p == nil {
		return ""
	}
	out := m.chartTitle()
	switch {
	case p.source != nil:
		out = fmt.Sprintf("%s · %d rows", out, p.rows())
	case p.doc == nil && len(p.templates) > 0:
		out = fmt.Sprintf("chart · %s · %d series · %d rows", strings.ToLower(strings.TrimSuffix(p.templates[p.template], " Chart")), max(1, len(p.shape.Measures)), p.shape.Rows)
	}
	switch {
	case p.err != nil:
		out += " · ⚠ " + safe(chartError(p.err))
	case m.chartGraphics && p.imgErr != nil:
		out += " · ⚠ " + safe(p.imgErr.Error()) + ": showing text"
	case m.chartGraphics && p.img != nil:
		out += " · graphics"
	case len(p.warnings) > 0:
		out += " · ⚠ " + safe(p.warnings[0].Message)
	case len(p.bound) > 0:
		out += " · " + safe(string(p.bound[0]))
	case p.source == nil && len(p.shape.Notes) > 0:
		out += " · " + safe(string(p.shape.Notes[0]))
	}
	return out
}

// chartFields describes the chart for the info box.
func (m *Model) chartFields() []document.Field {
	p := m.chartPane
	if p == nil {
		return nil
	}
	fields := []document.Field{{Label: "Type", Value: m.chartTitle()}}
	if p.doc == nil {
		series := max(1, len(p.shape.Measures))
		if p.source != nil {
			series = p.series
		}
		fields = append(fields, document.Field{Label: "Series", Value: fmt.Sprint(series)}, document.Field{Label: "Rows", Value: fmt.Sprint(p.rows())})
	}
	switch {
	case p.err != nil:
		fields = append(fields, document.Field{Label: "Problem", Value: chartError(p.err)})
	case len(p.warnings) > 0:
		fields = append(fields, document.Field{Label: "Warning", Value: p.warnings[0].Message})
	}
	return document.Section("Chart", fields...)
}
