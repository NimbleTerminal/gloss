// This experiment deliberately supports a bounded Markdown subset, not HTML/CSS.
package document

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/NimbleTerminal/gloss/internal/animate"
	"github.com/NimbleTerminal/gloss/internal/theme"
	"github.com/alecthomas/chroma/v2"
	"github.com/go-text/render"
	"github.com/go-text/typesetting/di"
	"github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/shaping"
	"github.com/yuin/goldmark/ast"
	ext "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/util"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/math/fixed"
)

const (
	bold   = 1
	italic = 2
	mono   = 4
	strike = 8
)

type span struct {
	token   chroma.TokenType
	href    string
	text    string
	style   int
	ink     color.RGBA // zero means the ordinary text color
	darkInk color.RGBA
}
type block struct {
	href         string
	kind         string
	spans        []span
	rows         [][][]span
	asset        image.Image
	destination  string
	indent, size int
}
type prepared struct {
	ctx        context.Context
	lines      bool // The blocks are the file's lines, one each.
	blocks     []block
	images     map[string]image.Image
	animations map[string]*animate.Player
}

func inline(n ast.Node, data []byte, style int) []span {
	var out []span
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		switch v := c.(type) {
		case *ast.Text:
			raw := v.Segment.Value(data)
			if style&mono == 0 {
				raw = util.ResolveEntityNames(util.ResolveNumericReferences(util.UnescapePunctuations(raw)))
			}
			s := string(raw)
			if v.HardLineBreak() {
				s += "\n"
			} else if v.SoftLineBreak() {
				s += " "
			}
			out = append(out, span{text: s, style: style})
		case *ast.String:
			out = append(out, span{text: string(v.Value), style: style})
		case *ast.Emphasis:
			flag := italic
			if v.Level == 2 {
				flag = bold
			}
			out = append(out, inline(v, data, style|flag)...)
		case *ast.CodeSpan:
			out = append(out, inline(v, data, style|mono)...)
		case *ext.Strikethrough:
			out = append(out, inline(v, data, style|strike)...)
		case *ext.TaskCheckBox:
			s := "[ ] "
			if v.IsChecked {
				s = "[x] "
			}
			out = append(out, span{text: s, style: style | mono})
		case *ast.Link:
			linked := inline(v, data, style)
			for i := range linked {
				linked[i].href = string(v.Destination)
				linked[i].ink = theme.Get("light").Accent
				linked[i].darkInk = theme.Get("dark").Accent
			}
			out = append(out, linked...)
		case *ast.AutoLink:
			out = append(out, span{text: string(v.Label(data)), href: string(v.URL(data)), style: style, ink: theme.Get("light").Accent, darkInk: theme.Get("dark").Accent})
		case *ast.Image:
			out = append(out, span{text: "[image: " + string(v.Destination) + "]", style: style})
		default:
			out = append(out, inline(c, data, style)...)
		}
	}
	return out
}
func (p *prepared) walk(n ast.Node, data []byte, indent int) {
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if p.ctx != nil {
			if err := p.ctx.Err(); err != nil {
				panic(err)
			}
		}
		if len(p.blocks) > 32768 {
			panic("typeset document exceeds 32,768 blocks")
		}
		switch v := c.(type) {
		case *ast.Heading:
			heading := inline(v, data, bold)
			for i := range heading {
				heading[i].ink, heading[i].darkInk = theme.Get("light").Accent, theme.Get("dark").Accent
			}
			p.blocks = append(p.blocks, block{kind: "heading", spans: heading, size: []int{50, 46, 42, 38, 33, 29}[v.Level-1], indent: indent})
		case *ast.Paragraph, *ast.TextBlock:
			if c.ChildCount() == 1 {
				if im := standaloneImage(c.FirstChild()); im != nil {
					p.blocks = append(p.blocks, block{kind: "image", href: imageLink(im), destination: string(im.Destination), asset: p.images[string(im.Destination)], spans: []span{{text: "[image unavailable: " + string(im.Destination) + "]", style: italic}}, indent: indent})
					continue
				}
			}
			p.blocks = append(p.blocks, block{kind: "paragraph", spans: inline(c, data, 0), indent: indent})
		case *ast.FencedCodeBlock, *ast.CodeBlock:
			if c.Lines().Len() > 8192 {
				panic("typeset code block exceeds 8,192 lines")
			}
			var b strings.Builder
			for i := 0; i < c.Lines().Len(); i++ {
				s := c.Lines().At(i)
				b.Write(s.Value(data))
			}
			language := ""
			if fence, ok := c.(*ast.FencedCodeBlock); ok {
				language = string(fence.Language(data))
			}
			p.blocks = append(p.blocks, block{kind: "code", spans: highlightCodeContext(p.ctx, strings.TrimSuffix(b.String(), "\n"), language, codeTheme), size: 25, indent: indent})
		case *ast.Blockquote:
			start := len(p.blocks)
			p.walk(c, data, indent+24)
			for i := start; i < len(p.blocks); i++ {
				if p.blocks[i].kind == "paragraph" {
					p.blocks[i].kind = "quote"
				}
			}
		case *ast.List:
			index := v.Start
			if index < 1 {
				index = 1
			}
			for li := v.FirstChild(); li != nil; li = li.NextSibling() {
				start := len(p.blocks)
				p.walk(li, data, indent+24)
				if start < len(p.blocks) {
					marker := "• "
					if v.IsOrdered() {
						marker = fmt.Sprintf("%d. ", index)
					}
					p.blocks[start].spans = append([]span{{text: marker}}, p.blocks[start].spans...)
				}
				index++
			}
		case *ext.Table:
			b := block{kind: "table", indent: indent}
			for row := v.FirstChild(); row != nil; row = row.NextSibling() {
				var cells [][]span
				style := 0
				if _, ok := row.(*ext.TableHeader); ok {
					style = bold
				}
				for cell := row.FirstChild(); cell != nil; cell = cell.NextSibling() {
					cells = append(cells, inline(cell, data, style))
				}
				b.rows = append(b.rows, cells)
			}
			p.blocks = append(p.blocks, b)
		case *ast.ThematicBreak:
			p.blocks = append(p.blocks, block{kind: "rule"})
		case *ast.HTMLBlock: // no raw HTML layout
		default:
			p.walk(c, data, indent)
		}
	}
}

