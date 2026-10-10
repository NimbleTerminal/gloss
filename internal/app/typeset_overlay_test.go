package app

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"strings"
	"testing"
	"testing/fstest"

	tea "charm.land/bubbletea/v2"
	"github.com/NimbleMarkets/ntcharts/v2/picture"
	"github.com/NimbleTerminal/gloss/internal/animate"
	"github.com/charmbracelet/x/ansi/kitty"
)

func gifPage(t testing.TB) fstest.MapFS {
	t.Helper()
	palette := color.Palette{color.RGBA{255, 0, 0, 255}, color.RGBA{0, 255, 0, 255}}
	first, second := image.NewPaletted(image.Rect(0, 0, 40, 20), palette), image.NewPaletted(image.Rect(0, 0, 40, 20), palette)
	for i := range second.Pix {
		second.Pix[i] = 1
	}
	var b bytes.Buffer
	if err := gif.EncodeAll(&b, &gif.GIF{Image: []*image.Paletted{first, second}, Delay: []int{10, 10}}); err != nil {
		t.Fatal(err)
	}
	return fstest.MapFS{
		"page.md": &fstest.MapFile{Data: []byte("# A page with motion\n\n![motion](a.gif)\n\n" + strings.Repeat("Café and a paragraph.\n\n", 60))},
		"a.gif":   &fstest.MapFile{Data: b.Bytes()},
	}
}

func typesetGIFModel(t testing.TB, render string, cols, rows int) *Model {
	t.Helper()
	t.Cleanup(func() { picture.ForceKittyCapability(picture.KittyCapabilityUnknown) })
	m := New(Options{Files: []string{"page.md"}, FilesFS: gifPage(t), MarkdownMode: "typeset", Render: render, DPI: 150, Page: 1})
	t.Cleanup(func() { _ = m.Close() })
	m.Update(tea.WindowSizeMsg{Width: cols, Height: rows})
	drainStill(t, m, m.load(false))
	if m.typeset == nil || m.typesetPlayback == nil || len(m.typesetPlayback.Regions) != 1 {
		t.Fatal("no typeset page with one animated region")
	}
	return m
}

// drainStill runs rendering commands with animation scheduling off: the test
// GIFs loop forever, so their ticks would never settle.
func drainStill(t testing.TB, m *Model, cmd tea.Cmd) {
	t.Helper()
	help := m.help
	m.help = true
	drainAnimation(t, m, cmd)
	m.help = help
}

func cellCount(line string) int { return strings.Count(line, string(kitty.Placeholder)) }
func idSGR(id int) string {
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", (id>>16)&0xff, (id>>8)&0xff, id&0xff)
}

// Under Kitty graphics the GIF plays in an overlay the size of its cells: the
// page picture carries no animation, the overlay's window is the GIF's cells,
// and the view splices the overlay's placeholders into the page grid there.
func TestTypesetOverlaysPlayGIFRegions(t *testing.T) {
	m := typesetGIFModel(t, "kitty", 80, 30)
	if m.pic.Mode() != picture.PictureKitty {
		t.Fatal("not under Kitty graphics")
	}
	if m.presentation.Source != nil {
		t.Fatal("the page picture still animates")
	}
	if len(m.typesetOverlays) != 1 {
		t.Fatalf("%d overlays, want 1", len(m.typesetOverlays))
	}
	o := m.typesetOverlays[0]
	cw, ch := m.pic.CellPixelSize()
	region := m.typesetPlayback.Regions[0].Rect
	want := image.Rect(region.Min.X/cw, region.Min.Y/ch, (region.Max.X+cw-1)/cw, (region.Max.Y+ch-1)/ch)
	if o.cells != want || !o.cells.In(image.Rect(0, 0, m.width, m.bodyHeight())) {
		t.Fatalf("overlay cells %v, want %v inside the body", o.cells, want)
	}
	if o.anim.Front == nil || o.anim.Source.Image().Bounds().Size() != image.Pt(o.cells.Dx()*cw, o.cells.Dy()*ch) {
		t.Fatalf("overlay not presented at its window size: front %v, image %v", o.anim.Front != nil, o.anim.Source.Image().Bounds())
	}
	w, h := m.width, m.bodyHeight()
	body, _ := m.body(w, h)
	lines := strings.Split(body, "\n")
	if len(lines) != h {
		t.Fatalf("%d body lines, want %d", len(lines), h)
	}
	overlay, page := idSGR(o.anim.ID), idSGR(m.presentation.ID)
	for y, line := range lines {
		if n := cellCount(line); n != w {
			t.Fatalf("row %d has %d cells, want %d", y, n, w)
		}
		inside := y >= o.cells.Min.Y && y < o.cells.Max.Y
		if strings.Contains(line, overlay) != inside {
			t.Fatalf("row %d: overlay placeholders present %v, want %v", y, !inside, inside)
		}
		if !strings.Contains(line, page) && (o.cells.Min.X > 0 || o.cells.Max.X < w || !inside) {
			t.Fatalf("row %d lost the page's placeholders", y)
		}
	}
	if cmd := m.scheduleTypesetOverlays(); cmd == nil {
		t.Fatal("visible overlay was not scheduled")
	}
	if frame := m.typesetExportFrame(); frame.Bounds() != m.source.Bounds() {
		t.Fatal("export frame is not the page")
	}
	before := o
	cmd, _ := m.documentKey("down")
	drainStill(t, m, cmd)
	if len(m.typesetOverlays) != 1 || m.typesetOverlays[0] == before || m.typesetOverlays[0].cells.Min.Y > before.cells.Min.Y {
		t.Fatalf("scrolling did not move the overlay up: %+v", m.typesetOverlays)
	}
	drainStill(t, m, m.clearGraphics())
	if len(m.typesetOverlays) != 0 {
		t.Fatal("leaving the document kept overlays")
	}
}

