package document

import (
	"image"
	"image/draw"
	"math"
	"sort"
	"strings"

	"github.com/yuin/goldmark/ast"
	"nimbleterminal.dev/nttypeset"
)

// TypesetItemKind says what a TypesetItem is.
type TypesetItemKind int

const (
	TypesetPicture TypesetItemKind = iota
	TypesetLinkItem
	TypesetTable
)

// TypesetItem is a picture, link or table on a typeset Markdown page, for
// the viewer to move between and act on.
type TypesetItem struct {
	Kind        TypesetItemKind
	Rects       []image.Rectangle // Page layout pixels (scale 1, top of page 0).
	URL         string            // A link, or the link around a picture.
	Destination string            // A picture's key.
	Path        string            // A picture's own file, as MarkdownImage.Path.
	Alt         string            // A picture's alt text.
	Image       image.Image       // A picture's still.
	Animated    bool              // A picture that plays: what is on screen is its frame.
	Cells       [][]string        // A table's plain text, header first.
}

// typesetItems lists the items of the page laid so far, in reading order.
// A link around a picture is the picture's; a footnote's links are none.
func (l *Loader) typesetItems(lay *nttypeset.Layout) []TypesetItem {
	page := nttypeset.View{Top: 0, Height: lay.Extent(), Width: lay.Width(), Scale: 1}
	var items []TypesetItem
	for _, im := range lay.Images(page) {
		items = append(items, TypesetItem{Kind: TypesetPicture, Rects: []image.Rectangle{im.Rect}, Destination: im.Key, Path: l.typesetPaths[im.Key], Alt: l.typesetAlts[im.Key], Image: l.typesetImages[im.Key], Animated: l.typesetAnimations[im.Key] != nil})
	}
	pictures := len(items)
	last := -1 // The link item the previous link rect went to.
links:
	for _, link := range lay.Links(page) {
		if _, note := FootnotePeer(link.URL); note {
			last = -1
			continue
		}
		for i := range items[:pictures] {
			if link.Rect.In(items[i].Rects[0]) {
				items[i].URL, last = link.URL, -1
				continue links
			}
		}
		if last >= 0 && items[last].URL == link.URL {
			items[last].Rects = append(items[last].Rects, link.Rect)
			continue
		}
		items = append(items, TypesetItem{Kind: TypesetLinkItem, Rects: []image.Rectangle{link.Rect}, URL: link.URL})
		last = len(items) - 1
	}
	starts, blocks := lay.Starts(), l.typesetDocument.Blocks
	for i, top := range starts {
		b := blocks[i]
		if b.Kind != nttypeset.KindTable || len(b.Rows) == 0 {
			continue
		}
		// As the layout places a table: from the block's indent to the right
		// margin, with a gap of 20 below it.
		bottom := lay.Extent() - nttypeset.Margin - 20
		if i+1 < len(starts) {
			bottom = starts[i+1] - 20
		}
		x := nttypeset.Margin + b.Indent
		rect := image.Rect(x, top, max(x+32, lay.Width()-nttypeset.Margin), bottom)
		if rect.Empty() {
			continue
		}
		var cells [][]string
		for _, row := range b.Rows {
			var texts []string
			for _, cell := range row {
				var s strings.Builder
				for _, span := range cell {
					s.WriteString(span.Text)
				}
				texts = append(texts, strings.TrimSpace(s.String()))
			}
			cells = append(cells, texts)
		}
		items = append(items, TypesetItem{Kind: TypesetTable, Rects: []image.Rectangle{rect}, Cells: cells})
	}
	// Down the page, then across; a table goes before the links in it.
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i].Rects[0].Min, items[j].Rects[0].Min
		if a.Y != b.Y {
			return a.Y < b.Y
		}
		return a.X < b.X
	})
	return items
}

// nextItemBlock is the first block from index from on that holds an item:
// a picture, a table, or a link that is not a footnote's; the last block
// when none does.
func nextItemBlock(blocks []nttypeset.Block, from int) int {
	for i := from; i < len(blocks); i++ {
		if holdsItem(blocks[i]) {
			return i
		}
	}
	return len(blocks) - 1
}

// holdsItem reports whether a block has an item in it: it is a picture or a
// table, or it links somewhere other than a footnote.
func holdsItem(b nttypeset.Block) bool {
	if b.Kind == nttypeset.KindImage || b.Kind == nttypeset.KindTable {
		return true
	}
	for _, s := range b.Spans {
		if _, note := FootnotePeer(s.Href); s.Href != "" && !note {
			return true
		}
	}
	return false
}

// pictureAlts is the alt text of each picture a Markdown tree shows, by
// destination: the first that has any.
func pictureAlts(root ast.Node, source []byte) map[string]string {
	alts := map[string]string{}
	_ = ast.Walk(root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if im, ok := n.(*ast.Image); ok && entering {
			if dest := string(im.Destination); alts[dest] == "" {
				alts[dest] = inlineText(im, source)
			}
			return ast.WalkSkipChildren, nil
		}
		return ast.WalkContinue, nil
	})
	return alts
}

// lastItemBlock is the last block that holds an item, as nextItemBlock
// finds them; -1 when none does.
func lastItemBlock(blocks []nttypeset.Block) int {
	for i := len(blocks) - 1; i >= 0; i-- {
		if holdsItem(blocks[i]) {
			return i
		}
	}
	return -1
}

// outlineFocus draws the focus rectangles, given in page pixels, onto the
// raster of view: a band in the accent, 3 layout pixels wide, just outside
// each one.
func outlineFocus(picture *image.RGBA, view nttypeset.View, focus []image.Rectangle, palette nttypeset.Palette) {
	width := max(1, int(math.Round(3*view.Scale)))
	ink := image.NewUniform(palette.Accent)
	for _, r := range focus {
		r = view.Project(r)
		outer := r.Inset(-width)
		for _, band := range []image.Rectangle{
			image.Rect(outer.Min.X, outer.Min.Y, outer.Max.X, r.Min.Y),
			image.Rect(outer.Min.X, r.Max.Y, outer.Max.X, outer.Max.Y),
			image.Rect(outer.Min.X, r.Min.Y, r.Min.X, r.Max.Y),
			image.Rect(r.Max.X, r.Min.Y, outer.Max.X, r.Max.Y),
		} {
			draw.Draw(picture, band.Intersect(picture.Bounds()), ink, image.Point{}, draw.Src)
		}
	}
}