// A link around an image changes its destination when clicked, not its visual
// content. Only unwrap image-only links; mixed prose keeps its inline layout.
func standaloneImage(n ast.Node) *ast.Image {
	for n != nil {
		switch v := n.(type) {
		case *ast.Image:
			return v
		case *ast.Link:
			if n.ChildCount() != 1 {
				return nil
			}
			n = n.FirstChild()
		default:
			return nil
		}
	}
	return nil
}

type shaped struct {
	tokens   []chroma.TokenType
	text     []rune
	runs     []shaping.Output
	styles   []int
	inks     []color.RGBA
	darkInks []color.RGBA
	size     int
}
type engine struct {
	fontKey      string
	look         FontLook
	fallback     [8]*font.Face
	missing      map[rune]bool
	fallbackUsed map[string]bool
	faces        [8]*font.Face
	shaper       shaping.HarfbuzzShaper
	segmenter    shaping.Segmenter
	cache        map[string]*shaped
	cacheBytes   int
	renderer     render.Renderer
	ctx          context.Context
	theme        string
	// Rendered glyph masks, keyed by glyph, size, scale and quarter-pixel
	// position. Drawing every glyph from its outline again on every scroll
	// step was the raster's largest cost; a mask is drawn with the run's
	// colour in one blend. Bounded like the shaping cache.
	glyphs                 map[glyphKey]*glyphMask
	glyphBytes             int
	glyphBudget            int
	glyphHits, glyphMisses int
}

