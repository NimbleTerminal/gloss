package document

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/yuin/goldmark/ast"
	ext "github.com/yuin/goldmark/extension/ast"
)

// Normalize extensions into the same standard nodes for terminal text and
// typesetting. Generated labels use String nodes; source bytes stay untouched.
func normalizeMarkdownExtensions(root ast.Node, source []byte, check func()) {
	var nodes []ast.Node
	refs := map[int]int{}
	_ = ast.Walk(root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		check()
		if entering {
			nodes = append(nodes, n)
			if ref, ok := n.(*ext.FootnoteLink); ok {
				refs[ref.Index] = ref.RefCount
			}
		}
		return ast.WalkContinue, nil
	})
	for _, n := range nodes {
		check()
		switch v := n.(type) {
		case *ext.FootnoteLink:
			p := v.Parent()
			p.ReplaceChild(p, v, footnoteLink("fn", v.Index, v.RefIndex, fmt.Sprintf("[%d]", v.Index)))
		case *ext.FootnoteBacklink:
			v.Parent().RemoveChild(v.Parent(), v)
		case *ext.FootnoteList:
			p := v.Parent()
			heading := ast.NewHeading(2)
			heading.AppendChild(heading, ast.NewString([]byte("Footnotes")))
			p.InsertBefore(p, v, heading)
			list := ast.NewList('.')
			list.Start, list.IsTight = 1, false
			for note := v.FirstChild(); note != nil; note = note.NextSibling() {
				check()
				index := note.(*ext.Footnote).Index
				first, ok := note.FirstChild().(*ast.Paragraph)
				if !ok {
					first = ast.NewParagraph()
					note.InsertBefore(note, note.FirstChild(), first)
				}
				for ref := refs[index] - 1; ref >= 0; ref-- {
					label := "[back]"
					if refs[index] > 1 {
						label = fmt.Sprintf("[back %d]", ref+1)
					}
					first.InsertBefore(first, first.FirstChild(), ast.NewString([]byte(" ")))
					first.InsertBefore(first, first.FirstChild(), footnoteLink("ref", index, ref, label))
				}
				item := ast.NewListItem(0)
				for note.FirstChild() != nil {
					item.AppendChild(item, note.FirstChild())
				}
				list.AppendChild(list, item)
			}
			p.ReplaceChild(p, v, list)
		case *ast.Blockquote:
			normalizeAlert(v, source)
		}
	}
}

func footnoteLink(kind string, note, ref int, label string) *ast.Link {
	link := ast.NewLink()
	link.Destination = []byte(fmt.Sprintf("gloss:%s:%d:%d", kind, note, ref))
	link.AppendChild(link, ast.NewString([]byte(label)))
	return link
}

// FootnotePeer pairs each generated reference with its return link. Only
// these document-local targets are handled internally, never by the OS.
func FootnotePeer(address string) (string, bool) {
	p := strings.Split(address, ":")
	if len(p) != 4 || p[0] != "gloss" || (p[1] != "fn" && p[1] != "ref") {
		return "", false
	}
	note, err := strconv.Atoi(p[2])
	if err != nil || note < 1 {
		return "", false
	}
	ref, err := strconv.Atoi(p[3])
	if err != nil || ref < 0 {
		return "", false
	}
	kind := "fn"
	if p[1] == "fn" {
		kind = "ref"
	}
	return fmt.Sprintf("gloss:%s:%d:%d", kind, note, ref), true
}

func normalizeAlert(quote *ast.Blockquote, source []byte) {
	p, ok := quote.FirstChild().(*ast.Paragraph)
	if !ok || p.Lines().Len() == 0 {
		return
	}
	line := p.Lines().At(0)
	label := ""
	switch strings.TrimSpace(string(line.Value(source))) {
	case "[!NOTE]":
		label = "Note"
	case "[!TIP]":
		label = "Tip"
	case "[!IMPORTANT]":
		label = "Important"
	case "[!WARNING]":
		label = "Warning"
	case "[!CAUTION]":
		label = "Caution"
	default:
		return
	}
	// Only a standalone marker on the first line is special. Escaped markers,
	// inline code, and similarly named ordinary quotes retain their source.
	for n := p.FirstChild(); n != nil; {
		t, ok := n.(*ast.Text)
		if !ok || t.Segment.Start >= line.Stop {
			break
		}
		next := n.NextSibling()
		p.RemoveChild(p, n)
		n = next
	}
	title := ast.NewParagraph()
	strong := ast.NewEmphasis(2)
	strong.AppendChild(strong, ast.NewString([]byte(label)))
	title.AppendChild(title, strong)
	quote.InsertBefore(quote, p, title)
	if p.ChildCount() == 0 {
		quote.RemoveChild(quote, p)
	}
}