func TestTypesetGlyphRenderingAnimatesWholePage(t *testing.T) {
	m := typesetGIFModel(t, "glyph", 80, 30)
	if len(m.typesetOverlays) != 0 || m.presentation.Source == nil {
		t.Fatal("glyph rendering should animate the composite page")
	}
}

func TestSpliceCells(t *testing.T) {
	cell := func(x int) string {
		return string(kitty.Placeholder) + string(kitty.Diacritic(0)) + string(kitty.Diacritic(x))
	}
	row := func(id, n int) string {
		var b strings.Builder
		b.WriteString(idSGR(id))
		for x := 0; x < n; x++ {
			b.WriteString(cell(x))
		}
		b.WriteString("\x1b[39m")
		return b.String()
	}
	base, over := row(1, 6), row(2, 2)
	got := spliceCells(base, 2, 4, over)
	want := idSGR(1) + cell(0) + cell(1) + "\x1b[39m" + over + idSGR(1) + cell(4) + cell(5) + "\x1b[39m"
	if got != want {
		t.Fatalf("splice:\n%q\nwant\n%q", got, want)
	}
	if spliceCells(base, 5, 7, over) != base || spliceCells("text", 0, 1, over) != "text" {
		t.Fatal("an impossible splice changed the row")
	}
}

// A page drawn above a GIF's resolution gets its overlay at the GIF's own:
// the picture's resolution factor shrinks the frame it prepares, the window
// is exactly that size, and the terminal enlarges it.
func TestTypesetOverlayAtGIFResolution(t *testing.T) {
	t.Cleanup(func() { picture.ForceKittyCapability(picture.KittyCapabilityUnknown) })
	m := New(Options{Files: []string{"page.md"}, FilesFS: gifPage(t), MarkdownMode: "typeset", Render: "kitty", DPI: 150, Page: 1})
	t.Cleanup(func() { _ = m.Close() })
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m.pic.SetCellPixelSize(20, 40) // 2400 pixels across: past the 960-point measure, so the page is drawn at 1.2×.
	drainStill(t, m, m.load(false))
	if len(m.typesetOverlays) != 1 || m.typesetShown == nil || m.typesetShown.Pixels.X != 2400 {
		t.Fatalf("overlays %d, shown %+v", len(m.typesetOverlays), m.typesetShown)
	}
	o := m.typesetOverlays[0]
	region := m.typesetPlayback.Regions[0].Rect
	if region.Dx() <= 40 {
		t.Fatalf("GIF not drawn above its 40 pixels: %v", region)
	}
	f := o.pic.KittyResolutionFactor()
	if f >= 1 || f < 0.8 {
		t.Fatalf("resolution factor %v, want about 1/1.2", f)
	}
	cw, ch := int(20*f), int(40*f)
	src := o.anim.Source.(*animate.CompositeSource)
	if got := src.Image().Bounds().Size(); got != image.Pt(o.cells.Dx()*cw, o.cells.Dy()*ch) {
		t.Fatalf("window %v, want %d×%d cells at %d×%d", got, o.cells.Dx(), o.cells.Dy(), cw, ch)
	}
	if len(src.Regions) != 1 || src.Regions[0].Rect.Size() != image.Pt(40, 20) {
		t.Fatalf("region not at the GIF's own size: %+v", src.Regions)
	}
	if o.anim.Front == nil || !o.anim.Exact(o.cells.Dx(), o.cells.Dy()) {
		t.Fatal("overlay not presented at its cells")
	}
	if frame := m.typesetExportFrame(); frame.Bounds() != m.source.Bounds() {
		t.Fatal("export frame is not the page")
	}
}

// A new page view gets its overlays only once it has stood still: a scroll
// or resize burst costs the page alone. A settle notice for an older view is
// ignored.
func TestTypesetOverlaysWaitForSettle(t *testing.T) {
	m := typesetGIFModel(t, "kitty", 80, 30)
	if len(m.typesetOverlays) != 1 {
		t.Fatalf("%d overlays after settling", len(m.typesetOverlays))
	}
	m.typesetTop += 75
	v := m.loader.Load(m.request(false))
	if v.Err != nil {
		t.Fatal(v.Err)
	}
	m.typesetShown = nil
	drainStill(t, m, func() tea.Msg { return nil })
	cmd := m.acceptTypesetView(v)
	if cmd == nil || len(m.typesetOverlays) != 0 || m.typesetPending == nil || m.presentation.Source != nil {
		t.Fatalf("overlays %d, pending %v: a moved view should wait", len(m.typesetOverlays), m.typesetPending != nil)
	}
	m.Update(typesetSettled{m.kittyID, m.typesetSettle - 1})
	if len(m.typesetOverlays) != 0 {
		t.Fatal("an older settle notice built overlays")
	}
	_, settled := m.Update(typesetSettled{m.kittyID, m.typesetSettle})
	if len(m.typesetOverlays) != 1 || m.typesetPending != nil || settled == nil {
		t.Fatalf("overlays %d after settling", len(m.typesetOverlays))
	}
}
