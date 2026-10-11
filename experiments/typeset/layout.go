// This experiment deliberately supports a bounded Markdown subset, not HTML/CSS.
package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/go-text/render"
	"github.com/go-text/typesetting/di"
	"github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/shaping"
	"github.com/yuin/goldmark/ast"
	ext "github.com/yuin/goldmark/extension/ast"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/math/fixed"
	"nimbleterminal.dev/gloss/internal/document"
)

const (
	bold   = 1
	italic = 2
	mono   = 4
	strike = 8
)

type span struct {
	text  string
	style int
	ink   color.RGBA // zero means the ordinary text color
}
type block struct {
	kind         string
	spans        []span
	rows         [][][]span
	asset        image.Image
	indent, size int
}
type prepared struct {
	blocks []block
	images map[string]image.Image
}

func boundedRead(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("input exceeds %d bytes", limit)
	}
	return b, nil
}

// Preflight every image before allocating any decoded pixels. Loading is offline.
// The spike handles PNG/JPEG; other assets deliberately use a placeholder.
func prepare(path string) (*prepared, error) {
	data, err := boundedRead(path, document.MaxMarkdownBytes)
	if err != nil {
		return nil, err
	}
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("invalid UTF-8")
	}
	root := document.MarkdownTree(data)
	raw := map[string][]byte{}
	refs, pixels, total, depth := 0, 0, 0, 0
	err = ast.Walk(root, func(n ast.Node, enter bool) (ast.WalkStatus, error) {
		if !enter {
			depth--
			return ast.WalkContinue, nil
		}
		depth++
		if depth > 128 {
			return ast.WalkStop, fmt.Errorf("nesting exceeds 128")
		}
		im, ok := n.(*ast.Image)
		if !ok {
			return ast.WalkContinue, nil
		}
		refs++
		if refs > 32 {
			return ast.WalkStop, fmt.Errorf("more than 32 image references")
		}
		dest := string(im.Destination)
		if _, ok := raw[dest]; ok {
			return ast.WalkContinue, nil
		}
		raw[dest] = nil
		u, e := url.Parse(dest)
		if e != nil || u.Scheme != "" || u.Host != "" || strings.HasPrefix(dest, "//") {
			return ast.WalkContinue, nil
		}
		name, e := url.PathUnescape(u.Path)
		if e != nil {
			return ast.WalkContinue, nil
		}
		if !filepath.IsAbs(name) {
			name = filepath.Join(filepath.Dir(path), name)
		}
		b, e := boundedRead(name, 128<<20)
		if os.IsNotExist(e) {
			return ast.WalkContinue, nil
		}
		if e != nil {
			return ast.WalkStop, e
		}
		total += len(b)
		if total > 128<<20 {
			return ast.WalkStop, fmt.Errorf("image inputs exceed 128 MiB")
		}
		cfg, format, e := image.DecodeConfig(bytes.NewReader(b))
		if e != nil || (format != "png" && format != "jpeg") {
			return ast.WalkContinue, nil
		}
		area := int64(cfg.Width) * int64(cfg.Height)
		if area < 1 || area > 16<<20 || int64(pixels)+area > 16<<20 {
			return ast.WalkStop, fmt.Errorf("images exceed 16 Mi pixels")
		}
		pixels += int(area)
		raw[dest] = b
		return ast.WalkContinue, nil
	})
	if err != nil {
		return nil, err
	}
	p := &prepared{images: map[string]image.Image{}}
	for dest, b := range raw {
		if b != nil {
			im, _, e := image.Decode(bytes.NewReader(b))
			if e != nil {
				return nil, e
			}
			p.images[dest] = im
		}
	}
	p.walk(root, data, 0)
	return p, nil
}

