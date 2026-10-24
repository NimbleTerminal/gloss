package document

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"nimbleterminal.dev/ntpdf/pdfdoc"
)

// MaxPageTextBytes bounds the text taken from one PDF page.
const MaxPageTextBytes = 16 << 20

// SparseTextRunes is how little text a page with pictures on it may have
// and be called sparse: a scan's page number, a figure's caption. Such a page
// is read better as a picture.
const SparseTextRunes = 100

// ErrNoTextLayer is a PDF page with no text of its own: a scan or a figure.
var ErrNoTextLayer = errors.New("no text layer")

// TextLayer says how much of a PDF page is text, for a caller deciding
// whether the text is the page or a picture would be better.
type TextLayer struct {
	Chars  int  // Runes of the text layer.
	Images int  // Images drawn on the page, inline ones among them; -1 when they could not be counted.
	Sparse bool // Images, and next to no text: read the page as a picture.
}

// pdfPageText reads the text layer of a page. A page with none, as a scan
// or a figure has, is an error rather than an empty text, so that a caller
// does not take silence for a blank page.
func pdfPageText(doc *pdfdoc.Document, page int) (string, error) {
	text, err := doc.PageText(page)
	if err != nil {
		return "", fmt.Errorf("page %d: text layer unreadable: %w", page, err)
	}
	if len(text) > MaxPageTextBytes {
		return "", fmt.Errorf("page %d: text exceeds 16 MiB", page)
	}
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("page %d has %w (a scan or a figure?): export it as a picture with --output", page, ErrNoTextLayer)
	}
	return strings.ToValidUTF8(strings.TrimSpace(text), "\ufffd") + "\n", nil
}

// measureLayer says how much of a page its text is.
func measureLayer(doc *pdfdoc.Document, page int, text string) *TextLayer {
	layer := &TextLayer{Chars: utf8.RuneCountInString(strings.TrimSpace(text)), Images: doc.PageImages(page)}
	layer.Sparse = layer.Images > 0 && layer.Chars < SparseTextRunes
	return layer
}

// Text is what a document says, for a reader that wants words rather than
// a picture: the Markdown made of a Word document, a page, or a notebook;
// Markdown as it is; text and JSON as they are; a sheet as CSV. It gives
// the text, the extension that suits it, and, for a document that is only
// a picture, an error that says what to do instead.
func Text(r Result) (text []byte, ext string, err error) {
	switch {
	case r.Err != nil:
		return nil, "", r.Err
	case r.Text != "":
		switch r.Kind {
		case "json":
			return []byte(r.Text), ".json", nil
		case "mermaid":
			return []byte(r.Text), ".mmd", nil
		}
		return []byte(r.Text), ".txt", nil
	case r.Sheet != nil:
		return r.Sheet.CSV(), ".csv", nil
	case r.Chart != nil:
		return r.Chart.Raw, ".json", nil
	case r.Markdown != nil:
		return r.Markdown.Source, ".md", nil
	}
	return nil, "", fmt.Errorf("a %s has no text: export it as a picture with --output", r.Kind)
}

// CSV writes the sheet's rows as comma-separated values.
func (s *Sheet) CSV() []byte {
	var b bytes.Buffer
	w := csv.NewWriter(&b)
	for _, row := range s.Rows {
		cells := make([]string, s.Columns)
		copy(cells, row)
		w.Write(cells)
	}
	w.Flush()
	return b.Bytes()
}
