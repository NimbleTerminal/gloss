package app

import (
	"context"
	"image"
	"image/draw"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/NimbleMarkets/ntcharts/v2/picture"
	"github.com/charmbracelet/x/ansi/kitty"
	xdraw "golang.org/x/image/draw"
	"nimbleterminal.dev/gloss/internal/animate"
	"nimbleterminal.dev/gloss/internal/document"
	ntanimation "nimbleterminal.dev/ntpicture/animation"
)

// Under Kitty graphics the typeset page is one still picture and its animated
// regions play in overlays: a picture for each group of body cells the regions
// cover, fed that window of the page. A frame is then the GIF's own cells, not
// the whole viewport, and the page is not sent again while a GIF plays. Glyph
// rendering keeps animating the composite page.
type typesetOverlay struct {
	cells image.Rectangle // Body cells, where the view splices its placeholders.
	pic   picture.Model
	anim  *animate.Animator
}

// An overlayGroup is the cells one overlay covers and the resolution it is
// prepared at: a GIF drawn larger than its own pixels is sent at its own, and
// the terminal enlarges it, instead of resampling every frame here.
type overlayGroup struct {
	cells  image.Rectangle
	factor float64 // Of the page's pixel density, at most 1.
}

// layoutTypesetOverlays replaces the overlays with those of a new page view.
// Regions keep their playback positions: the request carried them.
func (m *Model) layoutTypesetOverlays(v document.Result) tea.Cmd {
	cmds := []tea.Cmd{m.stopTypesetOverlays()}
	if v.Playback == nil || v.TypesetView == nil || v.Image == nil || m.pic.Mode() != picture.PictureKitty {
		return tea.Batch(cmds...)
	}
	cw, ch := m.cellPixelSize()
	cw, ch = max(1, cw), max(1, ch)
	bounds := v.Image.Bounds()
	var groups []overlayGroup
	for _, r := range v.Animated {
		px := r.Rect.Intersect(bounds)
		if px.Empty() || r.Player == nil {
			continue
		}
		own := r.Player.Image().Bounds().Size()
		factor := min(1, float64(own.X)/float64(max(1, r.Rect.Dx())), float64(own.Y)/float64(max(1, r.Rect.Dy())))
		groups = mergeCells(groups, overlayGroup{image.Rect(px.Min.X/cw, px.Min.Y/ch, (px.Max.X+cw-1)/cw, (px.Max.Y+ch-1)/ch), factor})
	}
	for _, g := range groups {
		cells := g.cells
		window := image.Rect(cells.Min.X*cw, cells.Min.Y*ch, cells.Max.X*cw, cells.Max.Y*ch)
		// The picture prepares frames at these cell pixels; the window must be exactly that size to be shown as it is.
		opw, oph := max(1, int(float64(cw)*g.factor)), max(1, int(float64(ch)*g.factor))
		o := &typesetOverlay{cells: cells}
		o.pic = picture.NewWithConfig(picture.Config{KittyID: nextKittyID(), KittyZ: -1, Background: pictureBackground, CellPixelWidth: cw, CellPixelHeight: ch, KittyResolutionFactor: g.factor, KittyFormat: picture.KittyFormatZlib})
		o.pic.Toggle()
		o.anim = animate.New(&o.pic, animate.Config{NextID: nextKittyID, Background: pictureBackground})
		source, err := v.Playback.Window(context.Background(), window, image.Pt(cells.Dx()*opw, cells.Dy()*oph))
		if err != nil {
			continue
		}
		o.anim.SetSource(animate.NewPictureSource(source))
		o.anim.SetSize(cells.Dx(), cells.Dy())
		cmds = append(cmds, o.anim.Show(o.anim.Source.Image()))
		m.typesetOverlays = append(m.typesetOverlays, o)
	}
	return tea.Batch(cmds...)
}

// mergeCells adds g to groups, joining it with every group it overlaps, so
// regions sharing cells share one overlay, at the finer of their resolutions.
func mergeCells(groups []overlayGroup, g overlayGroup) []overlayGroup {
	for {
		merged := false
		for i, o := range groups {
			if o.cells.Overlaps(g.cells) {
				g = overlayGroup{o.cells.Union(g.cells), max(o.factor, g.factor)}
				groups = append(groups[:i], groups[i+1:]...)
				merged = true
				break
			}
		}
		if !merged {
			return append(groups, g)
		}
	}
}

func (m *Model) stopTypesetOverlays() tea.Cmd {
	var cmds []tea.Cmd
	for _, o := range m.typesetOverlays {
		cmds = append(cmds, o.anim.Clear())
	}
	m.typesetOverlays = nil
	return tea.Batch(cmds...)
}

