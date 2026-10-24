package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"nimbleterminal.dev/gloss/internal/chart"
	"nimbleterminal.dev/gloss/internal/document"
)

func ctrlO() tea.KeyPressMsg { return tea.KeyPressMsg{Code: 'o', Mod: tea.ModCtrl} }

func TestOpenChartPageWritesAndLaunches(t *testing.T) {
	host := &fakeOpenHost{}
	dir := t.TempDir()
	scratch := func(name string, data []byte) (string, error) { return document.WriteNew(filepath.Join(dir, name), data) }
	m := viewing(t, Options{Files: []string{"book.xlsx"}, OpenWith: host, Scratch: scratch, IsTemporary: func(string) bool { return true }}, "")
	m.Update(tea.WindowSizeMsg{Width: 140, Height: 30})
	_, cmd := m.Update(document.Result{Generation: m.generation, Kind: "xlsx", Page: 1, Pages: 1, Sheet: salesSheet()})
	settle(t, m, cmd)
	_, cmd = m.Update(press("C"))
	settle(t, m, cmd)
	if m.chartPane == nil || m.chartPane.view == "" {
		t.Fatalf("no chart: %+v", m.chartPane)
	}
	_, cmd = m.Update(ctrlO())
	settle(t, m, cmd)
	if len(host.launched) != 1 || host.launched[0].Kind != "html" || !host.launched[0].Temporary {
		t.Fatalf("launched %+v %v", host.launched, host.ids)
	}
	page, err := os.ReadFile(filepath.Join(dir, "book-chart.html"))
	if err != nil || !strings.Contains(string(page), "echarts.init") {
		t.Fatalf("page: %v", err)
	}
	if !strings.Contains(m.note, "Editor") {
		t.Fatalf("note %q", m.note)
	}
	// A second opening numbers the file instead of overwriting it.
	_, cmd = m.Update(ctrlO())
	settle(t, m, cmd)
	if _, err := os.Stat(filepath.Join(dir, "book-chart-2.html")); err != nil {
		t.Fatal(err)
	}
}

func TestOpenChartPageNeedsAHost(t *testing.T) {
	m := tabulated(t, salesSheet(), 1, 1)
	m.Update(tea.WindowSizeMsg{Width: 140, Height: 30})
	_, cmd := m.Update(press("C"))
	settle(t, m, cmd)
	if _, cmd := m.Update(ctrlO()); cmd != nil || !strings.Contains(m.note, "not available") {
		t.Fatalf("note %q", m.note)
	}
	m.opts.ChartURL = "http://127.0.0.1:1/t/chart"
	if _, cmd := m.Update(ctrlO()); cmd != nil || !strings.Contains(m.note, m.opts.ChartURL) {
		t.Fatalf("note %q", m.note)
	}
}

func TestOpenChartPageNeedsAChart(t *testing.T) {
	m := tabulated(t, salesSheet(), 1, 1)
	m.opts.OpenWith = &fakeOpenHost{}
	if _, cmd := m.Update(ctrlO()); cmd != nil || !strings.Contains(m.note, "press C") {
		t.Fatalf("note %q", m.note)
	}
}

func TestOpenChartPageReportsTerminalOnlySpecs(t *testing.T) {
	doc, err := chart.Sniff([]byte(`{"type":"heatmap","width":4,"height":2,"heat":{"matrix":[[1,2],[3,4]]}}`))
	if err != nil {
		t.Fatal(err)
	}
	host := &fakeOpenHost{}
	dir := t.TempDir()
	m := viewing(t, Options{Files: []string{"heat.json"}, OpenWith: host, Scratch: func(name string, data []byte) (string, error) { return document.WriteNew(filepath.Join(dir, name), data) }}, "")
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	_, cmd := m.Update(document.Result{Generation: m.generation, Kind: "chart", Page: 1, Pages: 1, Chart: &doc})
	settle(t, m, cmd)
	if m.chartPane == nil || m.chartPane.view == "" {
		t.Fatalf("no chart: %+v", m.chartPane)
	}
	_, cmd = m.Update(ctrlO())
	settle(t, m, cmd)
	if len(host.launched) != 0 || !strings.Contains(m.note, "heatmap") {
		t.Fatalf("launched %v, note %q", host.launched, m.note)
	}
}

func TestChartedFollowsThePane(t *testing.T) {
	var seen []*chart.Document
	m := tabulated(t, salesSheet(), 1, 1)
	m.opts.Charted = func(d *chart.Document) { seen = append(seen, d) }
	m.Update(tea.WindowSizeMsg{Width: 140, Height: 30})
	_, cmd := m.Update(press("C"))
	settle(t, m, cmd)
	if len(seen) != 1 || seen[0] == nil || seen[0].Kind != chart.KindFlint {
		t.Fatalf("seen %v", seen)
	}
	// Hiding every number leaves nothing to chart: the record says so too.
	for _, col := range []int{4, 3, 2} {
		m.sheet.at = [2]int{0, col}
		_, cmd = m.Update(press("x"))
		settle(t, m, cmd)
	}
	if m.chartPane == nil || m.chartPane.view != "" || len(seen) == 0 || seen[len(seen)-1] != nil {
		t.Fatalf("pane %+v, seen %v", m.chartPane, seen)
	}
	// Showing them again draws, and the record follows.
	_, cmd = m.Update(press("X"))
	settle(t, m, cmd)
	if m.chartPane.view == "" || seen[len(seen)-1] == nil {
		t.Fatalf("pane %+v, seen %v", m.chartPane, seen)
	}
	_, cmd = m.Update(press("C"))
	settle(t, m, cmd)
	if seen[len(seen)-1] != nil {
		t.Fatalf("seen %v", seen)
	}
}

// A preview is not the viewer: what it draws is nobody's chart to serve,
// and a push is not its to take.
func TestPreviewReportsNoCharts(t *testing.T) {
	charts := make(chan ChartPush)
	m := New(Options{Files: []string{"a.png", "b.csv"}, Menu: true, Preview: true, Charted: func(*chart.Document) {}, Charts: charts})
	t.Cleanup(func() { m.Close() })
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	if m.preview == nil {
		t.Fatal("no preview")
	}
	if m.preview.opts.Charted != nil || m.preview.opts.Charts != nil {
		t.Fatal("the preview inherited the host's chart hooks")
	}
}