// glyphKey names a rendered glyph: the face and glyph, the size and raster
// scale, and the position within a pixel in quarters.
type glyphKey struct {
	face   *font.Face
	gid    font.GID
	size   int
	scale  float32
	fx, fy uint8
}

// glyphMask is a glyph's coverage, trimmed to its ink, with the offset of the
// mask's top-left from the glyph origin at an integer position.
type glyphMask struct {
	mask   *image.Alpha
	offset image.Point
}

const glyphQuanta = 4 // Positions within a pixel a mask is rendered for.

func newEngine(options ...string) (*engine, error) {
	e := &engine{glyphBudget: 32 << 20}
	name, override := "", ""
	if len(options) > 0 {
		name = options[0]
	}
	if len(options) > 1 {
		override = options[1]
	}
	_, err := e.setFonts(name, override)
	return e, err
}
func (e *engine) shape(spans []span, size int) *shaped {
	e.check()
	count := 0
	for _, sp := range spans {
		count += utf8.RuneCountInString(sp.text)
	}
	if count > 16384 {
		panic("typeset paragraph exceeds 16,384 runes")
	}

	var key strings.Builder
	fmt.Fprintf(&key, "%d/", size)
	for _, s := range spans {
		fmt.Fprintf(&key, "%d:%d:%v:%d:%s", s.token, s.style, [2]color.RGBA{s.ink, s.darkInk}, len(s.text), s.text)
	}
	k := key.String()
	if s := e.cache[k]; s != nil {
		return s
	}
	s := &shaped{size: size}
	for _, sp := range spans {
		r := []rune(sp.text)
		s.text = append(s.text, r...)
		for range r {
			s.styles = append(s.styles, sp.style)
			s.tokens = append(s.tokens, sp.token)
			s.inks = append(s.inks, sp.ink)
			s.darkInks = append(s.darkInks, sp.darkInk)
		}
	}
	offset := 0
	for _, sp := range spans {
		end := offset + utf8.RuneCountInString(sp.text)
		if end == offset {
			continue
		}
		in := shaping.Input{Text: s.text, RunStart: offset, RunEnd: end, Direction: di.DirectionLTR, Face: e.faces[sp.style&7], FontFeatures: noLigatures, Size: fixed.I(size)}
		for _, run := range e.segmenter.Split(in, fallbackMap{e, sp.style}) {
			s.runs = append(s.runs, e.shaper.Shape(run))
		}
		offset = end
	}
	// Per-engine bounded cache; eviction is deliberately simple for the spike.
	bytes := len(s.text)*28 + len(k)
	for _, r := range s.runs {
		bytes += len(r.Glyphs) * 128
	}
	if e.cacheBytes+bytes > 32<<20 {
		e.cache = map[string]*shaped{}
		e.cacheBytes = 0
	}
	if bytes <= 32<<20 {
		e.cache[k] = s
		e.cacheBytes += bytes
	}
	return s
}

type operation struct {
	token                 chroma.TokenType
	href                  string
	rect                  image.Rectangle
	run                   *shaping.Output
	style, size, baseline int
	ink                   color.RGBA
	darkInk               color.RGBA
	asset                 image.Image
	destination           string
	fill                  color.Color
	// A text run's place: the paragraph's runes, and which paragraph of
	// which block it is, the same at every width.
	text        []rune
	block, para int
	gutter      bool // A line number in the gutter: drawn muted, and not text to select.
}
type layout struct {
	starts         []int
	glyphs         int
	spent          time.Duration
	e              *engine
	p              *prepared
	width, y, next int
	ops            []operation
	para           int        // Paragraphs laid out of the current block.
	sel            *selection // Text to draw a highlight behind, while rastering.
	numbers        bool       // Lay a gutter of line numbers left of the lines.
	gutter         int        // Its width in page pixels, once measured.
}

