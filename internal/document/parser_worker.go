package document

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	Dark       bool           // A Mermaid diagram's palette.
	Limits     *pdfdoc.Limits // What a PDF may cost, sent with its bytes.
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
	Ended         bool // The worker has stopped and will answer no more.
}

// A worker answers one request and ends, except for a PDF: the request with
// its bytes opens it, and the worker stays to answer "page" requests about
// it until its input closes. The parent kills and reaps it on timeout,
// cancellation or excessive output, so an uncooperative parser cannot outlive
// its operation, and closes it with the document.
type parserServer struct {
	doc *pdfdoc.Document
}

// answer is the reply to q. Its Ended says the worker will answer no more:
// a one-shot request, a PDF that did not open, or a panic, which leaves the
// document in a state no one checked.
func (s *parserServer) answer(q parserRequest) (reply parserReply) {
	defer func() {
		if r := recover(); r != nil {
			s.doc = nil
			reply = parserReply{Error: fmt.Sprintf("parsing document: %v", r)}
		}
		reply.Ended = s.doc == nil
	}()
	reply, err := s.work(q)
	if err != nil {
		reply.Error = err.Error()
	}
	return reply
}

func (s *parserServer) work(q parserRequest) (parserReply, error) {
	var out parserReply
	if len(q.Data) > MaxFileBytes {
		return out, ErrTooLarge
	}
	switch {
	case q.Kind == "mermaid" && s.doc == nil:
		return mermaidWork(q)
	case q.Kind == "svg" && s.doc == nil:
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
	case q.Kind == "pdf" && s.doc == nil:
		if q.Limits == nil {
			return out, errors.New("PDF request without limits")
		}
		// pdfdoc checks the streams, filters, predictors and cross-references
		// against the budget before the PDF library sees the bytes.
		doc, err := pdfdoc.Open(q.Data, *q.Limits)
		if err != nil {
			return out, err
		}
		s.doc = doc
		return pdfPageWork(doc, q)
	case q.Kind == "page" && s.doc != nil:
		return pdfPageWork(s.doc, q)
	default:
		return out, errors.New("unknown parser operation")
	}
	return out, nil
}

// pdfPageWork describes one page of an open PDF, and reads its text if asked.
func pdfPageWork(doc *pdfdoc.Document, q parserRequest) (parserReply, error) {
	var out parserReply
	var err error
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
	return out, nil
}

// pdfLimits is gloss's policy for a PDF, sent to the worker with its bytes:
// the deadline is a variable that tests shorten.
func pdfLimits() *pdfdoc.Limits {
	return &pdfdoc.Limits{
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

// Messages each way are one JSON value on a line of their own.
func writeMessage(w io.Writer, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = w.Write(append(b, '\n'))
	return err
}

func readMessage(r *bufio.Reader, v any) error {
	var line []byte
	for {
		part, err := r.ReadSlice('\n')
		if len(line)+len(part) > maxParserMessage {
			return errors.New("parser message exceeds limit")
		}
		line = append(line, part...)
		if err == bufio.ErrBufferFull {
			continue
		}
		if err != nil && (err != io.EOF || len(line) == 0) {
			return err
		}
		return json.Unmarshal(line, v)
	}
}

func serveParser(input io.Reader, output io.Writer) error {
	in := bufio.NewReader(input)
	var s parserServer
	for {
		var q parserRequest
		if err := readMessage(in, &q); err != nil {
			if err == io.EOF && s.doc != nil {
				return nil
			}
			return err
		}
		reply := s.answer(q)
		if err := writeMessage(output, reply); err != nil || reply.Ended {
			return err
		}
	}
}

func renderSVGWithin(name string, data []byte, edge int) (image.Image, error) {
	out, err := runParser(parserRequest{Kind: "svg", Name: name, Data: data, Edge: edge})
	if err != nil {
		return nil, err
	}
	return decodeRaster(out.PNG)
}

// Only the worker holds a PDF parser. The viewer keeps the validated bytes,
// the open worker and the last page's metadata; PDFium draws the pictures.
// The worker is opened on the first question and again after one is
// stopped; closing the document stops it.
type safePDF struct {
	data   []byte
	worker parserConn
	reply  parserReply
	text   bool
}

func newSafePDF(ctx context.Context, data []byte, page int, text bool) (*safePDF, error) {
	p := &safePDF{data: data}
	_, err := p.page(ctx, page, text)
	return p, err
}

func (p *safePDF) page(ctx context.Context, page int, text bool) (parserReply, error) {
	if p.reply.Pages > 0 && p.reply.Page == max(1, min(page, p.reply.Pages)) && (!text || p.text) {
		return p.reply, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	q := parserRequest{Kind: "page", Page: page, Text: text}
	if p.worker == nil {
		worker, err := dialParser()
		if err != nil {
			return parserReply{}, err
		}
		p.worker = worker
		q.Kind, q.Data, q.Limits = "pdf", p.data, pdfLimits()
	}
	out, err := p.worker.ask(ctx, q)
	if err == nil {
		p.reply, p.text = out, text
	} else if out.Error == "" || out.Ended {
		// The worker is gone, or could not open the file.
		p.close()
	}
	return out, err
}

func (p *safePDF) close() {
	if p != nil && p.worker != nil {
		p.worker.close()
		p.worker = nil
	}
}

// dpi is the resolution that draws a page within maxEdge pixels.
func (out parserReply) dpi(maxEdge, dpi int) int {
	if maxEdge == 0 {
		return dpi
	}
	if edge := math.Max(out.Width, out.Height); edge > 0 {
		return int(math.Ceil(float64(maxEdge) * 72 / edge))
	}
	return dpi
}

// parserConn is a running worker: native, a child process; in the browser, a
// Web Worker.
type parserConn interface {
	// ask sends q and waits for the reply, at most parseDeadline or until
	// ctx is done, when the worker is stopped.
	ask(ctx context.Context, q parserRequest) (parserReply, error)
	close()
}

// runParser asks a worker one question, and stops it.
func runParser(q parserRequest) (parserReply, error) {
	worker, err := dialParser()
	if err != nil {
		return parserReply{}, err
	}
	defer worker.close()
	return worker.ask(context.Background(), q)
}
