package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"nimbleterminal.dev/gloss/internal/i18n"
)

// speaking sets the process-wide language for one test, and puts English
// back after it.
func speaking(t *testing.T, tag string) {
	t.Helper()
	t.Cleanup(func() { i18n.SetLanguage("en") })
	i18n.SetLanguage(tag)
}

// fits fails when a line of view is wider than w cells.
func fits(t *testing.T, view string, w int) {
	t.Helper()
	for _, line := range strings.Split(view, "\n") {
		if got := ansi.StringWidth(line); got > w {
			t.Fatalf("line is %d cells, wider than %d: %q", got, w, line)
		}
	}
}

func TestLandingPageSpeaksTheLanguage(t *testing.T) {
	for _, c := range []struct {
		tag  string
		want []string
	}{
		{"es", []string{"Suelte archivos aquí para abrirlos", "pulse o para explorar", "Imágenes:", "Documentos:", "Texto:", "Mapas:", "texto plano y código fuente", "mallas STL, 3MF, OBJ y PLY", "GeoJSON, GPX y PMTiles ráster"}},
		{"zh-Hans", []string{"将文件拖到此处打开", "按 o 浏览", "图片：", "文档：", "文本：", "地图：", "纯文本和源代码", "STL、3MF、OBJ 和 PLY 网格", "GeoJSON、GPX 和栅格 PMTiles"}},
	} {
		t.Run(c.tag, func(t *testing.T) {
			speaking(t, c.tag)
			m := New(Options{Render: "glyph", Page: 1, DPI: 72})
			t.Cleanup(func() { m.Close() })
			m.Init()
			m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
			view := ansi.Strip(m.View().Content)
			for _, want := range c.want {
				if !strings.Contains(view, want) {
					t.Errorf("landing page lacks %q:\n%s", want, view)
				}
			}
			fits(t, view, 80)
			// The labels still line up.
			starts := map[int]bool{}
			for _, line := range strings.Split(view, "\n") {
				for _, label := range c.want[2:6] {
					if i := strings.Index(line, label); i >= 0 {
						starts[ansi.StringWidth(line[:i])] = true
					}
				}
			}
			if len(starts) != 1 {
				t.Errorf("labels do not line up:\n%s", view)
			}
			// A short screen has the short list, and a tiny one the words.
			m.Update(tea.WindowSizeMsg{Width: 80, Height: 14})
			view = ansi.Strip(m.View().Content)
			if strings.Contains(view, c.want[2]) {
				t.Errorf("short screen has the long list:\n%s", view)
			}
			for _, item := range shortFormats() {
				// Wrapped between items, never through one.
				if !strings.Contains(view, item) {
					t.Errorf("short screen lacks %q:\n%s", item, view)
				}
			}
			fits(t, view, 80)
			m.Update(tea.WindowSizeMsg{Width: 80, Height: 3})
			if view := ansi.Strip(m.View().Content); !strings.Contains(view, "GLOSS") || strings.Contains(view, "drag here") {
				t.Errorf("tiny screen:\n%s", view)
			}
		})
	}
}

func TestLandingPageEnglishFormats(t *testing.T) {
	want := []string{
		"Images: PNG, JPEG, GIF, WebP, BMP, TIFF, HEIC, JPEG XL",
		"Doc:    SVG, Mermaid, PDF, Word, Excel, Grist, Jupyter",
		"Text:   Markdown, HTML, JSON, CSV, plain text and source",
		"3D:     STL, 3MF, OBJ and PLY meshes",
		"Maps:   GeoJSON, GPX, and raster PMTiles",
	}
	if got := formatLines(); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("formats:\n%s", strings.Join(got, "\n"))
	}
	if got := formatsShort(); got != "Images, SVG, Mermaid, PDF, Word, Excel, Grist, Jupyter, Markdown, HTML, text, JSON, CSV, 3D, maps" {
		t.Fatalf("short: %q", got)
	}
	// Wrapped between items, the short line is what a word wrap made of it.
	for w := 10; w <= 120; w++ {
		style := lipgloss.NewStyle().Width(w).Align(lipgloss.Center)
		if got, want := style.Render(wrapList(shortFormats(), ", ", w)), style.Render(formatsShort()); got != want {
			t.Fatalf("%d columns:\n%s\nwant\n%s", w, got, want)
		}
	}
}
