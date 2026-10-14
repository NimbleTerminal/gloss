package app

import (
	"strings"
	"testing"
	"testing/fstest"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/NimbleMarkets/ntcharts/v2/picture"
	"nimbleterminal.dev/gloss/examples"
)

// kittyTypesetModel shows a typeset page under Kitty graphics at cols×rows,
// with its first picture presented, as the viewer is between keystrokes.
func kittyTypesetModel(tb testing.TB, cols, rows int) *Model {
	tb.Helper()
	tb.Cleanup(func() { picture.ForceKittyCapability(picture.KittyCapabilityUnknown) })
	files := fstest.MapFS{"page.md": &fstest.MapFile{Data: []byte("# A typeset page\n\n" + strings.Repeat("Café and a paragraph.\n\n", 100))}}
	m := New(Options{Files: []string{"page.md"}, FilesFS: files, MarkdownMode: "typeset", Render: "kitty", DPI: 150, Page: 1})
	tb.Cleanup(func() { _ = m.Close() })
	m.Update(tea.WindowSizeMsg{Width: cols, Height: rows})
	drainAnimation(tb, m, m.load(false))
	if m.typeset == nil || m.pic.Mode() != picture.PictureKitty || !m.hasPictureFront() {
		tb.Fatal("no presented Kitty typeset picture")
	}
	return m
}

func kittyMeshModel(tb testing.TB, cols, rows int) *Model {
	tb.Helper()
	tb.Cleanup(func() { picture.ForceKittyCapability(picture.KittyCapabilityUnknown) })
	m := New(Options{Files: []string{"gloss.stl"}, FilesFS: examples.Files, Render: "kitty", Render3D: "software", Page: 1})
	tb.Cleanup(func() { _ = m.Close() })
	m.Update(tea.WindowSizeMsg{Width: cols, Height: rows})
	drainAnimation(tb, m, m.load(false))
	if m.chart == nil || !m.meshKitty() {
		tb.Fatal("no Kitty mesh")
	}
	return m
}

func BenchmarkMeshView(b *testing.B) {
	m := kittyMeshModel(b, 200, 52)
	b.ReportAllocs()
	for b.Loop() {
		m.View()
	}
}

func TestMeshBodyIsUnframed(t *testing.T) {
	m := kittyMeshModel(t, 200, 52)
	for _, size := range [][2]int{{200, 52}, {80, 24}, {40, 12}} {
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		w, h := m.width, m.bodyHeight()
		grid := m.meshFrame(m.chart.View().Content, w)
		body, _ := m.body(w, h)
		if body != grid || lipgloss.NewStyle().Width(w).Height(h).Render(grid) != grid {
			t.Fatalf("mesh body at %dx%d needs framing", w, h)
		}
	}
}

// The picture grid is already the body's exact size; drawing a frame must not
// re-measure its cols×rows placeholder graphemes on every View.
func BenchmarkPictureView(b *testing.B) {
	m := kittyTypesetModel(b, 200, 52)
	b.ReportAllocs()
	for b.Loop() {
		m.View()
	}
}

// A presented picture is returned as the body without a frame around it: the
// framed and unframed strings are identical, and the frame's measuring is gone.
func TestPictureBodyIsUnframed(t *testing.T) {
	m := kittyTypesetModel(t, 200, 52)
	w, h := m.width, m.bodyHeight()
	grid := m.pictureView()
	body, mouse := m.body(w, h)
	if body != grid || mouse != tea.MouseModeCellMotion {
		t.Fatal("picture body differs from the presented grid")
	}
	if framed := lipgloss.NewStyle().Width(w).Height(h).Render(grid); framed != grid {
		t.Fatal("the frame would have changed the grid, so skipping it changes the view")
	}
	if allocs := testing.AllocsPerRun(20, func() { m.body(w, h) }); allocs > 100 {
		t.Fatalf("picture body still measures the grid: %.0f allocations", allocs)
	}
}

// During a resize burst the page's grid is stale until the new one arrives;
// showing it cut to the new size must stay as cheap as showing an exact one.
func BenchmarkPictureViewStale(b *testing.B) {
	m := kittyTypesetModel(b, 200, 52)
	m.Update(tea.WindowSizeMsg{Width: 180, Height: 48}) // Reflow queued, not drained: the grid is the old 200×52 one.
	if !m.hasPictureFront() || m.still.pic.Status().Presented == m.still.pic.Status().Generation {
		b.Fatal("front grid was replaced")
	}
	b.ReportAllocs()
	for b.Loop() {
		m.View()
	}
}
