// Frozen goldmark-pdf adapter retained only for comparison, not shipped by gloss.
package main

import (
	"bytes"
	"fmt"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/signintech/gopdf"
	mdpdf "github.com/stephenafamo/goldmark-pdf"
	pdfgo "github.com/stephenafamo/goldmark-pdf/gopdf"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/util"
	"image"
	"image/png"
	"io"
	"math"
	"nimbleterminal.dev/gloss/internal/document"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"
)

const (
	typesetHeight = 540
	MaxPages      = document.MaxPages
	MaxFileBytes  = document.MaxFileBytes
	parseDeadline = 15 * time.Second
)

var typesetMu sync.Mutex

type typesetAsset struct{ image image.Image }

func legacyPDF(path, fontDir string, width int) (output []byte, err error) {
	// Third-party layout code may panic on malformed content. Never let it take
	// down the viewer, nor publish a partially generated PDF.
	defer func() {
		if p := recover(); p != nil {
			output = nil
			err = fmt.Errorf("typesetting Markdown: %v", p)
		}
	}()
	data, err := boundedRead(path, document.MaxMarkdownBytes)
	if err != nil {
		return nil, err
	}
	prep, err := prepare(path)
	if err != nil {
		return nil, err
	}
	root := document.MarkdownTree(data)
	assets := map[string]*typesetAsset{}
	for name, im := range prep.images {
		assets[name] = &typesetAsset{image: im}
	}

	typesetMu.Lock()
	defer typesetMu.Unlock()
	measure := float64(width)
	gp := &gopdf.GoPdf{}
	gp.Start(gopdf.Config{Unit: gopdf.UnitPT, PageSize: gopdf.Rect{W: measure, H: typesetHeight}})
	gp.SetMargins(24, 24, 24, 24)
	backend := &typesetPDF{Impl: pdfgo.Impl{GoPdf: gp, Width: measure, Height: typesetHeight}, started: time.Now()}
	for _, family := range []string{"Go", "GoMono"} {
		for _, variant := range []struct{ name, style string }{{"Regular", ""}, {"Bold", "B"}, {"Italic", "I"}, {"BoldItalic", "BI"}} {
			b, e := os.ReadFile(filepath.Join(fontDir, family+"-"+variant.name+".ttf"))
			if e != nil {
				return nil, e
			}
			style := gopdf.Regular
			if strings.Contains(variant.style, "B") {
				style |= gopdf.Bold
			}
			if strings.Contains(variant.style, "I") {
				style |= gopdf.Italic
			}
			if e = gp.AddTTFFontDataWithOption(family, b, gopdf.TtfOption{Style: style}); e != nil {
				return nil, e
			}
		}
	}
	gp.AddPage()
	body := mdpdf.Font{Family: "Go", Type: mdpdf.FontTypeCustom}
	code := mdpdf.Font{Family: "GoMono", Type: mdpdf.FontTypeCustom}
	renderer := mdpdf.New(mdpdf.WithPDF(backend), mdpdf.WithBodyFont(body), mdpdf.WithHeadingFont(body), mdpdf.WithCodeFont(code), mdpdf.WithEscapeHTML(false), mdpdf.WithCodeBlockTheme(styles.Get("github")), mdpdf.WithTraceWriter(io.Discard), mdpdf.WithNodeRenderers(util.Prioritized(&typesetNodes{assets: assets, backend: backend}, 100)))
	var buf bytes.Buffer
	if err = renderer.Render(&buf, data, root); err != nil {
		return nil, err
	}
	if buf.Len() > MaxFileBytes {
		return nil, document.ErrTooLarge
	}
	return buf.Bytes(), nil
}

// The tagged gopdf adapter drops a rune at each SplitText wrap. Keep it, and
// keep linear work even for long unbroken code lines.
type typesetPDF struct {
	pdfgo.Impl
	fill      [3]uint8
	pages     int
	started   time.Time
	strike    bool
	textColor [3]uint8
}

