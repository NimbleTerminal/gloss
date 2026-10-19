package document

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/png"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/yuin/goldmark/ast"
	mermaid "github.com/zkrebbekx/go-mermaid"
)

// Mermaid diagrams are drawn by go-mermaid, which lays a diagram out and
// writes SVG; the SVG is then rasterized like any other. Both run in the
// parser worker, under its deadline, because diagram source is untrusted and
// graph layout is not bounded by the input size alone.

const (
	mermaidAttribute = "gloss-mermaid"
	mermaidFence     = "gloss-mermaid-fence"
	mermaidPrefix    = "mermaid:"
	// MaxMermaidBytes bounds one diagram's source.
	MaxMermaidBytes = 64 << 10
	// go-mermaid lays out at 96 pixels per inch; Markdown pages are laid out
	// at 150. Diagrams are rasterized at twice their page size for zoom.
	mermaidPageScale   = 150.0 / 96.0
	mermaidRasterScale = 2
)

var mermaidSize = regexp.MustCompile(`^<svg[^>]*?\swidth="([0-9.]+)"[^>]*?\sheight="([0-9.]+)"`)

// normalizeMermaid replaces each fenced `mermaid` block with a paragraph
// holding one image, so both Markdown renderers place it as they place any
// standalone image. The image's destination is a content key; its source
// travels as a node attribute, and the fence too, so that a diagram that
// cannot be drawn can be put back as its source.
func normalizeMermaid(root ast.Node, source []byte, check func()) {
	var blocks []*ast.FencedCodeBlock
	_ = ast.Walk(root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		check()
		if fence, ok := n.(*ast.FencedCodeBlock); ok && entering {
			if strings.EqualFold(string(fence.Language(source)), "mermaid") {
				blocks = append(blocks, fence)
			}
			return ast.WalkSkipChildren, nil
		}
		return ast.WalkContinue, nil
	})
	for _, fence := range blocks {
		check()
		var b strings.Builder
		for i := 0; i < fence.Lines().Len(); i++ {
			line := fence.Lines().At(i)
			b.Write(line.Value(source))
		}
		code := b.String()
		sum := sha256.Sum256([]byte(code))
		link := ast.NewLink()
		link.Destination = []byte(mermaidPrefix + hex.EncodeToString(sum[:8]))
		im := ast.NewImage(link)
		im.AppendChild(im, ast.NewString([]byte("Mermaid "+mermaidKind(code))))
		im.SetAttributeString(mermaidAttribute, code)
		im.SetAttributeString(mermaidFence, fence)
		p := ast.NewParagraph()
		p.AppendChild(p, im)
		fence.Parent().ReplaceChild(fence.Parent(), fence, p)
	}
}

// mermaidKind is the diagram's header keyword, for its alternative text.
func mermaidKind(code string) string {
	for _, line := range strings.Split(code, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "%%") || line == "---" {
			continue
		}
		if f := strings.Fields(line); len(f) > 0 {
			return "diagram: " + f[0]
		}
	}
	return "diagram"
}

// mermaidSource returns the diagram source a normalized image carries.
func mermaidSource(im *ast.Image) (string, bool) {
	v, ok := im.AttributeString(mermaidAttribute)
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

// restoreMermaidSource replaces a diagram that could not be drawn with a note
// and its original fence, so a reader still sees what it says.
func restoreMermaidSource(im *ast.Image) {
	v, _ := im.AttributeString(mermaidFence)
	fence, ok := v.(*ast.FencedCodeBlock)
	p := im.Parent()
	if !ok || p == nil || p.Parent() == nil {
		return
	}
	note := ast.NewParagraph()
	em := ast.NewEmphasis(1)
	em.AppendChild(em, ast.NewString([]byte("Mermaid diagram could not be drawn; its source follows.")))
	note.AppendChild(note, em)
	p.Parent().InsertBefore(p.Parent(), p, note)
	p.Parent().ReplaceChild(p.Parent(), p, fence)
}

// renderMermaidImage draws a diagram from Markdown and records its page size
// on the node, so both renderers show it at reading size rather than at the
// raster's size. Dark draws it in go-mermaid's dark palette.
func renderMermaidImage(im *ast.Image, code string, dark bool) (image.Image, error) {
	img, w, h, err := renderMermaid(code, 0, dark)
	if err != nil {
		return nil, err
	}
	layout := MarkdownImageLayout(im)
	if layout.Width == 0 && layout.Height == 0 {
		layout.Width = max(1, int(math.Round(w*mermaidPageScale)))
		layout.Height = max(1, int(math.Round(h*mermaidPageScale)))
		im.SetAttributeString("gloss-image-layout", layout)
	}
	return img, nil
}

// renderMermaid returns the raster and the diagram's own size in its pixels.
// A positive edge fits the raster to it; zero uses the reading scale.
func renderMermaid(code string, edge int, dark bool) (image.Image, float64, float64, error) {
	if len(code) > MaxMermaidBytes {
		return nil, 0, 0, fmt.Errorf("mermaid: diagram exceeds %d KiB", MaxMermaidBytes>>10)
	}
	out, err := runParser(parserRequest{Kind: "mermaid", Name: "diagram.mmd", Data: []byte(code), Edge: edge, Dark: dark})
	if err != nil {
		return nil, 0, 0, err
	}
	img, err := decodeRaster(out.PNG)
	return img, out.Width, out.Height, err
}

// mermaidWork runs in the parser worker.
func mermaidWork(q parserRequest) (parserReply, error) {
	var out parserReply
	if len(q.Data) > MaxMermaidBytes {
		return out, fmt.Errorf("mermaid: diagram exceeds %d KiB", MaxMermaidBytes>>10)
	}
	palette := mermaid.Default
	if q.Dark {
		palette = mermaid.Dark
	}
	svg, err := mermaid.Render(string(q.Data), mermaid.WithTheme(palette))
	if err != nil {
		return out, err
	}
	m := mermaidSize.FindSubmatch(svg)
	if m == nil {
		return out, errors.New("mermaid: renderer returned no size")
	}
	out.Width, _ = strconv.ParseFloat(string(m[1]), 64)
	out.Height, _ = strconv.ParseFloat(string(m[2]), 64)
	if out.Width <= 0 || out.Height <= 0 {
		return out, errors.New("mermaid: renderer returned no size")
	}
	edge := q.Edge
	if edge <= 0 {
		edge = int(math.Ceil(max(out.Width, out.Height) * mermaidPageScale * mermaidRasterScale))
	}
	edge = max(1, min(edge, 4096))
	// go-mermaid paints its own background, a 100%-sized rectangle, in the
	// palette's page color.
	img, err := renderSVGUnbounded(q.Name, svg, edge)
	if err != nil {
		return out, err
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		return out, err
	}
	out.PNG = b.Bytes()
	return out, nil
}
