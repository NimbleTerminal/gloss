package document

import (
	"context"
	"errors"
	"fmt"
	"image"
	"strings"

	"github.com/alecthomas/chroma/v2/styles"
	emojiast "github.com/yuin/goldmark-emoji/ast"
	"github.com/yuin/goldmark/ast"
	ext "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/util"
	"nimbleterminal.dev/nttypeset"
)

// blockWalk turns a goldmark tree into the blocks nttypeset lays out.
type blockWalk struct {
	ctx      context.Context
	blocks   []nttypeset.Block
	images   map[string]image.Image
	animated map[string]bool // Destinations the viewer animates.
}

// walkLimit is a limit the walk gives up at; buildBlocks returns it.
type walkLimit string

// buildBlocks walks a parsed Markdown tree into an nttypeset document. Images
// are looked up by destination; animated destinations are left for the viewer
// to composite. A cancelled ctx or a walk limit is returned as an error.
func buildBlocks(ctx context.Context, root ast.Node, source []byte, images map[string]image.Image, animated map[string]bool) (doc *nttypeset.Document, err error) {
	w := &blockWalk{ctx: ctx, images: images, animated: animated}
	defer func() {
		if p := recover(); p != nil {
			switch v := p.(type) {
			case walkLimit:
				doc, err = nil, errors.New(string(v))
			case error:
				doc, err = nil, v
			default:
				panic(p)
			}
		}
	}()
	w.walk(root, source, 0)
	return &nttypeset.Document{Blocks: w.blocks}, nil
}

func inline(n ast.Node, data []byte, style nttypeset.Style) []nttypeset.Span {
	var out []nttypeset.Span
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		switch v := c.(type) {
		case *ast.Text:
			raw := v.Segment.Value(data)
			if style&nttypeset.Mono == 0 {
				raw = util.ResolveEntityNames(util.ResolveNumericReferences(util.UnescapePunctuations(raw)))
			}
			s := string(raw)
			if v.HardLineBreak() {
				s += "\n"
			} else if v.SoftLineBreak() {
				s += " "
			}
			out = append(out, nttypeset.Span{Text: s, Style: style})
		case *ast.String:
			out = append(out, nttypeset.Span{Text: string(v.Value), Style: style})
		case *emojiast.Emoji:
			out = append(out, nttypeset.Span{Text: string(v.Value.Unicode), Style: style})
		case *ast.Emphasis:
			flag := nttypeset.Italic
			if v.Level == 2 {
				flag = nttypeset.Bold
			}
			out = append(out, inline(v, data, style|flag)...)
		case *ast.CodeSpan:
			out = append(out, inline(v, data, style|nttypeset.Mono)...)
		case *ext.Strikethrough:
			out = append(out, inline(v, data, style|nttypeset.Strike)...)
		case *ext.TaskCheckBox:
			s := "[ ] "
			if v.IsChecked {
				s = "[x] "
			}
			out = append(out, nttypeset.Span{Text: s, Style: style | nttypeset.Mono})
		case *ast.Link:
			linked := inline(v, data, style)
			for i := range linked {
				linked[i].Href = string(v.Destination)
				linked[i].Role = nttypeset.RoleAccent
			}
			out = append(out, linked...)
		case *ast.AutoLink:
			out = append(out, nttypeset.Span{Text: string(v.Label(data)), Href: string(v.URL(data)), Style: style, Role: nttypeset.RoleAccent})
		case *ast.Image:
			out = append(out, nttypeset.Span{Text: "[image: " + string(v.Destination) + "]", Style: style})
		default:
			out = append(out, inline(c, data, style)...)
		}
	}
	return out
}

