package document

import (
	"bytes"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"io"
	"math"

	"nimbleterminal.dev/ntpdf/pdfdoc"
)

const parserWorkerArg = "--gloss-parser-worker"
const maxParserMessage = 256 << 20

type parserRequest struct {
	Kind, Name string
	Data       []byte
	Page, Edge int
	Text       bool
	Dark       bool // A Mermaid diagram's palette.
}
type parserReply struct {
	Error         string
	PNG           []byte
	Pages, Page   int
	Width, Height float64
	Text          string
	NoText        bool
	Layer         *TextLayer
	Fields        []Field
}

// One request per worker. The parent kills and reaps it on timeout, cancellation
// or excessive output, so an uncooperative parser cannot outlive its operation.
func parserWork(q parserRequest) parserReply {
	reply, err := recoverParse("parsing document", func() (parserReply, error) {
		var out parserReply
		if len(q.Data) > MaxFileBytes {
			return out, ErrTooLarge
		}
		switch q.Kind {
		case "mermaid":
			return mermaidWork(q)
		case "svg":
			if q.Edge < 1 || q.Edge > 4096 {
				return out, errors.New("invalid SVG edge")
			}
			img, err := renderSVGUnbounded(q.Name, q.Data, q.Edge)
			if err != nil {
				return out, err
			}
			var b bytes.Buffer
			if err := png.Encode(&b, img); err != nil {
				return out, err
			}
			out.PNG = b.Bytes()
		case "pdf":
			// pdfdoc checks the streams, filters, predictors and cross-references
			// against the budget before the PDF library sees the bytes.
			doc, err := pdfdoc.Open(q.Data, pdfLimits())
			if err != nil {
				return out, err
			}
			out.Pages = doc.NumPages()
			out.Page = max(1, min(q.Page, out.Pages))
			out.Width, out.Height, _, _ = doc.PageSize(out.Page)
			out.Fields = pdfFields(doc, out.Page)
			if q.Text {
				out.Text, err = pdfPageText(doc, out.Page)
				if err != nil && !errors.Is(err, ErrNoTextLayer) {
					return out, err
				}
				out.NoText = errors.Is(err, ErrNoTextLayer)
				out.Layer = measureLayer(doc, out.Page, out.Text)
			}
		default:
			return out, errors.New("unknown parser operation")
		}
		return out, nil
	})
	if err != nil {
		reply.Error = err.Error()
	}
	return reply
}

// pdfLimits is gloss's policy for a PDF, read when the worker parses: the
// deadline is a variable that tests shorten.
func pdfLimits() pdfdoc.Limits {
	return pdfdoc.Limits{
		MaxFileBytes:     MaxFileBytes,
		MaxPages:         MaxPages,
		MaxStreamBytes:   256 << 20, // What one stream may inflate to.
		MaxInflatedBytes: 1 << 30,   // And all of them together.
		MaxImagePixels:   MaxPixels,
		Timeout:          parseDeadline,
		MaxTreeWalk:      64, // Parents followed up the page tree.
		MaxFormDepth:     8,  // Forms drawn within forms, when images are counted.
		MaxImageDraws:    100000,
	}
}

func serveParser(input io.Reader, output io.Writer) error {
	var q parserRequest
	if err := json.NewDecoder(io.LimitReader(input, maxParserMessage)).Decode(&q); err != nil {
		return err
	}
	return json.NewEncoder(output).Encode(parserWork(q))
}

func renderSVGWithin(name string, data []byte, edge int) (image.Image, error) {
	out, err := runParser(parserRequest{Kind: "svg", Name: name, Data: data, Edge: edge})
	if err != nil {
		return nil, err
	}
	return decodeRaster(out.PNG)
}

// Only the worker holds a PDF parser. The viewer keeps validated bytes and a
// page's metadata; native PDFium rasterization still uses the existing backend.
type safePDF struct {
	data  []byte
	reply parserReply
	text  bool
}

func newSafePDF(data []byte, page int, text bool) (*safePDF, error) {
	p := &safePDF{data: data}
	_, err := p.page(page, text)
	return p, err
}
func (p *safePDF) page(page int, text bool) (parserReply, error) {
	if p.reply.Pages > 0 && p.reply.Page == max(1, min(page, p.reply.Pages)) && (!text || p.text) {
		return p.reply, nil
	}
	out, err := runParser(parserRequest{Kind: "pdf", Data: p.data, Page: page, Text: text})
	if err == nil {
		p.reply = out
		p.text = text
	}
	return out, err
}
func (p *safePDF) dpi(page, maxEdge, dpi int) (int, error) {
	if maxEdge == 0 {
		return dpi, nil
	}
	out, err := p.page(page, false)
	if err != nil {
		return 0, err
	}
	if edge := math.Max(out.Width, out.Height); edge > 0 {
		return int(math.Ceil(float64(maxEdge) * 72 / edge)), nil
	}
	return dpi, nil
}