func inline(n ast.Node, data []byte, style int) []span {
	var out []span
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		switch v := c.(type) {
		case *ast.Text:
			s := string(v.Segment.Value(data))
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
		switch v := c.(type) {
		case *ast.Heading:
			p.blocks = append(p.blocks, block{kind: "heading", spans: inline(v, data, bold), size: []int{50, 46, 42, 38, 33, 29}[v.Level-1], indent: indent})
		case *ast.Paragraph, *ast.TextBlock:
			if c.ChildCount() == 1 {
				if im, ok := c.FirstChild().(*ast.Image); ok {
					p.blocks = append(p.blocks, block{kind: "image", asset: p.images[string(im.Destination)], spans: []span{{text: "[image unavailable: " + string(im.Destination) + "]", style: italic}}, indent: indent})
					continue
				}
			}
			p.blocks = append(p.blocks, block{kind: "paragraph", spans: inline(c, data, 0), indent: indent})
		case *ast.FencedCodeBlock, *ast.CodeBlock:
			var b strings.Builder
			for i := 0; i < c.Lines().Len(); i++ {
				s := c.Lines().At(i)
				b.Write(s.Value(data))
			}
			language := ""
			if fence, ok := c.(*ast.FencedCodeBlock); ok {
				language = string(fence.Language(data))
			}
			p.blocks = append(p.blocks, block{kind: "code", spans: highlightCode(strings.TrimSuffix(b.String(), "\n"), language, codeTheme), size: 25, indent: indent})
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

type faceMap struct{ face *font.Face }

func (f faceMap) ResolveFace(rune) *font.Face { return f.face }

type shaped struct {
	text   []rune
	runs   []shaping.Output
	styles []int
	inks   []color.RGBA
	size   int
}
type engine struct {
	faces      [8]*font.Face
	shaper     shaping.HarfbuzzShaper
	segmenter  shaping.Segmenter
	cache      map[string]*shaped
	cacheBytes int
	renderer   render.Renderer
}

func newEngine(fontDir string) (*engine, error) {
	e := &engine{cache: map[string]*shaped{}}
	for i := 0; i < 8; i++ {
		family := "Go"
		if i&mono != 0 {
			family = "GoMono"
		}
		variant := []string{"Regular", "Bold", "Italic", "BoldItalic"}[i&3]
		b, err := boundedRead(filepath.Join(fontDir, family+"-"+variant+".ttf"), 2<<20)
		if err != nil {
			return nil, err
		}
		e.faces[i], err = font.ParseTTF(bytes.NewReader(b))
		if err != nil {
			return nil, err
		}
	}
	return e, nil
}
func (e *engine) shape(spans []span, size int) *shaped {
	var key strings.Builder
	fmt.Fprintf(&key, "%d/", size)
	for _, s := range spans {
		fmt.Fprintf(&key, "%d:%v:%d:%s", s.style, s.ink, len(s.text), s.text)
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
			s.inks = append(s.inks, sp.ink)
		}
	}
	offset := 0
	for _, sp := range spans {
		end := offset + utf8.RuneCountInString(sp.text)
		if end == offset {
			continue
		}
		in := shaping.Input{Text: s.text, RunStart: offset, RunEnd: end, Direction: di.DirectionLTR, Face: e.faces[sp.style&7], Size: fixed.I(size)}
		for _, run := range e.segmenter.Split(in, faceMap{in.Face}) {
			s.runs = append(s.runs, e.shaper.Shape(run))
		}
		offset = end
	}
	// Per-engine bounded cache; eviction is deliberately simple for the spike.
	bytes := len(s.text) * 20
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
	rect                  image.Rectangle
	run                   *shaping.Output
	style, size, baseline int
	ink                   color.RGBA
	asset                 image.Image
	fill                  color.Color
}
type layout struct {
	e              *engine
	p              *prepared
	width, y, next int
	ops            []operation
}

func (e *engine) layout(p *prepared, width int) *layout {
	return &layout{e: e, p: p, width: width, y: 50}
}
func (l *layout) text(spans []span, size, x, y, width int) ([]operation, int) {
	// go-text wraps one paragraph at a time. Preserve explicit line breaks,
	// including empty code lines, rather than feeding paragraph separators to it.
	var paragraphs [][]span
	current := []span{}
	for _, sp := range spans {
		parts := strings.Split(sp.text, "\n")
		for i, part := range parts {
			if i > 0 {
				paragraphs = append(paragraphs, current)
				current = nil
			}
			current = append(current, span{text: part, style: sp.style, ink: sp.ink})
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
			run.Glyphs = append([]shaping.Glyph(nil), r.Glyphs...)
			style := 0
			var ink color.RGBA
			if r.Runes.Offset < len(s.styles) {
				style = s.styles[r.Runes.Offset]
				ink = s.inks[r.Runes.Offset]
			}
			ops = append(ops, operation{rect: image.Rect(advance.Floor(), y, (advance + r.Advance).Ceil(), y+height), run: &run, size: size, style: style, ink: ink, baseline: baseline})
			advance += r.Advance
		}
		y += height
		if done {
			break
		}
	}
	return ops, y - start
}
func (l *layout) rect(r image.Rectangle, c color.Color) {
	l.ops = append(l.ops, operation{rect: r, fill: c})
}
func (l *layout) ensure(bottom int) {
	for l.next < len(l.p.blocks) && l.y < bottom {
		b := l.p.blocks[l.next]
		l.next++
		x := 50 + b.indent
		width := max(32, l.width-50-x)
		size := b.size
		if size == 0 {
			size = 25
		}
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
			l.ops = append(l.ops, operation{rect: image.Rect(x, l.y, x+w, l.y+h), asset: b.asset})
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
			l.y += h + 2*inset + 16
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
func (l *layout) viewport(top, height int) *image.RGBA {
	l.ensure(top + height)
	out := image.NewRGBA(image.Rect(0, 0, l.width, height))
	draw.Draw(out, out.Bounds(), image.White, image.Point{}, draw.Src)
	for _, op := range l.ops {
		r := op.rect.Sub(image.Pt(0, top))
		if !r.Overlaps(out.Bounds()) {
			continue
		}
		switch {
		case op.fill != nil:
			draw.Draw(out, r, &image.Uniform{C: op.fill}, image.Point{}, draw.Src)
		case op.asset != nil:
			xdraw.CatmullRom.Scale(out, r, op.asset, op.asset.Bounds(), draw.Over, nil)
		case op.run != nil:
			l.e.renderer.FontSize = float32(op.size)
			l.e.renderer.Color = color.RGBA{35, 42, 50, 255}
			if op.ink.A != 0 {
				l.e.renderer.Color = op.ink
			}
			// render allocates its rasterizer to the destination's dimensions.
			// Give each run a small transparent tile, not the entire viewport.
			pad := op.size
			tile := image.NewRGBA(image.Rect(0, 0, max(1, r.Dx())+2*pad, r.Dy()+2*pad))
			l.e.renderer.DrawShapedRunAt(*op.run, tile, pad, pad+op.baseline-op.rect.Min.Y)
			dst := tile.Bounds().Add(r.Min.Sub(image.Pt(pad, pad)))
			draw.Draw(out, dst, tile, image.Point{}, draw.Over)
			if op.style&strike != 0 {
				y := op.baseline - top - op.size/3
				draw.Draw(out, image.Rect(r.Min.X, y, r.Max.X, y+1), image.NewUniform(color.Gray{70}), image.Point{}, draw.Src)
			}
		}
	}
	return out
}
