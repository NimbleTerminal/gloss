package document

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"math"
	"time"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/styles"
	"nimbleterminal.dev/gloss/internal/theme"
	"nimbleterminal.dev/ntpicture/animation"
)

// A palette is the set of colors a request paints with, built once per
// request. The layout records roles and token types, never colors, so changing
// theme repaints without reshaping.
type palette struct {
	background, foreground, muted, border, code, header, accent, selection color.RGBA
	syntax                                                                 *chroma.Style
}

func paletteFor(name string) palette {
	p := theme.Get(name)
	return palette{p.Background, p.Foreground, p.Muted, p.Border, p.Code, p.Header, p.Accent, p.Selection, styles.Get(theme.Syntax(name))}
}

func (p palette) fillColor(f fillRole) color.RGBA {
	switch f {
	case fillBorder:
		return p.border
	case fillCode:
		return p.code
	case fillHeader:
		return p.header
	case fillMuted:
		return p.muted
	}
	return p.background
}

// runInk is the color a text run is painted in. A syntax token wins over the
// span's own ink, an accent role over both, and a gutter number is muted.
func (p palette) runInk(op *operation) color.RGBA {
	ink := p.foreground
	if op.ink.A != 0 {
		ink = op.ink
	}
	if op.token != 0 {
		ink = p.foreground
		if c := p.syntax.Get(op.token).Colour; c.IsSet() {
			ink = color.RGBA{c.Red(), c.Green(), c.Blue(), 255}
		}
	}
	if op.role == roleAccent {
		ink = p.accent
	}
	if op.gutter {
		ink = p.muted
	}
	return ink
}

// TypesetViewport uses pixels at the 150-DPI reading scale. Nil on Request
// selects a numbered 800×1125 export slice instead of a continuous viewport.
type TypesetRegion struct {
	Rect        image.Rectangle
	Player      *animation.Player
	Destination string
}

// TypesetLink is a web or document-local link hit area in rendered-image pixels.
type TypesetLink struct {
	Rect image.Rectangle
	URL  string
}

type TypesetViewport struct {
	// Link asks to reveal the first occurrence of this document-local link.
	// Layout progresses only as far as its target, under the usual budget.
	Link                    string
	Top, Height, Left, Zoom int
	End                     bool
	// Line asks for a file line at the top, where the first line sits at the
	// top of the page, for a document whose blocks are its lines; a result
	// says which line is at the top. Lines is how many the document has, on
	// a result; zero where blocks are not lines, as on a Markdown page.
	Line, Lines int
	// Numbers says the picture has a gutter of line numbers, on a result;
	// Chop, that its lines are laid out unwrapped.
	Numbers, Chop bool
	// Pixels is the size of the picture the viewer will show, in device
	// pixels. The raster is exactly that size, so the terminal picture is
	// copied, not resampled; the reading scale is unchanged. Zero rasters
	// at the layout scale.
	Pixels image.Point
	// Width is the page width the picture shows, set on a result: with Top,
	// Left and Height it maps a picture pixel back to the page.
	Width int
}

