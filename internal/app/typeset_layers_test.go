package app

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"strings"
	"testing"
	"testing/fstest"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi/kitty"
	"math"
	"nimbleterminal.dev/gloss/internal/document"
	ntpicture "nimbleterminal.dev/ntpicture"
	ntanimation "nimbleterminal.dev/ntpicture/animation"
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

	m := New(Options{Files: []string{"page.md"}, FilesFS: gifPage(t), MarkdownMode: "typeset", Render: render, DPI: 150, Page: 1})
	t.Cleanup(func() { _ = m.Close() })
	m.Update(tea.WindowSizeMsg{Width: cols, Height: rows})
	drainStill(t, m, m.load(false))
	if m.typeset == nil || m.typesetPlayback == nil || len(m.typesetPlayback.Regions()) != 1 {
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

// Under Kitty graphics the GIF plays in an overlay the size of its cells: the
// page picture carries no animation, the overlay's window is the GIF's cells,
// and the view splices the overlay's placeholders into the page grid there.
func TestTypesetOverlaysPlayGIFRegions(t *testing.T) {
	m := typesetGIFModel(t, "kitty", 80, 30)
	if !m.kitty {
		t.Fatal("not under Kitty graphics")
	}
	if m.playback != nil {
		t.Fatal("typeset page used standalone playback")
	}
	if len(m.typesetLayers.Layers()) != 1 {
		t.Fatalf("%d overlays, want 1", len(m.typesetLayers.Layers()))
	}
	o := m.typesetLayers.Layers()[0]
	cw, ch := m.cellPixelSize()
	region := m.typesetPlayback.Regions()[0].Rect
	want := image.Rect(region.Min.X/cw, region.Min.Y/ch, (region.Max.X+cw-1)/cw, (region.Max.Y+ch-1)/ch)
	if o.Cells != want || !o.Cells.In(image.Rect(0, 0, m.width, m.bodyHeight())) {
		t.Fatalf("overlay cells %v, want %v inside the body", o.Cells, want)
	}
	if o.Status.Presented == nil || o.Status.Source.Image().Bounds().Size() != image.Pt(o.Cells.Dx()*cw, o.Cells.Dy()*ch) {
		t.Fatalf("overlay not presented at its window size: front %v, image %v", o.Status.Presented != nil, o.Status.Source.Image().Bounds())
	}
	w, h := m.width, m.bodyHeight()
	body, _ := m.body(w, h)
	lines := strings.Split(body, "\n")
	if len(lines) != h {
		t.Fatalf("%d body lines, want %d", len(lines), h)
	}

	for y, line := range lines {
		if n := cellCount(line); n != w {
			t.Fatalf("row %d has %d cells, want %d", y, n, w)
		}
	}

	if cmd := m.pauseTypesetLayers(); cmd == nil {
		t.Fatal("visible overlay was not scheduled")
	}
	if frame := m.typesetExportFrame(); frame.Bounds() != m.source.Bounds() {
		t.Fatal("export frame is not the page")
	}
	before := o
	cmd, _ := m.documentKey("down")
	drainStill(t, m, cmd)
	if len(m.typesetLayers.Layers()) != 1 || m.typesetLayers.Layers()[0].Cells.Min.Y > before.Cells.Min.Y {
		t.Fatalf("scrolling did not move the overlay up: %+v", m.typesetLayers.Layers())
	}
	drainStill(t, m, m.clearGraphics())
	if m.typesetLayers != nil {
		t.Fatal("leaving the document kept layers")
	}
}

func TestTypesetGlyphRenderingAnimatesWholePage(t *testing.T) {
	m := typesetGIFModel(t, "glyph", 80, 30)
	if m.typesetLayers == nil || len(m.typesetLayers.Layers()) != 0 || m.typesetLayers.Status().Presented == nil {
		t.Fatal("glyph rendering should animate the composite page")
	}
}

// A page drawn above a GIF's resolution gets its overlay at the GIF's own:
// the picture's resolution factor shrinks the frame it prepares, the window
// is exactly that size, and the terminal enlarges it.
func TestTypesetOverlayAtGIFResolution(t *testing.T) {

	m := New(Options{Files: []string{"page.md"}, FilesFS: gifPage(t), MarkdownMode: "typeset", Render: "kitty", DPI: 150, Page: 1})
	t.Cleanup(func() { _ = m.Close() })
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m.terminal.Update(uv.CellSizeEvent{Width: 20, Height: 40}) // 2400 pixels across: past the 960-point measure, so the page is drawn at 1.2×.
	drainStill(t, m, m.load(false))
	if len(m.typesetLayers.Layers()) != 1 || m.typesetShown == nil || m.typesetShown.Pixels.X != 2400 {
		t.Fatalf("overlays %d, shown %+v", len(m.typesetLayers.Layers()), m.typesetShown)
	}
	o := m.typesetLayers.Layers()[0]
	region := m.typesetPlayback.Regions()[0].Rect
	if region.Dx() <= 40 {
		t.Fatalf("GIF not drawn above its 40 pixels: %v", region)
	}
	f := o.Resolution
	if f >= 1 || f < 0.8 {
		t.Fatalf("resolution factor %v, want about 1/1.2", f)
	}
	src := o.Status.Source.(*ntanimation.Composite)
	want := image.Pt(int(math.Round(float64(o.Cells.Dx()*20)*f)), int(math.Round(float64(o.Cells.Dy()*40)*f)))
	if got := src.Image().Bounds().Size(); got != want {
		t.Fatalf("window %v, want %v", got, want)
	}
	regions := src.Regions()
	if len(regions) != 1 || regions[0].Rect.Size() != image.Pt(40, 20) {
		t.Fatalf("region not at the GIF's own size: %+v", regions)
	}
	if o.Status.Presented == nil {
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
	if len(m.typesetLayers.Layers()) != 1 {
		t.Fatalf("%d overlays after settling", len(m.typesetLayers.Layers()))
	}
	m.typesetTop += 75
	v := m.loader.Load(m.request(false))
	if v.Err != nil {
		t.Fatal(v.Err)
	}
	m.typesetShown = nil
	drainStill(t, m, func() tea.Msg { return nil })
	cmd := m.acceptTypesetView(v)
	if cmd == nil || len(m.typesetLayers.Layers()) != 0 || m.typesetPending == nil || m.playback != nil {
		t.Fatalf("overlays %d, pending %v: a moved view should wait", len(m.typesetLayers.Layers()), m.typesetPending != nil)
	}
	m.Update(typesetSettled{m.ownerID, m.typesetSettle - 1})
	if len(m.typesetLayers.Layers()) != 0 {
		t.Fatal("an older settle notice built overlays")
	}
	_, settled := m.Update(typesetSettled{m.ownerID, m.typesetSettle})
	drainStill(t, m, settled)
	if len(m.typesetLayers.Layers()) != 1 || m.typesetPending != nil || settled == nil {
		t.Fatalf("overlays %d after settling", len(m.typesetLayers.Layers()))
	}
}

// The export must use committed layers even after a newer source has been
// accepted but its presentation commands have not run yet.
func TestTypesetLayersExportRetainsPresentedViewport(t *testing.T) {
	m := typesetGIFModel(t, "kitty", 80, 30)
	expected, err := document.ExportImage(document.Result{Image: m.typesetLayers.Snapshot()}, 256)
	if err != nil {
		t.Fatal(err)
	}
	var out saved
	m.opts.MaxEdge, m.opts.Save = 256, out.save(t)
	m.typesetTop += 75
	v := m.loader.Load(m.request(false))
	if v.Err != nil {
		t.Fatal(v.Err)
	}
	m.source = v.Image
	pending := m.acceptTypesetView(v)
	cmd := m.export()
	if cmd == nil {
		t.Fatal("missing export")
	}
	cmd()
	if len(out.images) != 1 {
		t.Fatal("missing saved viewport")
	}
	got := out.images[0]
	if got.Bounds() != expected.Bounds() {
		t.Fatal("export bounds changed")
	}
	for y := got.Bounds().Min.Y; y < got.Bounds().Max.Y; y++ {
		for x := got.Bounds().Min.X; x < got.Bounds().Max.X; x++ {
			if color.RGBA64Model.Convert(got.At(x, y)) != color.RGBA64Model.Convert(expected.At(x, y)) {
				t.Fatalf("export used unpresented pixels at %d,%d", x, y)
			}
		}
	}
	drainStill(t, m, pending)
}

func TestTypesetLayersCarryPositionsAndKeep(t *testing.T) {
	m := typesetGIFModel(t, "kitty", 80, 30)
	q := m.request(false)
	positions, ok := q.Playback.(ntanimation.Positions)
	if !ok || len(positions.Keys()) != 1 {
		t.Fatal("reflow did not receive lightweight positions")
	}
	if m.request(true).Playback != nil {
		t.Fatal("reload carried playback")
	}
	before := m.pictureView()
	m.opts.KeepScreen = true
	drainStill(t, m, m.quit())
	if m.pictureView() != before {
		t.Fatal("keep changed the presented viewport")
	}
	if m.typesetLayers.Snapshot() == nil {
		t.Fatal("keep lost its snapshot")
	}
}

func TestTypesetLayersFollowGraphicsToggle(t *testing.T) {
	m := typesetGIFModel(t, "kitty", 80, 30)
	cmd, _ := m.documentKey("g")
	drainStill(t, m, cmd)
	if m.typesetLayers == nil || m.typesetLayers.Mode() != ntpicture.PictureGlyph || len(m.typesetLayers.Layers()) != 0 {
		t.Fatal("graphics toggle did not select flattened glyph playback")
	}
	cmd, _ = m.documentKey("g")
	drainStill(t, m, cmd)
	if m.typesetLayers.Mode() != ntpicture.PictureKitty || len(m.typesetLayers.Layers()) != 1 {
		t.Fatal("graphics toggle did not restore overlays")
	}
}

func TestTypesetLayersScrollOffscreen(t *testing.T) {
	m := typesetGIFModel(t, "kitty", 80, 30)
	cmd, _ := m.documentKey("end")
	drainStill(t, m, cmd)
	if m.typesetLayers == nil || len(m.typesetLayers.Layers()) != 0 {
		t.Fatal("offscreen animation kept overlay windows")
	}
	if m.typesetLayers.Snapshot() == nil {
		t.Fatal("scrolling past animations lost the page")
	}
	cmd, _ = m.documentKey("home")
	drainStill(t, m, cmd)
	if len(m.typesetLayers.Layers()) != 1 {
		t.Fatal("returning to animation did not restore its overlay")
	}
}

func TestTypesetLayersPlaybackFinishes(t *testing.T) {
	for _, render := range []string{"kitty", "glyph"} {
		t.Run(render, func(t *testing.T) {
			files := gifPage(t)
			clip, err := gif.DecodeAll(bytes.NewReader(files["a.gif"].Data))
			if err != nil {
				t.Fatal(err)
			}
			clip.LoopCount = -1
			var encoded bytes.Buffer
			if err := gif.EncodeAll(&encoded, clip); err != nil {
				t.Fatal(err)
			}
			files["a.gif"].Data = encoded.Bytes()
			m := New(Options{Files: []string{"page.md"}, FilesFS: files, Typeset: true, Render: render, DPI: 150, Page: 1})
			t.Cleanup(func() { m.Close() })
			m.width, m.height = 80, 30
			drainAnimation(t, m, m.load(false))
			if m.typesetLayers == nil || !m.typesetLayers.Status().Finished {
				t.Fatal("finite animation did not finish")
			}
			m.help = true
			if cmd := m.pauseTypesetLayers(); cmd != nil {
				t.Fatal("hiding finished playback changed its state")
			}
			m.help = false
			if cmd := m.pauseTypesetLayers(); cmd != nil {
				t.Fatal("showing finished playback restarted it")
			}
			region := m.typesetPlayback.Regions()[0].Rect
			frame := m.typesetLayers.Snapshot()
			at := region.Min.Add(region.Size().Div(2))
			if got := color.RGBAModel.Convert(frame.At(at.X, at.Y)).(color.RGBA); got.G < 200 || got.R > 20 {
				t.Fatalf("last green frame was not presented: %v", got)
			}
		})
	}
}

func TestTypesetLayersSourceToggle(t *testing.T) {
	m := typesetGIFModel(t, "kitty", 80, 30)
	layers := m.typesetLayers
	cmd, _ := m.documentKey("s")
	drainStill(t, m, cmd)
	if m.markdown == nil || m.typesetLayers != layers || layers.View() != "" {
		t.Fatal("source view did not hide and retain its layered picture")
	}
	cmd, _ = m.documentKey("s")
	drainStill(t, m, cmd)
	if m.markdown != nil || m.typesetLayers != layers || len(layers.Layers()) != 1 || layers.Snapshot() == nil {
		t.Fatal("returning from source did not restore layered playback")
	}
}
