package document

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math"
	"unicode/utf8"

	"nimbleterminal.dev/ntmediabox"
)

type parserRenderServer struct{ server parserServer }

func (s *parserRenderServer) Close() error { s.server.doc = nil; return nil }
func (s *parserRenderServer) Handle(req ntmediabox.RenderRequest) (ntmediabox.RenderResponse, error) {
	if req.Kind != "gloss-parser" {
		return ntmediabox.RenderResponse{}, errors.New("unknown parser protocol")
	}
	var q parserRequest
	if err := json.Unmarshal(req.Meta, &q); err != nil {
		return ntmediabox.RenderResponse{}, err
	}
	q.Data = req.Data
	out := s.server.answer(q)
	// Encoded image decoding stays inside the worker. The host receives only
	// protocol-checked RGBA; the Web Worker retains its existing PNG transport.
	if len(out.PNG) > 0 && out.Error == "" {
		img, err := decodeRaster(out.PNG)
		if err != nil {
			return ntmediabox.RenderResponse{}, err
		}
		out.Image = ntmediabox.ToRGBA(img)
	}
	out.PNG = nil
	resp := ntmediabox.RenderResponse{Image: out.Image, Err: out.Error, Ended: out.Ended}
	out.Error = ""
	meta, err := json.Marshal(out)
	resp.Meta = meta
	return resp, err
}

func decodeParserResponse(resp ntmediabox.RenderResponse) (parserReply, error) {
	var out parserReply
	d := json.NewDecoder(bytes.NewReader(resp.Meta))
	d.DisallowUnknownFields()
	if err := d.Decode(&out); err != nil {
		return out, err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return parserReply{}, errors.New("trailing parser metadata")
	}
	invalid := func() (parserReply, error) { return parserReply{}, errors.New("invalid parser response metadata") }
	if len(out.PNG) != 0 || out.Error != "" || (out.Kind != "" && !confinedKind(out.Kind)) || out.Pages < 0 || out.Pages > MaxPages || out.Page < 0 || out.Page > MaxPages || (out.Pages > 0 && out.Page > out.Pages) {
		return invalid()
	}
	for _, v := range []float64{out.Width, out.Height} {
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 1e9 {
			return invalid()
		}
	}
	if len(out.Text) > MaxPageTextBytes || !utf8.ValidString(out.Text) || len(out.Fields) > 256 {
		return invalid()
	}
	for _, f := range out.Fields {
		if len(f.Label) > 1024 || len(f.Value) > 1<<20 || !utf8.ValidString(f.Label) || !utf8.ValidString(f.Value) {
			return invalid()
		}
	}
	if out.Layer != nil && (out.Layer.Chars < 0 || out.Layer.Chars > MaxPageTextBytes || out.Layer.Images < -1 || out.Layer.Images > 100000) {
		return invalid()
	}
	out.Image, out.Error, out.Ended = resp.Image, resp.Err, resp.Ended
	return out, nil
}