func (e *engine) layout(p *prepared, width int) *layout {
	return &layout{e: e, p: p, width: width, y: 50}
}
func (l *layout) text(spans []span, size, x, y, width int) ([]operation, int) {
	// go-text wraps one paragraph at a time. Preserve explicit line breaks,
	// including empty code lines, rather than feeding paragraph separators to it.
	if len(spans) > 32768 {
		panic("typeset block exceeds 32,768 style runs")
	}
	var paragraphs [][]span
	current := []span{}
	for _, sp := range spans {
		parts := strings.Split(sp.text, "\n")
		for i, part := range parts {
			if i > 0 {
				paragraphs = append(paragraphs, current)
				current = nil
			}
			current = append(current, span{text: part, token: sp.token, style: sp.style, ink: sp.ink, darkInk: sp.darkInk, href: sp.href})
		}
	}
	if len(paragraphs) > 0 {
		paragraphs = append(paragraphs, current)
		var ops []operation
		start := y
		for _, p := range paragraphs {
			o, h := l.text(p, size, x, y, width)
			ops = append(ops, o...)
			y += h
		}
		return ops, y - start
	}
	s := l.e.shape(spans, size)
	// Link destinations do not affect shaping. Keep them out of the cache and
	// project the caller's spans onto run offsets, avoiding repeated URL copies.
	hrefs := make([]string, 0, len(s.text))
	for _, sp := range spans {
		for range []rune(sp.text) {
			hrefs = append(hrefs, sp.href)
		}
	}
	para := l.para
	l.para++
	if len(s.text) == 0 {
		return nil, size * 3 / 2
	}
	var wrapper shaping.LineWrapper
	// Wrapping trims whitespace glyph advances in-place. Keep the cached
	// shaping immutable across different widths.
	runsForWrap := append([]shaping.Output(nil), s.runs...)
	for i := range runsForWrap {
		runsForWrap[i].Glyphs = append([]shaping.Glyph(nil), runsForWrap[i].Glyphs...)
	}
	wrapper.Prepare(shaping.WrapConfig{Direction: di.DirectionLTR}, s.text, shaping.NewSliceIterator(runsForWrap))
	var ops []operation
	start := y
	for {
		l.e.check()
		line, done := wrapper.WrapNextLine(max(1, width))
		runs := append(shaping.Line(nil), line.Line...)
		sort.SliceStable(runs, func(i, j int) bool { return runs[i].VisualIndex < runs[j].VisualIndex })
		ascent, descent := 0, 0
		for _, r := range runs {
			ascent = max(ascent, r.LineBounds.Ascent.Ceil())
			descent = max(descent, (-r.LineBounds.Descent).Ceil())
		}
		height := max(size*3/2, ascent+descent)
		baseline := y + (height-ascent-descent)/2 + ascent
		advance := fixed.I(x)
		for _, r := range runs {
			run := r
			l.glyphs += len(r.Glyphs)
			if l.glyphs > 262144 {
				panic("typeset layout exceeds 262,144 glyphs")
			}
			run.Glyphs = append([]shaping.Glyph(nil), r.Glyphs...)
			style := 0
			var ink, darkInk color.RGBA
			href := ""
			var token chroma.TokenType
			if r.Runes.Offset < len(s.styles) {
				style = s.styles[r.Runes.Offset]
				token = s.tokens[r.Runes.Offset]
				href = hrefs[r.Runes.Offset]
				ink = s.inks[r.Runes.Offset]
				darkInk = s.darkInks[r.Runes.Offset]
			}
			ops = append(ops, operation{token: token, rect: image.Rect(advance.Floor(), y, (advance + r.Advance).Ceil(), y+height), run: &run, href: href, size: size, style: style, ink: ink, darkInk: darkInk, baseline: baseline, text: s.text, block: l.next - 1, para: para})
			advance += r.Advance
		}
		y += height
		if done {
			break
		}
	}
	return ops, y - start
}

// gutterWidth is the room the line numbers take: the widest number the
// document has, in the monospace face at the lines' size, and a gap.
func (l *layout) gutterWidth(size int) int {
	if l.gutter == 0 {
		digits := strings.Repeat("0", len(strconv.Itoa(len(l.p.blocks))))
		var advance fixed.Int26_6
		for _, run := range l.e.shape([]span{{text: digits, style: mono}}, size).runs {
			advance += run.Advance
		}
		l.gutter = advance.Ceil() + 20
	}
	return l.gutter
}

