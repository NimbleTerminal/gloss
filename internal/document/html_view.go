package document

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"io/fs"
	"math"
	"strconv"
	"time"

	nthtml "nimbleterminal.dev/nthtml/document"
	"nimbleterminal.dev/nttypeset"
)

// htmlScale is typeset pixels, at the 150-DPI reading scale, to a CSS pixel.
const htmlScale = 150.0 / 96

// htmlLimits raise two of nthtml's defaults so that long pages, as Go's
// language specification is, can be read: the input as typeset Markdown's,
// and the elements and runs of text. The rest stay nthtml's own.
var htmlLimits = nthtml.Limits{Bytes: 2 << 20, Nodes: 40000}

// htmlView is an HTML page laid out by its own CSS for the viewer, kept
// while it is shown: a scroll paints another band of it.
type htmlView struct {
	result    *nthtml.Result
	index     *nthtml.TextIndex // Its painted text, for selecting.
	serial    int               // Which layout this is, as placed selection ends say.
	markdown  *Markdown         // Its text, for the text view and search.
	info      []Field
	width     int // The CSS width it was laid out at.
	hash      [32]byte
	base, typ string
	font      string // The FontKey and theme it was laid out in.
	theme     string
}

// htmlLayoutWidthFor is the CSS width a page is laid out at for a reading
// measure in points, as a browser at 96 DPI would.
func htmlLayoutWidthFor(measure int) int {
	return int(math.Round(float64(typesetMeasure(measure)) * 96 / 72))
}

// htmlViewCurrent says the kept layout answers q.
func (l *Loader) htmlViewCurrent(q Request) bool {
	if q.MarkdownMode != "typeset" || q.TypesetView == nil || q.TextOnly || q.BaseDir != l.html.base || q.Type != l.html.typ || htmlLayoutWidthFor(q.TypesetWidth) != l.html.width ||
		FontKey(q.Font, q.Mono) != l.html.font || q.Theme.Name != l.html.theme {
		return false
	}
	data, err := readFileFrom(l.Files, q.Path)
	return err == nil && sha256.Sum256(data) == l.html.hash
}

// layHTMLView lays a page out for the viewer and keeps it. A page nthtml
// cannot lay out is shown as its text, with the reason among its details.
func (l *Loader) layHTMLView(q Request, data []byte, page *Page, baseDir string, files fs.FS, file []Field, out *Result) {
	md, err := loadMarkdownContext(context.Background(), q.Path, page.Markdown, q.BaseDir, files, false, q.Theme.Dark)
	if err != nil {
		out.Err = err
		return
	}
	width := htmlLayoutWidthFor(q.TypesetWidth)
	res, err := layOutHTML(q.Context, q, data, width, baseDir, files)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			out.Err = err
			return
		}
		out.Markdown = md
		out.Info = append(append(file, page.fields(md)...), Section("Layout", Field{"Not laid out", err.Error()})...)
		return
	}
	info := append(page.fields(md), Section("Layout",
		Field{"Width", strconv.Itoa(width) + " CSS px"},
		Field{"Height", strconv.Itoa(res.Bounds().Dy()) + " CSS px"})...)
	for _, w := range res.Warnings {
		info = append(info, Field{"Warning", w})
	}
	l.htmlSerial++
	l.html = &htmlView{result: res, index: res.TextIndex(), serial: l.htmlSerial, markdown: md, info: append(file, info...), width: width, hash: sha256.Sum256(data), base: q.BaseDir, typ: q.Type, font: FontKey(q.Font, q.Mono), theme: q.Theme.Name}
	l.path = q.Path
	*out = l.renderHTMLView(q)
}

