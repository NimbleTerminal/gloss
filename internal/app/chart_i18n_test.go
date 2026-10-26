package app

import (
	"image"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"nimbleterminal.dev/gloss/internal/chart"
	"nimbleterminal.dev/gloss/internal/document"
)

// The chart pane speaks the language in use: its title, its status, its
// notes and its menu entries. What internal/chart says stays as it is, and
// so does what a push is told, which a program reads.
func TestChartSpeaksTheLanguage(t *testing.T) {
	for _, c := range []struct {
		tag                         string
		status, title, info         string
		graphics, text, noChart     string
		nothing, small, noTable     string
		chart, hide, browser, hints string
	}{
		{"es", "gráfico · líneas · 2 series · 4 filas · ", "gráfico de líneas · t tipo siguiente", "Gráfico",
			"gráfico como imagen: g alterna Kitty / glifos", "gráfico como texto", "no se muestra ningún gráfico: pulse C en una tabla o abra un archivo de gráfico",
			"nada que graficar", "la terminal es demasiado pequeña para un gráfico", "no hay ninguna tabla abierta para graficar",
			"Graficar estas columnas", "Ocultar el gráfico", "Abrir gráfico en el navegador", "C gráfico"},
		{"zh-Hans", "图表 · 折线图 · 2 个系列 · 4 行 · ", "折线图 · t 下一类型", "图表",
			"图表图形：g 切换 Kitty / 字符", "文本图表", "未显示图表：请在表格上按 C，或打开图表文件",
			"没有可绘制的数据", "终端太小，无法显示图表", "没有打开可绘制图表的表格",
			"为这些列绘制图表", "隐藏图表", "在浏览器中打开图表", "C 图表"},
	} {
		t.Run(c.tag, func(t *testing.T) {
			speaking(t, c.tag)
			m := tabulated(t, salesSheet(), 1, 1)
			m.opts.OpenWith, m.opts.Scratch = &fakeOpenHost{}, func(string, []byte) (string, error) { return "", nil }
			m.Update(tea.WindowSizeMsg{Width: 140, Height: 30})

			// The menu offers the chart before there is one.
			if got := labels(m.cellEntries()); !strings.Contains(strings.Join(got, "\n"), c.chart) {
				t.Errorf("entries %q, want %q", got, c.chart)
			}
			// No chart yet: ctrl+o says how to get one.
			m.Update(ctrlO())
			if m.note != c.noChart {
				t.Errorf("no chart note = %q, want %q", m.note, c.noChart)
			}
			_, cmd := m.Update(press("C"))
			settle(t, m, cmd)
			if p := m.chartPane; p == nil || p.view == "" {
				t.Fatal("no chart")
			}
			// The status names the chart; the compiler's note after it is data.
			if s := m.chartStatus(); !strings.HasPrefix(s, c.status) || !strings.HasSuffix(s, "quarter skipped: two category columns at most") {
				t.Errorf("status = %q, want it to start %q", s, c.status)
			}
			lines := plain(m)
			for _, line := range lines {
				if w := ansi.StringWidth(line); w > 140 {
					t.Errorf("line of %d cells: %q", w, line)
				}
			}
			if hint := lines[len(lines)-1]; !strings.Contains(hint, c.hints) {
				t.Errorf("hints %q lack %q", hint, c.hints)
			}
			if title := strings.Split(m.chartPaneView(69, 3), "\n")[0]; title != c.title {
				t.Errorf("pane title = %q, want %q", title, c.title)
			}
			var section bool
			for _, f := range m.chartFields() {
				if f.Label == "Chart" && infoLabel(f.Label) == c.info {
					section = true
				}
			}
			if !section {
				t.Errorf("info has no %q section: %+v", c.info, m.chartFields())
			}
			got := strings.Join(labels(m.cellEntries()), "\n")
			if !strings.Contains(got, c.hide) || !strings.Contains(got, c.browser) {
				t.Errorf("entries %q, want %q and %q", got, c.hide, c.browser)
			}

			// ctrl+g and back.
			_, cmd = m.Update(ctrlG())
			if m.note != c.graphics {
				t.Errorf("graphics note = %q, want %q", m.note, c.graphics)
			}
			drainAnimation(t, m, cmd)
			_, cmd = m.Update(ctrlG())
			if m.note != c.text {
				t.Errorf("text note = %q, want %q", m.note, c.text)
			}
			drainAnimation(t, m, cmd)

			// Hide every column the chart could use: the pane says why.
			for _, col := range []int{4, 3, 2} {
				m.sheet.at = [2]int{0, m.sheet.place(col)}
				_, cmd = m.Update(press("x"))
				settle(t, m, cmd)
			}
			if !strings.Contains(m.chartPaneView(69, 10), c.nothing) || !strings.Contains(m.chartStatus(), c.nothing) {
				t.Errorf("pane %q / status %q lack %q", m.chartPaneView(69, 10), m.chartStatus(), c.nothing)
			}
			m.Update(tea.WindowSizeMsg{Width: 140, Height: 4})
			if !strings.Contains(m.chartStatus(), c.small) {
				t.Errorf("status %q lacks %q", m.chartStatus(), c.small)
			}
		})
	}
}

// What a push is told stays English whatever the viewer speaks: a program
// reads it; the note on screen is translated.
func TestChartPushReplyStaysEnglish(t *testing.T) {
	speaking(t, "es")
	pushes := make(chan ChartPush, 1)
	m := viewing(t, Options{Files: []string{"a.png"}, Charts: pushes}, "")
	m.Update(document.Result{Generation: m.generation, Kind: "png", Page: 1, Pages: 1, Image: image.NewRGBA(image.Rect(0, 0, 4, 4))})
	bound, _ := chart.Sniff([]byte(flintPage))
	reply := make(chan ChartReply, 1)
	pushes <- ChartPush{Doc: bound, Reply: reply}
	settle(t, m, m.pushed(m.awaitCharts()().(chartPushMsg)))
	got := <-reply
	if got.Error != "no table is open to chart" {
		t.Errorf("reply %q", got.Error)
	}
	if m.note != "no hay ninguna tabla abierta para graficar" {
		t.Errorf("note %q", m.note)
	}
}
