package app

import (
	"context"
	"errors"
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"nimbleterminal.dev/gloss/internal/chart"
	"nimbleterminal.dev/gloss/internal/document"
)

var errSentinel = errors.New("sentinel failure")

func salesSheet() *document.Sheet {
	s := &document.Sheet{Name: "sales", Columns: 5}
	s.Rows = [][]string{
		{"region", "quarter", "units", "revenue", "booked"},
		{"North", "Q1", "120", "48.5", "2026-03-31"},
		{"South", "Q1", "95", "39.9", "2026-03-31"},
		{"North", "Q2", "131", "52", "2026-06-30"},
		{"South", "Q2", "102", "41.7", "2026-06-30"},
	}
	return s
}

// settle runs the commands a key returned until the chart result lands.
func settle(t *testing.T, m *Model, cmd tea.Cmd) {
	t.Helper()
	for i := 0; cmd != nil && i < 8; i++ {
		msg := cmd()
		if msg == nil {
			return
		}
		if batch, ok := msg.(tea.BatchMsg); ok {
			for _, c := range batch {
				settle(t, m, c)
			}
			return
		}
		_, cmd = m.Update(msg)
	}
}

func TestChartPaneTogglesBesideTheGrid(t *testing.T) {
	m := tabulated(t, salesSheet(), 1, 1)
	m.Update(tea.WindowSizeMsg{Width: 140, Height: 30})
	_, cmd := m.Update(press("C"))
	if m.chartPane == nil || cmd == nil {
		t.Fatal("C did not open the pane")
	}
	settle(t, m, cmd)
	if m.chartPane.err != nil || m.chartPane.view == "" {
		t.Fatalf("no chart: %v", m.chartPane.err)
	}
	lines := plain(m)
	if !strings.Contains(lines[1], "region") || !strings.Contains(strings.Join(lines, "\n"), "chart · line") {
		t.Fatalf("grid or status missing:\n%s", strings.Join(lines, "\n"))
	}
	grid, pane := m.chartSplit()
	if grid != 70 || pane != 69 {
		t.Fatalf("split %d/%d", grid, pane)
	}
	_, _ = m.Update(press("C"))
	if m.chartPane != nil || m.chartWanted {
		t.Fatal("C did not close the pane")
	}
}

func TestChartPaneReplacesTheGridWhenNarrow(t *testing.T) {
	m := tabulated(t, salesSheet(), 1, 1)
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	_, cmd := m.Update(press("C"))
	settle(t, m, cmd)
	if grid, pane := m.chartSplit(); grid != 0 || pane != 80 {
		t.Fatalf("split %d/%d", grid, pane)
	}
	if body := strings.Join(plain(m), "\n"); strings.Contains(body, "North") && strings.Contains(body, " 1 ") {
		t.Fatal("grid still drawn under the chart")
	}
	// Grid keys still move the cursor underneath.
	m.Update(press("j"))
	if m.sheet.at[0] != 1 {
		t.Fatal("j did not move the hidden grid's cursor")
	}
}

func TestChartPaneDropsStaleResults(t *testing.T) {
	m := tabulated(t, salesSheet(), 1, 1)
	m.Update(tea.WindowSizeMsg{Width: 140, Height: 30})
	_, first := m.Update(press("C"))
	rev := m.chartPane.revision
	_, second := m.Update(tea.WindowSizeMsg{Width: 150, Height: 30})
	if m.chartPane.revision == rev {
		t.Fatal("resize did not start a new compile")
	}
	settle(t, m, first) // The old result arrives late.
	if m.chartPane.view != "" {
		t.Fatal("stale frame painted")
	}
	settle(t, m, second)
	if m.chartPane.view == "" {
		t.Fatal("current frame dropped")
	}
}

// A compile started for one sheet never lands on the pane of the next.
func TestChartPaneIgnoresResultsOfAReplacedPane(t *testing.T) {
	m := tabulated(t, salesSheet(), 1, 2)
	m.Update(tea.WindowSizeMsg{Width: 140, Height: 30})
	_, first := m.Update(press("C"))
	other := salesSheet()
	other.Rows = other.Rows[:3] // A different chart: 2 rows.
	m.generation++
	_, second := m.Update(document.Result{Generation: m.generation, Kind: "xlsx", Page: 2, Pages: 2, Sheet: other})
	settle(t, m, second)
	view := m.chartPane.view
	if view == "" || m.chartPane.shape.Rows != 2 {
		t.Fatalf("new pane not drawn: rows=%d", m.chartPane.shape.Rows)
	}
	settle(t, m, first) // The replaced pane's compile, cancelled, reports late.
	if m.chartPane.view != view || m.chartPane.err != nil {
		t.Fatalf("stale result accepted: err=%v", m.chartPane.err)
	}
}

