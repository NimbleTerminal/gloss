package app

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"nimbleterminal.dev/gloss/internal/chart"
	"nimbleterminal.dev/gloss/internal/document"
)

func ctrlG() tea.KeyPressMsg { return tea.KeyPressMsg{Code: 'g', Mod: tea.ModCtrl} }

// graphed is the sales sheet charted beside the grid, then switched to
// graphics with ctrl+g.
func graphed(t *testing.T, cols ...int) *Model {
	t.Helper()
	m := tabulated(t, salesSheet(), 1, 1)
	m.Update(tea.WindowSizeMsg{Width: 140, Height: 30})
	if len(cols) > 0 {
		m.sheet.cols = cols
	}
	_, cmd := m.Update(press("C"))
	settle(t, m, cmd)
	if m.chartPane == nil || m.chartPane.view == "" {
		t.Fatalf("no chart: %+v", m.chartPane)
	}
	_, cmd = m.Update(ctrlG())
	drainAnimation(t, m, cmd)
	return m
}

func TestChartGraphicsToggle(t *testing.T) {
	m := graphed(t)
	p := m.chartPane
	if !m.chartGraphics || p.img == nil || p.imgErr != nil {
		t.Fatalf("graphics=%v img=%v err=%v", m.chartGraphics, p.img != nil, p.imgErr)
	}
	if m.chartPic == nil {
		t.Fatal("no picture for the chart")
	}
	// Without Kitty the picture is drawn in half-blocks, beside the grid.
	view := m.View().Content
	if !strings.ContainsAny(view, "▀▄") {
		t.Fatalf("no half-block picture:\n%s", view)
	}
	if lines := plain(m); !strings.Contains(lines[1], "region") || !strings.Contains(strings.Join(lines, "\n"), "graphics") {
		t.Fatalf("grid or status missing:\n%s", strings.Join(lines, "\n"))
	}
	// Another template stays in graphics.
	img, template := p.img, p.template
	_, cmd := m.Update(press("t"))
	drainAnimation(t, m, cmd)
	if m.chartPane.img == nil || m.chartPane.img == img || m.chartPane.template == template {
		t.Fatal("the next template was not drawn as graphics")
	}
	_, cmd = m.Update(ctrlG())
	drainAnimation(t, m, cmd)
	if m.chartGraphics || m.chartPic != nil {
		t.Fatalf("ctrl+g did not return to text: graphics=%v picture=%v", m.chartGraphics, m.chartPic != nil)
	}
	if strings.ContainsAny(m.chartPaneView(69, 29), "▀▄") {
		t.Fatal("text chart still shows the picture")
	}
	// The choice outlives the pane.
	_, _ = m.Update(ctrlG())
	_, _ = m.Update(press("C"))
	_, cmd = m.Update(press("C"))
	drainAnimation(t, m, cmd)
	if !m.chartGraphics || m.chartPane.img == nil {
		t.Fatal("a new pane forgot graphics")
	}
}

// What the renderer cannot draw stays a text chart, and says why.
func TestChartGraphicsFallsBackToText(t *testing.T) {
	m := graphed(t, 0, 1, 2) // region, quarter, units: a heatmap.
	p := m.chartPane
	if !errors.Is(p.imgErr, chart.ErrBlank) || p.img != nil {
		t.Fatalf("img=%v err=%v", p.img != nil, p.imgErr)
	}
	if m.chartPic != nil {
		t.Fatal("a picture for a chart with none")
	}
	if p.view == "" || !strings.Contains(m.chartPaneView(69, 29), "North") {
		t.Fatalf("text chart missing:\n%s", m.chartPaneView(69, 29))
	}
	if s := m.chartStatus(); !strings.Contains(s, "cannot be drawn as graphics") {
		t.Fatalf("status %q", s)
	}
}

func TestExportSavesTheChartGraphics(t *testing.T) {
	var out saved
	m := graphed(t)
	m.opts.Save = out.save(t)
	_, cmd := m.Update(press("e"))
	if cmd == nil {
		t.Fatalf("e refused: %q", m.note)
	}
	m.Update(cmd())
	if len(out.names) != 1 || out.names[0] != "book-chart.png" {
		t.Fatalf("names=%q", out.names)
	}
	// Drawn again at the export cell, as a text chart is, not the screen's.
	if b := out.images[0].Bounds(); b.Dx() != 69*document.ChartCellWidth || b.Dy() != m.chartPane.height*document.ChartCellHeight {
		t.Fatalf("exported %v, want the pane's 69×%d export cells", b, m.chartPane.height)
	}
	if b := m.chartPane.img.Bounds(); b.Dx() == 69*document.ChartCellWidth {
		t.Fatalf("the screen picture %v is already export size: the test proves nothing", b)
	}
}

// A chart only graphics can draw, such as a pie, is drawn when they are on,
// and while they are off the error says how to get them.
func TestChartGraphicsDrawWhatTextCannot(t *testing.T) {
	doc, err := chart.Sniff([]byte(`{"data":{"values":[]},"chart_spec":{"chartType":"Pie Chart","encodings":{"size":{"field":"revenue","aggregate":"sum"},"color":{"field":"region"}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	m := tabulated(t, salesSheet(), 1, 1)
	m.Update(tea.WindowSizeMsg{Width: 140, Height: 30})
	settle(t, m, m.applyChart(&doc))
	if p := m.chartPane; p.err == nil || p.shown() {
		t.Fatalf("text drew a pie: %+v", p)
	}
	if body := m.chartPaneView(69, 29); !strings.Contains(body, "ctrl+g") || strings.Contains(body, `"renderer"`) {
		t.Fatalf("the error does not point at ctrl+g:\n%s", body)
	}
	_, cmd := m.Update(ctrlG())
	drainAnimation(t, m, cmd)
	p := m.chartPane
	if p.err != nil || p.img == nil || !p.shown() || m.chartPic == nil {
		t.Fatalf("pie not drawn: err=%v img=%v", p.err, p.img != nil)
	}
	if !strings.ContainsAny(m.chartPaneView(69, 29), "▀▄") {
		t.Fatal("no picture in the pane")
	}
	if strings.Contains(m.chartStatus(), "⚠") {
		t.Fatalf("status %q", m.chartStatus())
	}
	var out saved
	m.opts.Save = out.save(t)
	_, cmd = m.Update(press("e"))
	if cmd == nil {
		t.Fatalf("e refused: %q", m.note)
	}
	m.Update(cmd())
	if len(out.images) != 1 {
		t.Fatalf("not exported: %q", m.note)
	}
}