// number lays a block's line number in the gutter, at the block's top. It is
// not a paragraph of the block: places in the text stay the same with or
// without numbers.
func (l *layout) number(n, size int) {
	digits := len(strconv.Itoa(len(l.p.blocks)))
	para := l.para
	ops, _ := l.text([]span{{text: fmt.Sprintf("%*d", digits, n), style: mono}}, size, 50, l.y, l.gutter)
	l.para = para
	for i := range ops {
		ops[i].gutter, ops[i].para = true, -1
	}
	l.ops = append(l.ops, ops...)
}

// lineAt is the line at the top of a view: the block at the reading top,
// the page margin below the view's edge, where the first line sits when the
// view is at the top of the page.
func (l *layout) lineAt(top int) int {
	at := 0
	for i, y := range l.starts {
		if y > top+50 {
			break
		}
		at = i
	}
	return at + 1
}

func (l *layout) rect(r image.Rectangle, c color.Color) {
	l.ops = append(l.ops, operation{rect: r, fill: c})
}
func (l *layout) ensure(bottom int) {
	start := time.Now()
	defer func() { l.spent += time.Since(start) }()

	for l.next < len(l.p.blocks) && l.y < bottom {
		l.e.check()
		if l.spent+time.Since(start) > 15*time.Second {
			panic("typeset layout exceeds 15 seconds")
		}
		if len(l.ops) > 32768 || l.y > 1125*MaxPages {
			panic("typeset layout exceeds display-list limit")
		}
		l.block()
	}
}

// block lays out the next block. If that is given up part-way, by a
// cancelled request or a limit, the layout is left as it was before the
// block, so that the next request can take it up again.
func (l *layout) block() {
	starts, ops, y, next, glyphs := len(l.starts), len(l.ops), l.y, l.next, l.glyphs
	defer func() {
		if p := recover(); p != nil {
			l.starts, l.ops, l.y, l.next, l.glyphs = l.starts[:starts], l.ops[:ops], y, next, glyphs
			panic(p)
		}
	}()
	{
		l.starts = append(l.starts, l.y)
		b := l.p.blocks[l.next]
		l.next++
		l.para = 0
		x := 50 + b.indent
		size := b.size
		if size == 0 {
			size = 25
		}
		if l.numbers && l.p.lines {
			x += l.gutterWidth(size)
			l.number(l.next, size)
		}
		width := max(32, l.width-50-x)
		switch b.kind {
		case "rule":
			l.rect(image.Rect(x, l.y, x+width, l.y+2), color.Gray{210})
			l.y += 20
		case "image":
			if b.asset == nil {
				ops, h := l.text(b.spans, 20, x, l.y, width)
				l.ops = append(l.ops, ops...)
				l.y += h + 20
				break
			}
			bounds := b.asset.Bounds()
			w := min(width, bounds.Dx())
			h := max(1, bounds.Dy()*w/bounds.Dx())
			l.ops = append(l.ops, operation{rect: image.Rect(x, l.y, x+w, l.y+h), asset: b.asset, destination: b.destination, href: b.href})
			l.y += h + 20
		case "table":
			l.table(b, x, width)
		default:
			if b.kind == "heading" && l.next > 1 {
				l.y += 16
			}
			inset := 0
			if b.kind == "code" {
				inset = 16
			}
			ops, h := l.text(b.spans, size, x+inset, l.y+inset, max(1, width-inset*2))
			if b.kind == "code" {
				l.rect(image.Rect(x, l.y, x+width, l.y+h+32), color.RGBA{245, 247, 249, 255})
			}
			if b.kind == "quote" {
				l.rect(image.Rect(x-18, l.y, x-14, l.y+h), color.RGBA{158, 172, 185, 255})
			}
			l.ops = append(l.ops, ops...)
			l.y += h + 2*inset
			if b.kind != "line" {
				l.y += 16
			}
		}
		if l.y+50 > 1125*MaxPages || len(l.ops) > 32768 {
			panic("typeset layout exceeds display-list limit")
		}
	}
}
func (l *layout) table(b block, x, width int) {
	if len(b.rows) == 0 {
		return
	}
	cols := len(b.rows[0])
	if cols == 0 {
		return
	}
	weights := make([]int, cols)
	sum := 0
	for _, row := range b.rows {
		for i, cell := range row {
			if i >= cols {
				break
			}
			s := l.e.shape(cell, 25)
			advance := fixed.Int26_6(0)
			for _, r := range s.runs {
				advance += r.Advance
			}
			weights[i] = max(weights[i], min(width/2, max(80, advance.Ceil()+24)))
		}
	}
	for _, w := range weights {
		sum += w
	}
	if sum == 0 {
		return
	}
	for rowIndex, row := range b.rows {
		l.e.check()
		var ops []operation
		xx, height := x, 0
		for i, cell := range row {
			if i >= cols {
				break
			}
			w := width * weights[i] / sum
			if i == cols-1 {
				w = x + width - xx
			}
			cellOps, h := l.text(cell, 25, xx+12, l.y+10, max(1, w-24))
			ops = append(ops, cellOps...)
			height = max(height, h+20)
			xx += w
		}
		if rowIndex == 0 {
			l.rect(image.Rect(x, l.y, x+width, l.y+height), color.RGBA{237, 241, 245, 255})
		}
		l.ops = append(l.ops, ops...)
		l.rect(image.Rect(x, l.y+height-1, x+width, l.y+height), color.Gray{210})
		l.y += height
	}
	l.y += 20
}