func (l *Loader) renderTypeset(q Request) (out Result) {
	out = Result{FontKey: FontKey(q.Font, q.Mono), Theme: theme.Resolve(q.Theme), Generation: q.Generation, Kind: l.typesetKind, Text: l.typesetText, Typeset: l.typeset, TypesetWidth: typesetMeasure(q.TypesetWidth), Page: 1, Pages: 1}
	ctx := q.Context
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	l.typesetEngine.ctx = ctx
	defer func() {
		if p := recover(); p != nil {
			out.Err = typesetError(p)
			// A cancelled request leaves a consistent layout behind, which
			// the next request takes up where it was; a layout that failed
			// of itself is not kept.
			if ctx.Err() == nil {
				l.direct = nil
			}
		}
	}()
	changed, err := l.typesetEngine.setFonts(q.Font, q.Mono)
	if err != nil {
		out.Err = err
		return
	}
	width := int(math.Round(float64(out.TypesetWidth) * 150 / 72))
	numbers := q.TypesetLineNumbers && l.typesetPrepared.lines
	chop := q.TypesetChop && l.typesetPrepared.lines
	old := l.direct
	if old == nil || old.width != width || changed || old.numbers != numbers || old.chop != chop {
		l.direct = l.typesetEngine.layout(l.typesetPrepared, width)
		l.direct.numbers, l.direct.chop = numbers, chop
	}
	lay := l.direct
	top, height, left, zoom := 0, 1125, 0, 0
	if q.TypesetView != nil {
		v := q.TypesetView
		top, height, left, zoom = max(0, min(v.Top, 1125*MaxPages)), max(1, min(v.Height, 4096)), max(0, v.Left), max(0, min(v.Zoom, 4))
		if old != nil && old != lay && top > 0 {
			// Anchor the visible block when line wrapping changes above it.
			index := 0
			for i, y := range old.starts {
				if y <= top {
					index = i
				}
			}
			for lay.next <= index && lay.next < len(lay.p.blocks) {
				lay.ensure(lay.y + 1)
			}
			if index < len(lay.starts) {
				top = lay.starts[index] + max(0, top-old.starts[index])
			}
		}
		if v.Line > 0 && lay.p.lines {
			index := min(v.Line, len(lay.p.blocks)) - 1
			for lay.next <= index && lay.next < len(lay.p.blocks) {
				lay.ensure(lay.y + 1)
			}
			top = max(0, lay.starts[index]-50)
		}
		if v.End {
			lay.ensure(1125 * MaxPages)
			top = max(0, lay.y+50-max(1, height>>zoom))
		}
		if _, ok := FootnotePeer(v.Link); ok {
			at := 0
			for {
				found := false
				for ; at < len(lay.ops); at++ {
					if op := lay.ops[at]; op.href == v.Link {
						top, left, found = max(0, op.rect.Min.Y-20), max(0, op.rect.Min.X-50), true
						break
					}
				}
				if found || lay.next == len(lay.p.blocks) {
					break
				}
				lay.ensure(lay.y + 1)
			}
		}
	} else {
		lay.ensure(1125 * MaxPages)
		out.Pages = max(1, (lay.y+50+1124)/1125)
		if err := pastEnd(q, out.Pages, "slice", "typeset text"); err != nil {
			out.Err = err
			return
		}
		out.Page = max(1, min(q.Page, out.Pages))
		top = (out.Page - 1) * 1125
	}
	width = max(1, width>>zoom)
	height = max(1, height>>zoom)
	lay.ensure(top + height)
	done := lay.next == len(lay.p.blocks)
	if q.TypesetView != nil && done {
		top = min(top, max(0, lay.y+50-height))
	}
	left = min(left, max(0, max(lay.width, lay.extentX+50)-width))
	scale := float64(max(36, min(q.DPI, 600))) / 150
	if q.DPI == 0 {
		scale = 1
	}
	if q.MaxEdge > 0 {
		scale = float64(q.MaxEdge) / float64(max(width, height))
	}
	var pixels image.Point
	if q.TypesetView != nil {
		if p := q.TypesetView.Pixels; p.X > 0 && p.Y > 0 {
			pixels = p
			scale = float64(p.X) / float64(width)
		}
		out.TypesetView = &TypesetViewport{Top: top, Height: height, Left: left, Zoom: zoom, Pixels: pixels, Width: width, Numbers: numbers, Chop: chop}
		if lay.p.lines {
			out.TypesetView.Lines, out.TypesetView.Line = len(lay.p.blocks), lay.lineAt(top)
		}
		out.TypesetDone = done
		out.TypesetExtent = lay.y + 50
	}
	w, h := rasterSize(width, height, scale)
	if pixels != (image.Point{}) {
		w, h = pixels.X, pixels.Y
	}
	if w > MaxPixels/h {
		out.Err = fmt.Errorf("typeset raster exceeds %d pixels", MaxPixels)
		return
	}
	if q.Animate && !q.Preview {
		viewport := image.Rect(0, 0, w, h)
		for _, im := range lay.images(top, left, scale, viewport) {
			if player := l.typesetAnimations[im.destination]; player != nil {
				out.Animated = append(out.Animated, TypesetRegion{Rect: im.rect, Player: player, Destination: im.destination})
			}
		}
	}
	if q.TypesetSelect != nil {
		asked := *q.TypesetSelect
		out.TypesetAsked = &asked
		if sel, ok := lay.resolve(asked); ok {
			lay.sel = &sel
			defer func() { lay.sel = nil }()
			out.TypesetSelection, out.TypesetSelected = &sel.places, lay.selected(sel)
		}
	}
	pal := paletteFor(out.Theme)
	if len(out.Animated) > 0 {
		// The page is drawn once, without its animated images: the composite
		// puts their current frames in. Drawing them here too would be a second
		// raster and a resample of each first frame, thrown away.
		out.AnimationBase = lay.rasterTo(top, height, left, width, w, h, scale, pal, len(out.Animated) > 0)
	} else {
		out.Image = lay.rasterTo(top, height, left, width, w, h, scale, pal, false)
	}
	if q.Animate && !q.Preview && (len(out.Animated) > 0 || q.Playback != nil) {
		base := out.AnimationBase
		if base == nil {
			base = out.Image
		}
		previous := q.Playback
		if q.Reload {
			previous = nil
		}
		ctx := q.Context
		if ctx == nil {
			ctx = context.Background()
		}
		regions := make([]animation.Region, 0, len(out.Animated))
		for _, r := range out.Animated {
			regions = append(regions, animation.Region{Rect: r.Rect, Player: r.Player, Key: r.Destination})
		}
		playback, err := animation.NewCompositeImmutable(ctx, base, regions, previous)
		if err != nil {
			out.Err = err
			return
		}
		out.Playback = playback
		out.Image = out.Playback.Image()
	}
	for _, op := range lay.ops {
		if op.href == "" {
			continue
		}
		r := op.rect.Sub(image.Pt(left, top))
		r = image.Rect(int(math.Round(float64(r.Min.X)*scale)), int(math.Round(float64(r.Min.Y)*scale)), int(math.Round(float64(r.Max.X)*scale)), int(math.Round(float64(r.Max.Y)*scale))).Intersect(out.Image.Bounds())
		if !r.Empty() {
			out.Links = append(out.Links, TypesetLink{Rect: r, URL: op.href})
		}
	}
	out.Info = append(append([]Field(nil), l.file...), l.typesetInfo...)
	out.Info = append(out.Info, l.typesetEngine.fontFields()...)
	return
}

// Keep the spike's testable unit-scale viewport entry point.
func (l *layout) viewport(top, height int, p palette) *image.RGBA {
	return l.raster(top, height, 0, l.width, 1, p)
}