func TestChartPaneKeepsTheLastChartOnError(t *testing.T) {
	m := tabulated(t, salesSheet(), 1, 1)
	m.Update(tea.WindowSizeMsg{Width: 140, Height: 30})
	_, cmd := m.Update(press("C"))
	settle(t, m, cmd)
	good := m.chartPane.view
	m.chartPane.revision++
	m.Update(chartResult{pane: m.chartPane, revision: m.chartPane.revision, err: errSentinel})
	if m.chartPane.view != good || !strings.Contains(m.chartStatus(), "sentinel") {
		t.Fatalf("view=%q status=%q", m.chartPane.view, m.chartStatus())
	}
}

func TestChartPaneFollowsHiddenColumns(t *testing.T) {
	m := tabulated(t, salesSheet(), 1, 1)
	m.Update(tea.WindowSizeMsg{Width: 140, Height: 30})
	_, cmd := m.Update(press("C"))
	settle(t, m, cmd)
	// Hide the date column (E): region becomes x and two measures stack.
	m.sheet.at = [2]int{0, 4}
	_, cmd = m.Update(press("x"))
	settle(t, m, cmd)
	if got := m.chartPane.templates[m.chartPane.template]; got != "Stacked Bar Chart" {
		t.Fatalf("template after hiding the date: %s", got)
	}
	// The column picker's changes count too.
	m.Update(press("c"))
	m.Update(press("space")) // Toggle the column under the picker's cursor.
	_, cmd = m.Update(press("esc"))
	settle(t, m, cmd)
	if equalInts(m.chartPane.cols, []int{0, 1, 2, 3}) {
		t.Fatal("picker change not followed")
	}
}

func TestChartPaneCyclesTemplates(t *testing.T) {
	m := tabulated(t, salesSheet(), 1, 1)
	m.Update(tea.WindowSizeMsg{Width: 140, Height: 30})
	_, cmd := m.Update(press("C"))
	settle(t, m, cmd)
	_, cmd = m.Update(press("t"))
	settle(t, m, cmd)
	if got := m.chartPane.templates[m.chartPane.template]; got != "Area Chart" || m.chartPane.view == "" {
		t.Fatalf("t: %s", got)
	}
	_, cmd = m.Update(press("t"))
	settle(t, m, cmd)
	if got := m.chartPane.templates[m.chartPane.template]; got != "Line Chart" {
		t.Fatalf("t wraps: %s", got)
	}
}

func TestChartPaneExplainsUnchartableSheet(t *testing.T) {
	m := tabulated(t, salesSheet(), 1, 1)
	m.Update(tea.WindowSizeMsg{Width: 140, Height: 30})
	_, cmd := m.Update(press("C"))
	settle(t, m, cmd)
	for _, col := range []int{4, 3, 2} {
		m.sheet.at = [2]int{0, m.sheet.place(col)}
		_, cmd = m.Update(press("x"))
		settle(t, m, cmd)
	}
	body := strings.Join(plain(m), "\n")
	if !strings.Contains(body, "nothing to chart") || !strings.Contains(m.chartStatus(), "nothing to chart") {
		t.Fatalf("no explanation:\n%s", body)
	}
}

func TestChartPaneStaysAcrossSheets(t *testing.T) {
	m := tabulated(t, salesSheet(), 1, 2)
	m.Update(tea.WindowSizeMsg{Width: 140, Height: 30})
	_, cmd := m.Update(press("C"))
	settle(t, m, cmd)
	other := salesSheet()
	other.Name = "other"
	m.generation++
	_, cmd = m.Update(document.Result{Generation: m.generation, Kind: "xlsx", Page: 2, Pages: 2, Sheet: other})
	settle(t, m, cmd)
	if m.chartPane == nil || m.chartPane.view == "" {
		t.Fatal("pane did not survive the sheet change")
	}
}