// keepTypesetOverlays freezes the overlays for a main-screen quit.
func (m *Model) keepTypesetOverlays() tea.Cmd {
	var cmds []tea.Cmd
	for _, o := range m.typesetOverlays {
		cmds = append(cmds, o.anim.Keep())
	}
	m.typesetOverlays = nil
	return tea.Batch(cmds...)
}

// typesetOverlaySources are the overlays' playback states, which a new page
// view continues from.
func (m *Model) typesetOverlaySources() []ntanimation.PositionSource {
	var sources []ntanimation.PositionSource
	for _, o := range m.typesetOverlays {
		if s, ok := o.anim.Source.(*animate.PictureSource); ok {
			sources = append(sources, s.Composite.Positions())
		}
	}
	return sources
}

func (m *Model) scheduleTypesetOverlays() tea.Cmd {
	active := m.mediaVisible() && m.pic.Mode() == picture.PictureKitty
	var cmds []tea.Cmd
	for _, o := range m.typesetOverlays {
		cmds = append(cmds, o.anim.Schedule(active))
	}
	return tea.Batch(cmds...)
}

// typesetOverlayMessage hands an overlay its own tick, frame or presentation.
func (m *Model) typesetOverlayMessage(msg tea.Msg) (tea.Cmd, bool) {
	for _, o := range m.typesetOverlays {
		if cmd, ok := o.anim.Update(msg); ok {
			return cmd, true
		}
	}
	return nil, false
}

// updateTypesetOverlayPictures routes picture messages, as the page's own picture gets them.
func (m *Model) updateTypesetOverlayPictures(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd
	for _, o := range m.typesetOverlays {
		cmds = append(cmds, o.anim.Transmission(msg, o.pic.Update(msg)))
	}
	return tea.Batch(cmds...)
}

// spliceTypesetOverlays puts each presented overlay's placeholders into the
// page grid at its cells. An overlay still in flight leaves the page's own
// pixels, which show the frame the page was laid out with.
func (m *Model) spliceTypesetOverlays(grid string) string {
	if len(m.typesetOverlays) == 0 {
		return grid
	}
	lines := strings.Split(grid, "\n")
	for _, o := range m.typesetOverlays {
		if !o.anim.Exact(o.cells.Dx(), o.cells.Dy()) {
			continue
		}
		for i, row := range strings.Split(o.anim.View(), "\n") {
			if y := o.cells.Min.Y + i; y < len(lines) {
				lines[y] = spliceCells(lines[y], o.cells.Min.X, o.cells.Max.X, row)
			}
		}
	}
	return strings.Join(lines, "\n")
}

// spliceCells replaces cells x0 to x1 of a placeholder row with another row.
// A row is an SGR naming the image, three runes per cell and a reset; the
// replacement brings its own, so the page's are closed before and reopened
// after it. A row without those cells is returned as it is.
func spliceCells(line string, x0, x1 int, repl string) string {
	var starts []int
	for i, r := range line {
		if r == kitty.Placeholder {
			starts = append(starts, i)
		}
	}
	if x0 < 0 || x0 >= x1 || x1 > len(starts) {
		return line
	}
	end := len(line)
	if i := strings.IndexByte(line[starts[len(starts)-1]:], '\x1b'); i >= 0 {
		end = starts[len(starts)-1] + i
	}
	starts = append(starts, end)
	return line[:starts[x0]] + "\x1b[39m" + repl + line[:starts[0]] + line[starts[x1]:]
}

// typesetExportFrame is the page as shown: the page picture with each
// overlay's current frame drawn in its window.
func (m *Model) typesetExportFrame() image.Image {
	if len(m.typesetOverlays) == 0 || m.source == nil {
		return m.source
	}
	out := image.NewRGBA(m.source.Bounds())
	draw.Draw(out, out.Bounds(), m.source, m.source.Bounds().Min, draw.Src)
	cw, ch := m.cellPixelSize()
	for _, o := range m.typesetOverlays {
		frame := o.anim.Source.Image()
		dst := image.Rect(o.cells.Min.X*cw, o.cells.Min.Y*ch, o.cells.Max.X*cw, o.cells.Max.Y*ch)
		if dst.Size() == frame.Bounds().Size() {
			draw.Draw(out, dst, frame, frame.Bounds().Min, draw.Src)
		} else {
			xdraw.CatmullRom.Scale(out, dst, frame, frame.Bounds(), draw.Src, nil)
		}
	}
	return out
}