// layOutHTML parses and lays out a page at a CSS width, without painting
// it, with the pictures beside it. Where the page names no font, or only
// monospace, it is set in q's font look; where it sets no colors, in its
// theme's.
func layOutHTML(ctx context.Context, q Request, data []byte, width int, baseDir string, files fs.FS) (*nthtml.Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	doc, err := nthtml.Parse(ctx, bytes.NewReader(data), htmlLimits)
	if errors.Is(err, nthtml.ErrLimit) {
		return nil, fmt.Errorf("the page is too large to lay out (%w)", err)
	}
	if err != nil {
		return nil, err
	}
	o := nthtml.Options{Width: width, Images: htmlImages(ctx, doc.ImageSources(), baseDir, files), NoImage: true}
	if q.Theme.Dark {
		o.ColorScheme = "dark"
	}
	if look, err := ResolveFont(q.Font, q.Mono); err == nil {
		o.FontFamilies = nthtml.FontFamilies{Default: look.Body, Monospace: look.Code}
	}
	if p := q.Theme.Palette; p.Background.A > 0 {
		o.Colors = nthtml.Colors{Background: p.Background, Text: p.Foreground, Link: p.Accent}
	}
	res, err := doc.Render(ctx, o)
	if errors.Is(err, nthtml.ErrLimit) {
		return nil, fmt.Errorf("the page is too large to lay out (%w)", err)
	}
	return res, err
}

// renderHTMLView paints the part of the kept page the viewport asks for,
// in its terms: typeset pixels at the reading scale, rastered at the
// viewer's device pixels.
func (l *Loader) renderHTMLView(q Request) (out Result) {
	h := l.html
	out = Result{Kind: "html", Page: 1, Pages: 1, Generation: q.Generation, Typeset: h.markdown, TypesetWidth: typesetMeasure(q.TypesetWidth), FontKey: FontKey(q.Font, q.Mono), Theme: q.Theme.Name, Info: h.info}
	ctx := q.Context
	if ctx == nil {
		ctx = context.Background()
	}
	v := *q.TypesetView
	bounds := h.result.Bounds()
	pageWidth, extent := int(math.Round(float64(bounds.Dx())*htmlScale)), int(math.Round(float64(bounds.Dy())*htmlScale))
	zoom := max(0, min(v.Zoom, 4))
	width, height := max(1, pageWidth>>zoom), max(1, max(1, min(v.Height, 4096))>>zoom)
	top := max(0, v.Top)
	if v.End {
		top = extent - height
	}
	if len(v.Link) > 1 && v.Link[0] == '#' {
		if p, ok := h.result.Anchors[v.Link[1:]]; ok {
			top = int(math.Round(float64(p.Y)*htmlScale)) - 20
		}
	}
	top = max(0, min(top, extent-height))
	left := max(0, min(v.Left, pageWidth-width))
	// scale is device pixels to a typeset pixel.
	scale, size := 1.0, image.Pt(width, height)
	if p := v.Pixels; p.X > 0 && p.Y > 0 {
		scale, size = float64(p.X)/float64(width), p
	}
	if size.X > MaxPixels/size.Y {
		out.Err = fmt.Errorf("page view exceeds %d pixels", MaxPixels)
		return
	}
	area := image.Rect(int(math.Floor(float64(left)/htmlScale)), int(math.Floor(float64(top)/htmlScale)),
		int(math.Ceil(float64(left+width)/htmlScale)), int(math.Ceil(float64(top+height)/htmlScale))).Intersect(bounds)
	canvas := image.NewRGBA(image.Rectangle{Max: size})
	// Past the page's end, its own canvas goes on.
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(h.result.Background()), image.Point{}, draw.Src)
	// device places a page point in CSS pixels on the canvas.
	device := func(p image.Point) image.Point {
		return image.Pt(int(math.Round((float64(p.X)*htmlScale-float64(left))*scale)), int(math.Round((float64(p.Y)*htmlScale-float64(top))*scale)))
	}
	if !area.Empty() {
		painted, err := h.result.Paint(ctx, area, htmlScale*scale)
		if err != nil {
			out.Err = err
			return
		}
		draw.Draw(canvas, painted.Bounds().Add(device(area.Min)), painted, image.Point{}, draw.Src)
	}
	for _, link := range h.result.Links {
		for _, r := range link.Regions {
			if rect := (image.Rectangle{device(r.Min), device(r.Max)}).Intersect(canvas.Bounds()); !rect.Empty() {
				out.Links = append(out.Links, TypesetLink{Rect: rect, URL: link.Href})
			}
		}
	}
	out.Image = canvas
	var pixels image.Point
	if v.Pixels.X > 0 && v.Pixels.Y > 0 {
		pixels = v.Pixels
	}
	out.TypesetView = &TypesetViewport{Top: top, Height: height, Left: left, Zoom: zoom, Pixels: pixels, Width: width}
	out.TypesetDone, out.TypesetExtent = true, extent
	if q.TypesetSelect != nil {
		asked := *q.TypesetSelect
		out.TypesetAsked = &asked
		if sel, text, start, end := h.selection(asked); text != "" {
			out.TypesetSelection, out.TypesetSelected = &sel, text
			c := q.Theme.Palette.Selection
			if c.A == 0 {
				c = color.RGBA{0x33, 0x99, 0xff, 0xff}
			}
			tint := image.NewUniform(color.NRGBA{c.R, c.G, c.B, 0x70})
			for _, r := range h.index.Regions(start, end) {
				draw.Draw(canvas, image.Rectangle{device(r.Min), device(r.Max)}, tint, image.Point{}, draw.Over)
			}
		}
	}
	// Its items are its links, the whole page's at once.
	out.TypesetItems, out.TypesetItemsAll = h.items(), true
	if len(q.TypesetFocus) > 0 {
		out.TypesetFocus = append([]image.Rectangle(nil), q.TypesetFocus...)
		outlineFocus(canvas, nttypeset.View{Top: top, Left: left, Scale: scale}, q.TypesetFocus, q.Theme.Palette)
	}
	// Its links have no marks to tint: what was asked is answered as it was
	// asked, so the viewer does not ask again.
	out.TypesetMarks = append([]TypesetMark(nil), q.TypesetMarks...)
	return out
}

