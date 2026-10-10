package document

import (
	"context"
	"fmt"
	"github.com/NimbleTerminal/gloss/internal/animate"
	"github.com/NimbleTerminal/gloss/internal/theme"
	"image"
	"math"
	"time"
)

// TypesetViewport uses pixels at the 150-DPI reading scale. Nil on Request
// selects a numbered 800×1125 export slice instead of a continuous viewport.
type TypesetRegion = animate.Region

type TypesetViewport struct {
	Top, Height, Left, Zoom int
	End                     bool
}

func (l *Loader) renderTypeset(q Request) (out Result) {
	out = Result{Theme: theme.Resolve(q.Theme), Generation: q.Generation, Kind: l.typesetKind, Text: l.typesetText, Typeset: l.typeset, TypesetWidth: typesetMeasure(q.TypesetWidth), Page: 1, Pages: 1}
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
			l.direct = nil
		}
	}()
	width := int(math.Round(float64(out.TypesetWidth) * 150 / 72))
	old := l.direct
	if old == nil || old.width != width {
		l.direct = l.typesetEngine.layout(l.typesetPrepared, width)
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
	left = min(left, max(0, lay.width-width))
	if q.TypesetView != nil {
		out.TypesetView = &TypesetViewport{Top: top, Height: height, Left: left, Zoom: zoom}
		out.TypesetDone = done
		out.TypesetExtent = lay.y + 50
	}
	scale := float64(max(36, min(q.DPI, 600))) / 150
	if q.DPI == 0 {
		scale = 1
	}
	if q.MaxEdge > 0 {
		scale = float64(q.MaxEdge) / float64(max(width, height))
	}
	if float64(width)*float64(height)*scale*scale > MaxPixels {
		out.Err = fmt.Errorf("typeset raster exceeds %d pixels", MaxPixels)
		return
	}
	out.Image = lay.raster(top, height, left, width, scale)
	if q.Animate && !q.Preview {
		for _, op := range lay.ops {
			player := lay.p.animations[op.destination]
			if op.asset == nil || player == nil {
				continue
			}
			r := op.rect.Sub(image.Pt(left, top))
			r = image.Rect(int(math.Round(float64(r.Min.X)*scale)), int(math.Round(float64(r.Min.Y)*scale)), int(math.Round(float64(r.Max.X)*scale)), int(math.Round(float64(r.Max.Y)*scale)))
			if r.Overlaps(out.Image.Bounds()) {
				out.Animated = append(out.Animated, TypesetRegion{Rect: r, Player: player, Destination: op.destination})
			}
		}
		if len(out.Animated) > 0 {
			out.AnimationBase = lay.rasterFrame(top, height, left, width, scale, true)
		}
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
	out.Info = append(append([]Field(nil), l.file...), l.typesetInfo...)
	return
}

// Keep the spike's testable unit-scale viewport entry point.
func (l *layout) viewport(top, height int) *image.RGBA { return l.raster(top, height, 0, l.width, 1) }
