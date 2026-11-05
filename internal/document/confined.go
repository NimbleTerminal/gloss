package document

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"time"

	"nimbleterminal.dev/ntmediabox"
	"nimbleterminal.dev/ntpicture/animation"
)

// Only values needed by the supported, one-shot document operations cross the
// boundary. Host paths are labels; the worker reads only the supplied bytes.
type confinedRequest struct {
	Type                                                             string
	Page, DPI, MaxEdge                                               int
	TextOnly, Describe, Preview, Animate, Strict, Dark, Stdin, Probe bool
	Locale                                                           Locale
	Accept                                                           AcceptFilter
}

func (l *Loader) loadConfined(q Request) Result {
	out := Result{Generation: q.Generation}
	if q.Typeset || q.MarkdownMode == "typeset" || q.Layout {
		out.Err = errors.New("strict mode does not yet support typesetting or HTML layout")
		return out
	}
	data, err := readFileFrom(l.Files, q.Path)
	if err != nil {
		out.Err = err
		return out
	}
	worker, err := dialParserPolicy(true)
	if err != nil {
		out.Err = fmt.Errorf("strict mode requires OS confinement: %w", err)
		return out
	}
	defer worker.close()
	reply, err := worker.ask(q.Context, parserRequest{Kind: "document", Name: q.Path, Data: data, Load: &confinedRequest{
		Type: q.Type, Page: q.Page, DPI: q.DPI, MaxEdge: q.MaxEdge, TextOnly: q.TextOnly, Describe: q.Describe, Preview: q.Preview, Animate: q.Animate, Strict: q.Strict, Dark: q.Theme.Dark, Stdin: q.Stdin, Locale: q.Locale,
	}})
	out.Kind, out.Page, out.Pages, out.Text, out.Layer, out.Info = reply.Kind, reply.Page, reply.Pages, reply.Text, reply.Layer, reply.Fields
	if reply.Image != nil {
		out.Image = reply.Image
	}
	out.Theme = q.Theme.Name
	out.Err = err
	if reply.NoText {
		out.Err = fmt.Errorf("%s: %w", reply.Error, ErrNoTextLayer)
	}
	return out
}

func confinedProbe(files fs.FS, name, forced string, accept AcceptFilter) (string, error) {
	data, err := readFileFrom(files, name)
	if err != nil {
		return "", err
	}
	w, err := dialParserPolicy(true)
	if err != nil {
		return "", fmt.Errorf("strict mode requires OS confinement: %w", err)
	}
	defer w.close()
	reply, err := w.ask(context.Background(), parserRequest{Kind: "document", Name: name, Data: data, Load: &confinedRequest{Type: forced, Probe: true, Accept: accept}})
	return reply.Kind, err
}

func confinedKind(kind string) bool {
	switch kind {
	case "pdf", "svg", "mermaid", "image", "png", "jpeg", "gif", "webp", "bmp", "tiff", "heic", "heif", "jxl":
		return true
	}
	return false
}

func confinedWork(q parserRequest) (parserReply, error) {
	if q.Load == nil {
		return parserReply{}, errors.New("missing confined load options")
	}
	c := q.Load
	kind, err := Detect(q.Name, q.Data, c.Type)
	if err != nil {
		return parserReply{}, err
	}
	if !confinedKind(kind) {
		return parserReply{}, fmt.Errorf("strict mode does not yet support %s", kind)
	}
	if err := c.Accept.Check(q.Name, q.Data); err != nil {
		return parserReply{}, err
	}
	if c.Probe {
		return parserReply{Kind: kind}, nil
	}
	if c.Animate && !c.Preview && animation.Animated(q.Data) {
		return parserReply{}, errors.New("strict mode does not yet support animation; use a still export or preview")
	}
	name := filepath.Base(q.Name)
	l := &Loader{Files: confinedFileSystem{name: name, data: q.Data}}
	defer l.Close()
	result := l.Load(Request{Path: name, Type: kind, Page: c.Page, DPI: c.DPI, MaxEdge: c.MaxEdge, TextOnly: c.TextOnly, Describe: c.Describe, Preview: c.Preview, Strict: c.Strict, Stdin: c.Stdin, Locale: c.Locale, Theme: Theme{Dark: c.Dark}})
	return parserReply{Kind: result.Kind, Image: ntmediabox.ToRGBA(result.Image), Pages: result.Pages, Page: result.Page, Text: result.Text, Layer: result.Layer, Fields: result.Info, NoText: errors.Is(result.Err, ErrNoTextLayer)}, result.Err
}

type confinedFileSystem struct {
	name string
	data []byte
}

func (f confinedFileSystem) Open(name string) (fs.File, error) {
	if name != f.name {
		return nil, fs.ErrNotExist
	}
	return &confinedFile{Reader: bytes.NewReader(f.data), name: f.name}, nil
}

type confinedFile struct {
	*bytes.Reader
	name string
}

func (f *confinedFile) Close() error               { return nil }
func (f *confinedFile) Stat() (fs.FileInfo, error) { return confinedInfo{f.name, f.Size()}, nil }

type confinedInfo struct {
	name string
	size int64
}

func (f confinedInfo) Name() string     { return f.name }
func (f confinedInfo) Size() int64      { return f.size }
func (confinedInfo) Mode() fs.FileMode  { return 0400 }
func (confinedInfo) ModTime() time.Time { return time.Time{} }
func (confinedInfo) IsDir() bool        { return false }
func (confinedInfo) Sys() any           { return nil }