// Rasterize a viewport, never an unbounded document image. Layout remains reusable.
func (l *layout) raster(top, height, left, width int, scale float64) *image.RGBA {
	return l.rasterFrame(top, height, left, width, scale, false)
}
func (l *layout) rasterFrame(top, height, left, width int, scale float64, base bool) *image.RGBA {
	w, h := rasterSize(width, height, scale)
	return l.rasterTo(top, height, left, width, w, h, scale, base)
}

// rasterSize is the raster of a width×height layout region at scale.
func rasterSize(width, height int, scale float64) (w, h int) {
	return max(1, int(math.Round(float64(width)*scale))), max(1, int(math.Round(float64(height)*scale)))
}

// rasterTo draws the layout region into a w×h picture, scaling each
// operation by scale. The region normally maps onto the picture; a viewer
// may ask for its picture's exact device size, which can differ from the
// scaled region by a rounding pixel at the bottom or right edge.
func (l *layout) rasterTo(top, height, left, width, w, h int, scale float64, base bool) *image.RGBA {
	l.ensure(top + height)
	if w > MaxPixels/h {
		panic("typeset raster exceeds pixel limit")
	}
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	palette := theme.Get(l.e.theme)
	draw.Draw(out, out.Bounds(), image.NewUniform(palette.Background), image.Point{}, draw.Src)
	for i, op := range l.ops {
		l.e.check()
		r := op.rect.Sub(image.Pt(left, top))
		r = image.Rect(int(math.Round(float64(r.Min.X)*scale)), int(math.Round(float64(r.Min.Y)*scale)), int(math.Round(float64(r.Max.X)*scale)), int(math.Round(float64(r.Max.Y)*scale)))
		if !r.Overlaps(out.Bounds()) {
			continue
		}
		switch {
		case op.fill != nil:
			draw.Draw(out, r, &image.Uniform{C: typesetFill(op.fill, palette)}, image.Point{}, draw.Src)
		case op.asset != nil:
			if base && l.p.animations[op.destination] != nil {
				continue
			}
			xdraw.CatmullRom.Scale(out, r, op.asset, op.asset.Bounds(), draw.Over, nil)
		case op.run != nil:
			ink := palette.Foreground
			if op.ink.A != 0 {
				ink = op.ink
			}
			if theme.Dark(l.e.theme) && op.darkInk.A != 0 {
				ink = op.darkInk
			}
			if op.token != 0 {
				ink = theme.Ink(l.e.theme, op.token)
			}
			if op.ink == theme.Get("light").Accent && op.darkInk == theme.Get("dark").Accent {
				ink = palette.Accent
			}
			if op.gutter {
				ink = palette.Muted
			}
			if x0, x1, ok := l.sel.span(i, &op); ok {
				hr := image.Rect(int(math.Round(float64(x0-left)*scale)), r.Min.Y, int(math.Round(float64(x1-left)*scale)), r.Max.Y)
				draw.Draw(out, hr, image.NewUniform(palette.Selection), image.Point{}, draw.Src)
			}
			baseline := int(math.Round(float64(op.baseline-top) * scale))
			l.e.drawRun(out, op.run, op.size, scale, r.Min.X, baseline, r.Dy(), ink)
			if op.style&strike != 0 {
				y := int(math.Round(float64(op.baseline-top-op.size/3) * scale))
				draw.Draw(out, image.Rect(r.Min.X, y, r.Max.X, y+1), image.NewUniform(ink), image.Point{}, draw.Src)
			}
		}
	}
	return out
}

