package app

import (
	"context"
	"strings"

	tea "charm.land/bubbletea/v2"
	"nimbleterminal.dev/gloss/internal/chart"
	"nimbleterminal.dev/gloss/internal/openwith"
)

// chartPageSaved is the answer of the command that writes the chart as a
// page of its own, for the browser.
type chartPageSaved struct {
	generation uint64
	path       string
	err        error
}

// openChartPage writes the chart as a page of its own in the scratch folder
// and opens it as any HTML file opens; without an opener it names the served
// page instead.
func (m *Model) openChartPage() tea.Cmd {
	p := m.chartPane
	if p == nil || p.compiled == nil || p.view == "" {
		m.note = "no chart is shown: press C on a table, or open a chart file"
		return nil
	}
	if m.opts.OpenWith == nil || m.opts.Scratch == nil || m.isPreview {
		if m.opts.ChartURL != "" {
			m.note = "chart page: " + m.opts.ChartURL
		} else {
			m.note = "Opening in a browser is not available in this host"
		}
		return nil
	}
	q := m.exportRequest()
	name := strings.TrimSuffix(exportName(q.Path, m.kind, m.page, q.Stdin), ".png")
	if p.doc == nil {
		name += "-chart"
	}
	name += ".html"
	title, doc, scratch, generation := m.chartTitle(), *p.compiled, m.opts.Scratch, m.generation
	m.note = "opening…"
	return func() tea.Msg {
		option, err := chart.Default.Option(context.Background(), doc)
		if err != nil {
			return chartPageSaved{generation: generation, err: err}
		}
		page, _ := chart.Page(title, option)
		path, err := scratch(name, page)
		return chartPageSaved{generation: generation, path: path, err: err}
	}
}

// chartPageWritten opens the page written, in the app that opens HTML.
func (m *Model) chartPageWritten(v chartPageSaved) tea.Cmd {
	if v.generation != m.generation {
		return nil
	}
	if v.err != nil {
		m.note = "chart page: " + safe(v.err.Error())
		return nil
	}
	return m.requestOpenWith(v.path, "html", false, false, openwith.Open)
}