// items are the page's links in reading order, as typeset items: each
// line of a link in page pixels at the reading scale.
func (h *htmlView) items() []TypesetItem {
	var items []TypesetItem
	for _, link := range h.result.Links {
		var rects []image.Rectangle
		for _, r := range link.Regions {
			if !r.Empty() {
				rects = append(rects, image.Rect(int(math.Floor(float64(r.Min.X)*htmlScale)), int(math.Floor(float64(r.Min.Y)*htmlScale)),
					int(math.Ceil(float64(r.Max.X)*htmlScale)), int(math.Ceil(float64(r.Max.Y)*htmlScale))))
			}
		}
		if len(rects) > 0 {
			items = append(items, TypesetItem{Kind: TypesetLinkItem, Rects: rects, URL: link.Href})
		}
	}
	return items
}

// selection places the ends of a selection in the page's text: a point
// on the page at the word nearest it, or an end placed before, in this
// layout. It gives the selection placed, its text, and its byte range.
func (h *htmlView) selection(asked TypesetSelection) (TypesetSelection, string, int, int) {
	place := func(p TypesetPlace) (TypesetPlace, nthtml.Match, bool) {
		if p.Placed {
			// An end placed in another layout of the page is not one here.
			return p, nthtml.Match{Start: p.Para, End: p.Rune}, p.Block == h.serial
		}
		at := image.Pt(int(float64(p.Point.X)/htmlScale), int(float64(p.Point.Y)/htmlScale))
		m, ok := h.index.Hit(image.Rectangle{at, at.Add(image.Pt(1, 1))})
		return TypesetPlace{Point: p.Point, Block: h.serial, Para: m.Start, Rune: m.End, Placed: true}, m, ok
	}
	anchor, a, okA := place(asked.Anchor)
	focus, f, okF := place(asked.Focus)
	if !okA || !okF {
		return TypesetSelection{}, "", 0, 0
	}
	start, end := min(a.Start, f.Start), max(a.End, f.End)
	return TypesetSelection{Anchor: anchor, Focus: focus}, h.index.Selected(start, end), start, end
}
