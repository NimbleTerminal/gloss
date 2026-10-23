package document

import (
	"context"
	"errors"
	"fmt"
	"image"
	"math"
	"time"

	"nimbleterminal.dev/ntpicture/animation"
	"nimbleterminal.dev/nttypeset"
)

// TypesetRegion is an animated image placed in a typeset picture, in
// rendered-image pixels, with the player that animates it.
type TypesetRegion struct {
	Rect        image.Rectangle
	Player      *animation.Player
	Destination string
}

// TypesetLink is a web or document-local link hit area in rendered-image pixels.
type TypesetLink = nttypeset.Link

// TypesetPlace is one end of a selection: a page point in layout pixels, or a
// rune of a paragraph of a block once placed.
type TypesetPlace = nttypeset.Place

// TypesetSelection is the text between two places, in either order.
type TypesetSelection = nttypeset.Selection

// TypesetViewport uses pixels at the 150-DPI reading scale. Nil on Request
// selects a numbered 800×1125 export slice instead of a continuous viewport.
type TypesetViewport struct {
	// Link asks to reveal the first occurrence of this document-local link.
	// Layout progresses only as far as its target, under the usual budget.
	Link                    string
	Top, Height, Left, Zoom int
	End                     bool
	// Ahead asks the layout to go on past the view to the next block with a
	// picture, a link or a table in it, or to the end, without moving the
	// view: the next item is then among the result's TypesetItems.
	Ahead bool
	// Last asks the layout to go on to the last block with an item in it,
	// for a step back from the first item round to the last.
	Last bool
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
	out = Result{FontKey: FontKey(q.Font, q.Mono), Theme: q.Theme.Name, Generation: q.Generation, Kind: l.typesetKind, Text: l.typesetText, Typeset: l.typeset, TypesetWidth: typesetMeasure(q.TypesetWidth), Page: 1, Pages: 1}
	if q.Theme.Palette.Background.A == 0 {
		// Every theme's background is opaque: no palette is a caller's mistake,
		// and drawing the page in transparent black would hide it.
		out.Err = errors.New("typeset page has no theme palette")
		return
	}
	ctx := q.Context
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	// fail reports a library error. A cancelled request leaves a consistent
	// layout behind, which the next request takes up where it was; a layout
	// that failed of itself is not kept.
	fail := func(err error) Result {
		out.Err = err
		if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			l.direct = nil
		}
		return out
	}
	changed, err := setTypesetFonts(l.typesetEngine, q.Font, q.Mono)
	if err != nil {
		out.Err = err
		return
	}
	doc := l.typesetDocument
	width := int(math.Round(float64(out.TypesetWidth) * 150 / 72))
	numbers := q.TypesetLineNumbers && doc.Lines
	chop := q.TypesetChop && doc.Lines
	options := nttypeset.LayoutOptions{Numbers: numbers, Chop: chop}
	old := l.direct
	if old == nil || old.Width() != width || changed || l.directOptions != options {
		l.direct, err = l.typesetEngine.Layout(doc, width, options)
		if err != nil {
			return fail(err)
		}
		l.directOptions = options
	}
	lay := l.direct
	top, height, left, zoom := 0, 1125, 0, 0
	if q.TypesetView != nil {
		v := q.TypesetView
		top, height, left, zoom = max(0, min(v.Top, 1125*MaxPages)), max(1, min(v.Height, 4096)), max(0, v.Left), max(0, min(v.Zoom, 4))
		if old != nil && old != lay && top > 0 {
			// Anchor the visible block when line wrapping changes above it.
			was := old.Starts()
			index := 0
			for i, y := range was {
				if y <= top {
					index = i
				}
			}
			if err := lay.EnsureBlock(ctx, index); err != nil {
				return fail(err)
			}
			if starts := lay.Starts(); index < len(starts) && index < len(was) {
				top = starts[index] + max(0, top-was[index])
			}
		}
		if v.Line > 0 && doc.Lines {
			index := min(v.Line, len(doc.Blocks)) - 1
			if err := lay.EnsureBlock(ctx, index); err != nil {
				return fail(err)
			}
			if starts := lay.Starts(); index >= 0 && index < len(starts) {
				top = max(0, starts[index]-50)
			}
		}
		if v.End {
			if err := lay.Complete(ctx); err != nil {
				return fail(err)
			}
			top = max(0, lay.Extent()-max(1, height>>zoom))
		}
		if v.Ahead {
			if err := lay.EnsureBlock(ctx, nextItemBlock(doc.Blocks, lay.Laid())); err != nil {
				return fail(err)
			}
		}
		if last := lastItemBlock(doc.Blocks); v.Last && last >= 0 {
			if err := lay.EnsureBlock(ctx, last); err != nil {
				return fail(err)
			}
		}
		if _, ok := FootnotePeer(v.Link); ok {
			r, found, err := lay.Find(ctx, v.Link)
			if err != nil {
				return fail(err)
			}
			if found {
				top, left = max(0, r.Min.Y-20), max(0, r.Min.X-50)
			}
		}
	} else {
		if err := lay.Complete(ctx); err != nil {
			return fail(err)
		}
		out.Pages = max(1, (lay.Extent()+1124)/1125)
		if err := pastEnd(q, out.Pages, "slice", "typeset text"); err != nil {
			out.Err = err
			return
		}
		out.Page = max(1, min(q.Page, out.Pages))
		top = (out.Page - 1) * 1125
	}
	width = max(1, width>>zoom)
	height = max(1, height>>zoom)
	if err := lay.Ensure(ctx, top+height); err != nil {
		return fail(err)
	}
	done := lay.Done()
	if q.TypesetView != nil && done {
		top = min(top, max(0, lay.Extent()-height))
	}
	left = min(left, max(0, max(lay.Width(), lay.ExtentX()+50)-width))
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
		if doc.Lines {
			out.TypesetView.Lines, out.TypesetView.Line = len(doc.Blocks), lay.LineAt(top)
		}
		out.TypesetDone = done
		out.TypesetExtent = lay.Extent()
	}
	view := nttypeset.View{Top: top, Left: left, Width: width, Height: height, Scale: scale, Pixels: pixels}
	size := view.Size()
	if w, h := size.X, size.Y; w > MaxPixels/h {
		out.Err = fmt.Errorf("typeset raster exceeds %d pixels", MaxPixels)
		return
	}
	if q.Animate && !q.Preview {
		for _, im := range lay.Images(view) {
			if player := l.typesetAnimations[im.Key]; player != nil {
				out.Animated = append(out.Animated, TypesetRegion{Rect: im.Rect, Player: player, Destination: im.Key})
			}
		}
	}
	opts := nttypeset.RasterOptions{Palette: q.Theme.Palette, OmitAnimated: len(out.Animated) > 0}
	if q.TypesetSelect != nil {
		asked := *q.TypesetSelect
		out.TypesetAsked = &asked
		sel, err := lay.Select(ctx, asked)
		if err != nil {
			return fail(err)
		}
		if sel != nil {
			places := sel.Places()
			opts.Selection = sel
			out.TypesetSelection, out.TypesetSelected = &places, sel.Text()
		}
	}
	picture, err := lay.Raster(ctx, view, opts)
	if err != nil {
		return fail(err)
	}
	// The focus is drawn into the page before the animations' frames go in:
	// it lies outside them, so none covers it, and it shows in glyphs and
	// under Kitty alike.
	if len(q.TypesetFocus) > 0 {
		out.TypesetFocus = append([]image.Rectangle(nil), q.TypesetFocus...)
		outlineFocus(picture, view, q.TypesetFocus, q.Theme.Palette)
	}
	if len(out.Animated) > 0 {
		// The page is drawn once, without its animated images: the composite
		// puts their current frames in. Drawing them here too would be a second
		// raster and a resample of each first frame, thrown away.
		out.AnimationBase = picture
	} else {
		out.Image = picture
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
	out.Links = lay.Links(view)
	if q.TypesetView != nil && l.typesetKind == "markdown" {
		out.TypesetItems = l.typesetItems(lay)
		out.TypesetItemsAll = lay.Laid() > lastItemBlock(doc.Blocks)
	}
	out.Info = append(append([]Field(nil), l.file...), l.typesetInfo...)
	out.Info = append(out.Info, fontFields(l.typesetEngine)...)
	return
}