func (w *blockWalk) walk(n ast.Node, data []byte, indent int) {
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if w.ctx != nil {
			if err := w.ctx.Err(); err != nil {
				panic(err)
			}
		}
		if len(w.blocks) > 32768 {
			panic(walkLimit("typeset document exceeds 32,768 blocks"))
		}
		switch v := c.(type) {
		case *ast.Heading:
			heading := inline(v, data, nttypeset.Bold)
			for i := range heading {
				heading[i].Role = nttypeset.RoleAccent
			}
			w.blocks = append(w.blocks, nttypeset.Block{Kind: nttypeset.Heading, Spans: heading, Size: []int{50, 46, 42, 38, 33, 29}[v.Level-1], Indent: indent})
		case *ast.Paragraph, *ast.TextBlock:
			if c.ChildCount() == 1 {
				if im := standaloneImage(c.FirstChild()); im != nil {
					dest := string(im.Destination)
					bounds := MarkdownImageLayout(im)
					w.blocks = append(w.blocks, nttypeset.Block{
						Kind: nttypeset.KindImage,
						Image: &nttypeset.Image{
							Key:      dest,
							Still:    w.images[dest],
							Bounds:   nttypeset.Bounds{Width: bounds.Width, Height: bounds.Height, Align: bounds.Align},
							Animated: w.animated[dest],
						},
						Href:   imageLink(im),
						Spans:  []nttypeset.Span{{Text: "[image unavailable: " + dest + "]", Style: nttypeset.Italic}},
						Indent: indent,
					})
					continue
				}
			}
			w.blocks = append(w.blocks, nttypeset.Block{Kind: nttypeset.Paragraph, Spans: inline(c, data, 0), Indent: indent})
		case *ast.FencedCodeBlock, *ast.CodeBlock:
			if c.Lines().Len() > 8192 {
				panic(walkLimit("typeset code block exceeds 8,192 lines"))
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
			w.blocks = append(w.blocks, nttypeset.Block{Kind: nttypeset.Code, Spans: nttypeset.Highlight(w.ctx, strings.TrimSuffix(b.String(), "\n"), language, styles.Get("github")), Size: 25, Indent: indent})
		case *ast.Blockquote:
			start := len(w.blocks)
			w.walk(c, data, indent+24)
			for i := start; i < len(w.blocks); i++ {
				w.blocks[i].Quotes = append(w.blocks[i].Quotes, nttypeset.Quote{Indent: indent + 24, Last: i == len(w.blocks)-1})
				if w.blocks[i].Kind == nttypeset.Paragraph {
					w.blocks[i].Kind = nttypeset.KindQuote
				}
			}
		case *ast.List:
			index := v.Start
			if index < 1 {
				index = 1
			}
			for li := v.FirstChild(); li != nil; li = li.NextSibling() {
				start := len(w.blocks)
				w.walk(li, data, indent+24)
				if start < len(w.blocks) {
					marker := "• "
					if v.IsOrdered() {
						marker = fmt.Sprintf("%d. ", index)
					}
					w.blocks[start].Spans = append([]nttypeset.Span{{Text: marker}}, w.blocks[start].Spans...)
				}
				index++
			}
		case *ext.Table:
			b := nttypeset.Block{Kind: nttypeset.Table, Indent: indent}
			for row := v.FirstChild(); row != nil; row = row.NextSibling() {
				var cells [][]nttypeset.Span
				var style nttypeset.Style
				if _, ok := row.(*ext.TableHeader); ok {
					style = nttypeset.Bold
				}
				for cell := row.FirstChild(); cell != nil; cell = cell.NextSibling() {
					cells = append(cells, inline(cell, data, style))
				}
				b.Rows = append(b.Rows, cells)
			}
			w.blocks = append(w.blocks, b)
		case *ast.ThematicBreak:
			w.blocks = append(w.blocks, nttypeset.Block{Kind: nttypeset.Rule})
		case *ast.HTMLBlock: // no raw HTML layout
		default:
			w.walk(c, data, indent)
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

func imageLink(im *ast.Image) string {
	if link, ok := im.Parent().(*ast.Link); ok {
		return string(link.Destination)
	}
	return ""
}
