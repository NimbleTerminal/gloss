package document

import (
	"context"
	"fmt"
	"image"
	"math"
	"nimbleterminal.dev/gloss/internal/animate"
	"nimbleterminal.dev/gloss/internal/theme"
	"time"
)

// TypesetViewport uses pixels at the 150-DPI reading scale. Nil on Request
// selects a numbered 800×1125 export slice instead of a continuous viewport.
type TypesetRegion = animate.Region

// TypesetLink is a web link hit area in rendered-image pixels.
type TypesetLink struct {
	Rect image.Rectangle
	URL  string
}

type TypesetViewport struct {
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
	l.typesetEngine.theme = out.Theme
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
		for _, op := range lay.ops {
			player := lay.p.animations[op.destination]
			if op.asset == nil || player == nil {
				continue
			}
			r := op.rect.Sub(image.Pt(left, top))
			r = image.Rect(int(math.Round(float64(r.Min.X)*scale)), int(math.Round(float64(r.Min.Y)*scale)), int(math.Round(float64(r.Max.X)*scale)), int(math.Round(float64(r.Max.Y)*scale)))
			if r.Overlaps(viewport) {
				out.Animated = append(out.Animated, TypesetRegion{Rect: r, Player: player, Destination: op.destination})
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
	if len(out.Animated) > 0 {
		// The page is drawn once, without its animated images: the composite
		// puts their current frames in. Drawing them here too would be a second
		// raster and a resample of each first frame, thrown away.
		out.AnimationBase = lay.rasterTo(top, height, left, width, w, h, scale, true)
	} else {
		out.Image = lay.rasterTo(top, height, left, width, w, h, scale, false)
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
		out.Playback = animate.NewComposite(base, out.Animated, previous)
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
func (l *layout) viewport(top, height int) *image.RGBA { return l.raster(top, height, 0, l.width, 1) }
