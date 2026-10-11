package document

import (
	"bytes"
	"net/url"
	"strconv"
	"strings"

	"github.com/yuin/goldmark/ast"
	"golang.org/x/net/html"
)

// ImageLayout describes the small README HTML subset, not a CSS layout engine.
// Dimensions are positive pixel bounds; images retain their aspect ratio.
type ImageLayout struct {
	Width, Height int
	Align         string
}

func MarkdownImageLayout(im *ast.Image) ImageLayout {
	if v, ok := im.AttributeString("gloss-image-layout"); ok {
		if v, ok := v.(ImageLayout); ok {
			return v
		}
	}
	return ImageLayout{}
}

// Fit applies HTML bounds without stretching or enlarging the original image.
func (s ImageLayout) Fit(w, h int) (int, int) {
	if s.Width > 0 && w > s.Width {
		h = max(1, h*s.Width/w)
		w = s.Width
	}
	if s.Height > 0 && h > s.Height {
		w = max(1, w*s.Height/h)
		h = s.Height
	}
	return w, h
}
func (s ImageLayout) Offset(available, width int) int {
	switch s.Align {
	case "center":
		return max(0, (available-width)/2)
	case "right":
		return max(0, available-width)
	}
	return 0
}

type readmeHTML struct{ href, align, blocked string }

func safeReadmeLink(s string) string {
	u, err := url.Parse(s)
	if err != nil || (u.Scheme != "" && u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "mailto") {
		return ""
	}
	return cleanMarkdownSource([]byte(s))
}

// Only explicit HTML image tokens become assets. Attributes such as style,
// srcset and event handlers are never interpreted, and active subtrees are dropped.
func (s *readmeHTML) read(data []byte, check func(), emit func(ast.Node)) {
	z := html.NewTokenizer(bytes.NewReader(data))
	for {
		check()
		kind := z.Next()
		if kind == html.ErrorToken {
			return
		}
		t := z.Token()
		if s.blocked != "" {
			if kind == html.EndTagToken && t.Data == s.blocked {
				s.blocked = ""
			}
			continue
		}
		if kind == html.EndTagToken {
			if t.Data == "a" {
				s.href = ""
			}
			if t.Data == "p" || t.Data == "div" {
				s.align = ""
			}
			continue
		}
		if kind != html.StartTagToken && kind != html.SelfClosingTagToken {
			continue
		}
		switch t.Data {
		case "script", "style", "iframe", "object", "svg", "template":
			s.blocked = t.Data
			continue
		}
		attrs := map[string]string{}
		for _, a := range t.Attr {
			if a.Namespace == "" {
				if _, exists := attrs[a.Key]; !exists {
					attrs[a.Key] = a.Val
				}
			}
		}
		switch t.Data {
		case "p", "div":
			s.align = readmeAlign(attrs["align"])
		case "a":
			s.href = safeReadmeLink(attrs["href"])
		case "img":
			link := ast.NewLink()
			link.Destination = []byte(cleanMarkdownSource([]byte(attrs["src"])))
			im := ast.NewImage(link)
			im.AppendChild(im, ast.NewString([]byte(cleanMarkdownSource([]byte(attrs["alt"])))))
			align := readmeAlign(attrs["align"])
			if align == "" {
				align = s.align
			}
			im.SetAttributeString("gloss-image-layout", ImageLayout{Width: readmeDimension(attrs["width"]), Height: readmeDimension(attrs["height"]), Align: align})
			if s.href != "" {
				a := ast.NewLink()
				a.Destination = []byte(s.href)
				a.AppendChild(a, im)
				emit(a)
			} else {
				emit(im)
			}
		}
	}
}
func readmeAlign(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "left":
		return "left"
	case "center":
		return "center"
	case "right":
		return "right"
	}
	return ""
}
func readmeDimension(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n < 1 || n > 4096 {
		return 0
	}
	return n
}

func normalizeMarkdownHTML(root ast.Node, source []byte, check func()) {
	var nodes []ast.Node
	_ = ast.Walk(root, func(n ast.Node, enter bool) (ast.WalkStatus, error) {
		check()
		if enter {
			switch n.(type) {
			case *ast.HTMLBlock, *ast.RawHTML:
				nodes = append(nodes, n)
			}
		}
		return ast.WalkContinue, nil
	})
	var state readmeHTML
	var parent ast.Node
	for _, n := range nodes {
		check()
		p := n.Parent()
		switch v := n.(type) {
		case *ast.HTMLBlock:
			var b bytes.Buffer
			for i := 0; i < v.Lines().Len(); i++ {
				line := v.Lines().At(i)
				b.Write(line.Value(source))
			}
			if v.HasClosure() {
				b.Write(v.ClosureLine.Value(source))
			}
			s := readmeHTML{}
			s.read(b.Bytes(), check, func(im ast.Node) { para := ast.NewParagraph(); para.AppendChild(para, im); p.InsertBefore(p, n, para) })
			// Keep other raw HTML opaque, as before. The image nodes are safe and
			// participate in exactly the same discovery/layout as Markdown images.
			p.RemoveChild(p, n)
		case *ast.RawHTML:
			if parent != p {
				state = readmeHTML{}
				parent = p
			}
			var b bytes.Buffer
			for i := 0; i < v.Segments.Len(); i++ {
				seg := v.Segments.At(i)
				b.Write(seg.Value(source))
			}
			state.read(b.Bytes(), check, func(im ast.Node) { p.InsertBefore(p, n, im) })
			p.RemoveChild(p, n)
		}
	}
}