func (p *typesetPDF) AddPage() {
	p.pages++
	if p.pages > MaxPages {
		panic("typeset Markdown exceeds PDF page limit")
	}
	p.GoPdf.AddPage()
}
func (p *typesetPDF) ensure(h float64) {
	if time.Since(p.started) > parseDeadline {
		panic("typesetting Markdown exceeded layout deadline")
	}
	_, _, _, bottom := p.GetMargins()
	if p.GetY()+h > p.Height-bottom {
		p.AddPage()
	}
}
func (p *typesetPDF) BR(h float64)               { p.GoPdf.Br(h); p.ensure(h) }
func (p *typesetPDF) SetFillColor(r, g, b uint8) { p.fill = [3]uint8{r, g, b} }
func (p *typesetPDF) SetTextColor(r, g, b uint8) {
	p.textColor = [3]uint8{r, g, b}
	p.GoPdf.SetTextColor(r, g, b)
}
func (p *typesetPDF) strikeLine(x, y, w, h float64) {
	if p.strike {
		p.GoPdf.SetStrokeColor(p.textColor[0], p.textColor[1], p.textColor[2])
		p.GoPdf.SetLineWidth(.5)
		p.GoPdf.Line(x, y+h*.55, x+w, y+h*.55)
	}
}
func (p *typesetPDF) SetFont(family, style string, size int) error {
	p.strike = strings.Contains(style, "S")
	if err := p.GoPdf.SetFont(family, style, size); err != nil {
		panic(err)
	}
	return nil
}
func (p *typesetPDF) CellFormat(w, h float64, text, border string, ln int, align string, fill bool, link int, url string) {
	p.ensure(h)
	if fill {
		p.GoPdf.SetFillColor(p.fill[0], p.fill[1], p.fill[2])
		p.GoPdf.RectFromUpperLeftWithStyle(p.GetX(), p.GetY(), w, h, "F")
	}
	borders := 0
	if border == "1" {
		borders = gopdf.AllBorders
	} else {
		for _, v := range []struct {
			letter string
			value  int
		}{{"L", gopdf.Left}, {"R", gopdf.Right}, {"T", gopdf.Top}, {"B", gopdf.Bottom}} {
			if strings.Contains(border, v.letter) {
				borders |= v.value
			}
		}
	}
	x, y := p.GetX(), p.GetY()
	float := gopdf.Right
	if ln == 1 {
		float = gopdf.Bottom
	}
	if err := p.GoPdf.CellWithOption(&gopdf.Rect{W: w, H: h}, text, gopdf.CellOption{Align: gopdf.Left, Border: borders, Float: float}); err != nil {
		panic(err)
	}
	p.strikeLine(x, y, p.MeasureTextWidth(text), h)
	if ln == 1 {
		left, _, _, _ := p.GetMargins()
		p.SetX(left)
	}
}
func (p *typesetPDF) WriteText(h float64, text string)                 { p.writeText(h, text, "") }
func (p *typesetPDF) WriteExternalLink(h float64, text, url string)    { p.writeText(h, text, url) }
func (p *typesetPDF) WriteInternalLink(h float64, text, anchor string) { p.writeText(h, text, "") }
func (p *typesetPDF) writeText(h float64, text, url string) {
	// Split into words, retaining spaces. Inline style runs share the cursor.
	runes := []rune(text)
	for start := 0; start < len(runes); {
		end := start + 1
		for end < len(runes) && !unicode.IsSpace(runes[end-1]) && !unicode.IsSpace(runes[end]) {
			end++
		}
		word := string(runes[start:end])
		start = end
		left, _, right, _ := p.GetMargins()
		width := p.MeasureTextWidth(word)
		if p.GetX()+width > p.Width-right && p.GetX() > left {
			p.BR(h)
		}
		if strings.TrimSpace(word) == "" && p.GetX() == left {
			continue
		}
		chunks := p.SplitText(word, p.Width-left-right)
		for i, chunk := range chunks {
			if i > 0 {
				p.BR(h)
			}
			p.ensure(h)
			x, y := p.GetX(), p.GetY()
			w := p.MeasureTextWidth(chunk)
			if err := p.GoPdf.CellWithOption(&gopdf.Rect{W: w, H: h}, chunk, gopdf.CellOption{Align: gopdf.Left, Float: gopdf.Right}); err != nil {
				panic(err)
			}
			p.strikeLine(x, y, w, h)
			if url != "" {
				p.GoPdf.AddExternalLink(url, x, y, w, h)
			}
		}
	}
}

func (p *typesetPDF) SplitText(s string, width float64) []string {
	var lines []string
	var line []rune
	used := 0.0
	for _, r := range s {
		w := p.MeasureTextWidth(string(r))
		if r == '\n' || (len(line) > 0 && used+w > width) {
			lines = append(lines, string(line))
			line = nil
			used = 0
		}
		if r != '\n' {
			line = append(line, r)
			used += w
		}
	}
	if len(line) > 0 {
		lines = append(lines, string(line))
	}
	return lines
}

// Override image nodes entirely: goldmark-pdf's default image handler always
// falls back to a web filesystem. It is never invoked here, even for errors.
type typesetNodes struct {
	assets  map[string]*typesetAsset
	backend *typesetPDF
}

func (r *typesetNodes) RegisterFuncs(reg mdpdf.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindImage, r.render)
	reg.Register(ast.KindText, r.text)
}

// The tagged renderer ignores Goldmark's line-break flags, joining words
// across source lines. Soft breaks are spaces; explicit breaks start a line.
func (r *typesetNodes) text(w *mdpdf.Writer, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*ast.Text)
	text := string(n.Segment.Value(source))
	if n.SoftLineBreak() {
		text += " "
	}
	w.WriteText(text)
	if n.HardLineBreak() {
		w.Pdf.BR(w.Styles.Normal.Size + w.Styles.Normal.Spacing)
	}
	return ast.WalkContinue, nil
}

func (r *typesetNodes) render(w *mdpdf.Writer, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	mdpdf.SetStyle(w.Pdf, *w.Styles.Normal)
	a := r.assets[string(n.(*ast.Image).Destination)]
	p := r.backend
	if a == nil {
		w.Pdf.WriteText(14, "[image unavailable]")
		w.Pdf.BR(16)
		return ast.WalkSkipChildren, nil
	}
	b := a.image.Bounds()
	width := math.Min(p.Width-48, float64(b.Dx())*72/150)
	height := width * float64(b.Dy()) / float64(b.Dx())
	if height > p.Height-48 {
		width *= (p.Height - 48) / height
		height = p.Height - 48
	}
	if p.GetY()+height > p.Height-24 {
		p.AddPage()
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, a.image); err != nil {
		return ast.WalkStop, err
	}
	holder, err := gopdf.ImageHolderByBytes(encoded.Bytes())
	if err != nil {
		return ast.WalkStop, err
	}
	if err = p.GoPdf.ImageByHolder(holder, 24, p.GetY(), &gopdf.Rect{W: width, H: height}); err != nil {
		return ast.WalkStop, err
	}
	p.SetY(p.GetY() + height + 8)
	p.SetX(24)
	return ast.WalkSkipChildren, nil
}