// drawRun draws a shaped run with its origin at x on the baseline, each glyph
// from a cached mask blended in ink. Positions follow go-text's own: offsets
// and advances in 26.6 units scaled by the raster scale, accumulated in
// float32. A run holding a glyph that is not an outline is drawn by go-text.
func (e *engine) drawRun(out *image.RGBA, run *shaping.Output, size int, scale float64, x, baseline, height int, ink color.RGBA) {
	for _, g := range run.Glyphs {
		if _, ok := run.Face.GlyphData(g.GlyphID).(font.GlyphOutline); !ok {
			e.drawRunDirect(out, run, size, scale, x, baseline, height, ink)
			return
		}
	}
	pixScale := float32(scale)
	src := image.NewUniform(ink)
	pos := float32(x)
	for _, g := range run.Glyphs {
		gx := pos + float32(g.XOffset)/64*pixScale
		gy := float32(baseline) - float32(g.YOffset)/64*pixScale
		pos += float32(g.Advance) / 64 * pixScale
		ix, fx := quantize(gx)
		iy, fy := quantize(gy)
		m := e.glyph(glyphKey{run.Face, g.GlyphID, size, pixScale, fx, fy}, g, run.Face, height)
		if m == nil {
			continue
		}
		b := m.mask.Bounds()
		dst := image.Rect(ix+m.offset.X, iy+m.offset.Y, ix+m.offset.X+b.Dx(), iy+m.offset.Y+b.Dy())
		draw.DrawMask(out, dst, src, image.Point{}, m.mask, b.Min, draw.Over)
	}
}

// quantize splits a position into its integer part and quarter within it.
func quantize(v float32) (int, uint8) {
	i := int(math.Floor(float64(v)))
	q := int(math.Round(float64(v-float32(i)) * glyphQuanta))
	if q == glyphQuanta {
		return i + 1, 0
	}
	return i, uint8(q)
}

