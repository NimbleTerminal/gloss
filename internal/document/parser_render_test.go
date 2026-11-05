package document

import (
	"encoding/json"
	"nimbleterminal.dev/ntmediabox"
	"testing"
)

func TestParserResponseRejectsMalformedMetadata(t *testing.T) {
	for _, out := range []parserReply{
		{PNG: []byte("encoded image")}, {Kind: "html"}, {Pages: MaxPages + 1}, {Page: 3, Pages: 2}, {Width: -1}, {Fields: make([]Field, 257)}, {Layer: &TextLayer{Chars: -1}}, {Error: "unsanitized error"},
	} {
		meta, _ := json.Marshal(out)
		if _, err := decodeParserResponse(ntmediabox.RenderResponse{Meta: meta}); err == nil {
			t.Fatalf("accepted %+v", out)
		}
	}
	for _, meta := range []string{`{} {}`, `{"unexpected":true}`, `{"Width":1e300}`} {
		if _, err := decodeParserResponse(ntmediabox.RenderResponse{Meta: []byte(meta)}); err == nil {
			t.Fatalf("accepted %s", meta)
		}
	}
}