func TestChartDocumentIsAPage(t *testing.T) {
	m := viewing(t, Options{Files: []string{"sales.json"}}, "")
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	doc, err := chart.Sniff([]byte(`{"type":"bar","width":1,"height":1,"x_axis":{"type":"category","labels":["a","b"]},"data":{"series":[{"name":"s","values":[{"y":1},{"y":2}]}]}}`))
	if err != nil {
		t.Fatal(err)
	}
	_, cmd := m.Update(document.Result{Generation: m.generation, Kind: "chart", Page: 1, Pages: 1, Chart: &doc})
	settle(t, m, cmd)
	if m.chartPane == nil || m.chartPane.doc == nil || m.chartPane.view == "" {
		t.Fatalf("chart document not drawn: %+v", m.chartPane)
	}
	if grid, pane := m.chartSplit(); grid != 0 || pane != 80 {
		t.Fatalf("split %d/%d", grid, pane)
	}
	_, _ = m.Update(press("C"))
	if m.chartPane == nil {
		t.Fatal("C closed a chart document")
	}
	if !strings.Contains(strings.Join(plain(m), "\n"), "chart · spec") {
		t.Fatal("status does not name the document kind")
	}
}

func TestChartOptionBindsToTheSheet(t *testing.T) {
	doc, err := chart.Sniff([]byte(`{"data":{"values":[]},"chart_spec":{"chartType":"Bar Chart","encodings":{"x":{"field":"region"},"y":{"field":"revenue","aggregate":"sum"}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	m := viewing(t, Options{Files: []string{"book.xlsx"}, Chart: &doc}, "")
	m.Update(tea.WindowSizeMsg{Width: 140, Height: 30})
	_, cmd := m.Update(document.Result{Generation: m.generation, Kind: "xlsx", Page: 1, Pages: 1, Sheet: salesSheet()})
	settle(t, m, cmd)
	if m.chartPane == nil || m.chartPane.source == nil || m.chartPane.view == "" || m.chartPane.err != nil {
		t.Fatalf("pane: %+v", m.chartPane)
	}
	if !strings.Contains(m.chartStatus(), "chart · flint") {
		t.Fatalf("status %q", m.chartStatus())
	}
	// t does nothing with a document in charge.
	if cmd := m.nextChartTemplate(); cmd != nil {
		t.Fatal("t cycled a bound document")
	}
	if strings.Contains(m.hints(), "t chart type") {
		t.Fatal("hint offers t while a document is in charge")
	}
	// Hiding the revenue column makes the document fail, and says why.
	before := m.chartPane.revision
	m.sheet.at = [2]int{0, 3}
	_, cmd = m.Update(press("x"))
	settle(t, m, cmd)
	if m.chartPane.err == nil || !strings.Contains(m.chartPane.err.Error(), "revenue") {
		t.Fatalf("hidden field not reported: %v", m.chartPane.err)
	}
	// The compile that was cancelled by the change reports late: it must not
	// replace the reason with "context canceled".
	m.Update(chartResult{pane: m.chartPane, revision: before, err: context.Canceled})
	if !strings.Contains(m.chartPane.err.Error(), "revenue") {
		t.Fatalf("stale cancel overwrote the error: %v", m.chartPane.err)
	}
}

const flintPage = `{"data":{"values":[{"a":"x","b":1}]},"chart_spec":{"chartType":"Bar Chart","encodings":{"x":{"field":"a"},"y":{"field":"b"}}}}`

func TestChartReloadRereadsTheFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chart.json")
	first := `{"data":{"values":[]},"chart_spec":{"chartType":"Bar Chart","encodings":{"x":{"field":"region"},"y":{"field":"revenue"}}}}`
	if err := os.WriteFile(path, []byte(first), 0o600); err != nil {
		t.Fatal(err)
	}
	doc, _ := chart.Sniff([]byte(first))
	m := viewing(t, Options{Files: []string{samples + "sales.csv"}, Chart: &doc, ChartPath: path}, "")
	m.Update(tea.WindowSizeMsg{Width: 140, Height: 30})
	settle(t, m, m.load(false))
	if m.chartPane == nil || m.chartPane.view == "" {
		t.Fatalf("not drawn: %+v", m.chartPane)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(first, "Bar Chart", "Lollipop Chart", 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	_, cmd := m.Update(press("R"))
	settle(t, m, cmd)
	if !strings.Contains(string(m.opts.Chart.Raw), "Lollipop") {
		t.Fatal("R did not re-read --chart")
	}
}

func TestExportSavesTheChartPane(t *testing.T) {
	var out saved
	m := tabulated(t, salesSheet(), 1, 1)
	m.opts.Save = out.save(t)
	m.Update(tea.WindowSizeMsg{Width: 140, Height: 30})
	_, cmd := m.Update(press("C"))
	settle(t, m, cmd)
	_, cmd = m.Update(press("e"))
	if cmd == nil || m.note == "spreadsheets cannot be exported as PNG" {
		t.Fatalf("e refused: %q", m.note)
	}
	m.Update(cmd())
	if len(out.names) != 1 || out.names[0] != "book-chart.png" {
		t.Fatalf("names=%q", out.names)
	}
	if b := out.images[0].Bounds(); b.Dx() != 69*document.ChartCellWidth {
		t.Fatalf("exported %v, want the pane's 69 columns", b)
	}
}

func TestChartPushAppliesAndReplies(t *testing.T) {
	pushes := make(chan ChartPush, 1)
	m := viewing(t, Options{Files: []string{"book.xlsx"}, Charts: pushes}, "")
	m.Update(tea.WindowSizeMsg{Width: 140, Height: 30})
	m.Update(document.Result{Generation: m.generation, Kind: "xlsx", Page: 1, Pages: 1, Sheet: salesSheet()})
	doc, _ := chart.Sniff([]byte(`{"data":{"values":[]},"chart_spec":{"chartType":"Bar Chart","encodings":{"x":{"field":"region"},"y":{"field":"revenue","aggregate":"sum"}}}}`))
	reply := make(chan ChartReply, 1)
	pushes <- ChartPush{Doc: doc, Reply: reply}
	// Take the push as Update would, without re-arming the wait (settle
	// would run it and block on the empty channel).
	push := m.awaitCharts()().(chartPushMsg)
	settle(t, m, m.pushed(push))
	select {
	case got := <-reply:
		if got.Type != "bar" || got.Series != 1 || got.Rows != 4 || got.Error != "" || got.Warnings == nil {
			t.Fatalf("%+v", got)
		}
	default:
		t.Fatal("no reply")
	}
	if m.chartPane == nil || m.chartPane.source == nil || m.chartPane.view == "" {
		t.Fatal("pane not drawn from the push")
	}
	// A push over a page that is not a table is refused, not reported drawn.
	m.Update(document.Result{Generation: m.generation, Kind: "png", Page: 1, Pages: 1, Image: image.NewRGBA(image.Rect(0, 0, 4, 4))})
	bound, _ := chart.Sniff([]byte(flintPage))
	reply = make(chan ChartReply, 1)
	pushes <- ChartPush{Doc: bound, Reply: reply}
	push = m.awaitCharts()().(chartPushMsg)
	settle(t, m, m.pushed(push))
	select {
	case got := <-reply:
		if !strings.Contains(got.Error, "no table") || m.chartPane != nil {
			t.Fatalf("pushed over a picture: %+v pane=%v", got, m.chartPane != nil)
		}
	default:
		t.Fatal("no reply over a picture")
	}
	m.Update(document.Result{Generation: m.generation, Kind: "xlsx", Page: 1, Pages: 1, Sheet: salesSheet()})
	// A push that cannot be drawn answers at once, with the reason.
	unbound, _ := chart.Sniff([]byte(`{"data":{"values":[]},"chart_spec":{"chartType":"Bar Chart","encodings":{"x":{"field":"planet"},"y":{"field":"revenue"}}}}`))
	reply = make(chan ChartReply, 1)
	pushes <- ChartPush{Doc: unbound, Reply: reply}
	push = m.awaitCharts()().(chartPushMsg)
	settle(t, m, m.pushed(push))
	select {
	case got := <-reply:
		if !strings.Contains(got.Error, "planet") {
			t.Fatalf("%+v", got)
		}
	default:
		t.Fatal("no reply for a refused push")
	}
}

func TestChartFromTheActionsMenu(t *testing.T) {
	m := tabulated(t, salesSheet(), 1, 1)
	m.opts.OpenWith, m.opts.Scratch = &fakeOpenHost{}, func(string, []byte) (string, error) { return "", nil }
	m.Update(tea.WindowSizeMsg{Width: 140, Height: 30})
	entry := func(label string) menuEntry {
		for _, e := range m.cellEntries() {
			if e.label == label {
				return e
			}
		}
		t.Fatalf("no %q among %v", label, labels(m.cellEntries()))
		return menuEntry{}
	}
	settle(t, m, entry("Chart these columns").do(m))
	if m.chartPane == nil || m.chartPane.view == "" {
		t.Fatal("the menu drew no chart")
	}
	entry("Open chart in browser")
	settle(t, m, entry("Hide the chart").do(m))
	if m.chartPane != nil {
		t.Fatal("the chart stayed")
	}
}