// glyph is the rendered mask for key, drawing it with go-text on a miss: the
// glyph alone, in white on a clear tile at its quarter-pixel position, kept
// trimmed to its ink. The cache is emptied when it passes its budget.
func (e *engine) glyph(key glyphKey, g shaping.Glyph, face *font.Face, height int) *glyphMask {
	if m, ok := e.glyphs[key]; ok {
		e.glyphHits++
		return m
	}
	e.glyphMisses++
	pad := max(1, int(math.Ceil(float64(key.size)*float64(key.scale))))
	advance := int(math.Ceil(float64(g.Advance) / 64 * float64(key.scale)))
	tile := image.NewRGBA(image.Rect(0, 0, max(1, advance)+2*pad, max(1, height)+2*pad))
	e.renderer.FontSize, e.renderer.PixScale, e.renderer.Color = float32(key.size), key.scale, color.RGBA{255, 255, 255, 255}
	single := shaping.Output{Face: face, Glyphs: []shaping.Glyph{g}}
	// The quarter-pixel position goes through the glyph's own offsets, which
	// go-text scales by PixScale, so divide it out here.
	single.Glyphs[0].XOffset = fixed.Int26_6(math.Round(float64(g.XOffset) + float64(key.fx)/glyphQuanta/float64(key.scale)*64))
	single.Glyphs[0].YOffset = fixed.Int26_6(math.Round(float64(g.YOffset) - float64(key.fy)/glyphQuanta/float64(key.scale)*64))
	e.renderer.DrawShapedRunAt(single, tile, pad, pad)
	bounds := image.Rectangle{Min: image.Pt(tile.Rect.Max.X, tile.Rect.Max.Y)}
	for y := 0; y < tile.Rect.Dy(); y++ {
		for x := 0; x < tile.Rect.Dx(); x++ {
			if tile.Pix[y*tile.Stride+x*4+3] != 0 {
				bounds = bounds.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	var m *glyphMask
	if !bounds.Empty() {
		mask := image.NewAlpha(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
		for y := 0; y < bounds.Dy(); y++ {
			for x := 0; x < bounds.Dx(); x++ {
				mask.Pix[y*mask.Stride+x] = tile.Pix[(bounds.Min.Y+y)*tile.Stride+(bounds.Min.X+x)*4+3]
			}
		}
		m = &glyphMask{mask: mask, offset: bounds.Min.Sub(image.Pt(pad, pad))}
		if e.glyphBytes+len(mask.Pix) > e.glyphBudget {
			e.glyphs = map[glyphKey]*glyphMask{}
			e.glyphBytes = 0
		}
		e.glyphBytes += len(mask.Pix)
	}
	e.glyphs[key] = m
	return m
}

// drawRunDirect draws a run with go-text, on a transparent tile the run's size.
func (e *engine) drawRunDirect(out *image.RGBA, run *shaping.Output, size int, scale float64, x, baseline, height int, ink color.RGBA) {
	e.renderer.FontSize, e.renderer.PixScale, e.renderer.Color = float32(size), float32(scale), ink
	pad := max(1, int(math.Ceil(float64(size)*scale)))
	width := 0
	for _, g := range run.Glyphs {
		width += int(math.Ceil(float64(g.Advance) / 64 * scale))
	}
	tile := image.NewRGBA(image.Rect(0, 0, max(1, width)+2*pad, max(1, height)+2*pad))
	e.renderer.DrawShapedRunAt(*run, tile, pad, pad)
	draw.Draw(out, tile.Bounds().Add(image.Pt(x-pad, baseline-pad)), tile, image.Point{}, draw.Over)
}

// The loader serializes this engine; each job supplies its own cancellation.
func (e *engine) check() {
	if e.ctx != nil {
		if err := e.ctx.Err(); err != nil {
			panic(err)
		}
	}
}

func typesetError(p any) error {
	if err, ok := p.(error); ok {
		return err
	}
	return fmt.Errorf("typesetting: %v", p)
}

// Display-list fills are semantic colors from the fixed light palette. Resolve
// them at paint time so changing theme never reshapes or rewraps text.
func typesetFill(c color.Color, p theme.Palette) color.Color {
	switch color.RGBAModel.Convert(c).(color.RGBA) {
	case color.RGBA{210, 210, 210, 255}:
		return p.Border
	case color.RGBA{245, 247, 249, 255}:
		return p.Code
	case color.RGBA{237, 241, 245, 255}:
		return p.Header
	case color.RGBA{158, 172, 185, 255}:
		return p.Muted
	}
	return c
}

func imageLink(im *ast.Image) string {
	if link, ok := im.Parent().(*ast.Link); ok {
		return string(link.Destination)
	}
	return ""
}
