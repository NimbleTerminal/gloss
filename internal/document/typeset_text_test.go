package document

import (
	"bytes"
	"strings"
	"testing"
	"testing/fstest"

	"nimbleterminal.dev/nttypeset"
)

func TestTypesetTextLiteralAndHighlighted(t *testing.T) {
	for _, tc := range []struct {
		path, input, kind string
		code              bool
	}{
		{"literal.txt", "# Café *literal* [link](https://example.com)\n\n    indented\n", "text", false},
		{"main.go", "package main\n\nfunc main() {\n\tprintln(\"Café\")\n}\n", "text", true},
		{"data.json", "{\"café\":true}", "json", true},
	} {
		t.Run(tc.path, func(t *testing.T) {
			l := &Loader{Files: fstest.MapFS{tc.path: {Data: []byte(tc.input)}}}
			defer l.Close()
			q := Request{Path: tc.path, Typeset: true, Page: 1, TypesetView: &TypesetViewport{Height: 400}}
			r := l.Load(q)
			if r.Err != nil || r.Image == nil || r.Kind != tc.kind || r.Typeset == nil || r.Markdown != nil {
				t.Fatalf("typeset: %+v", r)
			}
			var text strings.Builder
			colored := false
			for i, b := range l.typesetDocument.Blocks {
				if b.Kind != nttypeset.KindLine {
					t.Fatalf("text parsed as Markdown: %d", b.Kind)
				}
				if i > 0 {
					text.WriteByte('\n')
				}
				for _, s := range b.Spans {
					text.WriteString(s.Text)
					colored = colored || s.Token != 0
					if tc.code && strings.TrimSpace(s.Text) != "" && s.Style&nttypeset.Mono == 0 {
						t.Fatal("source lost monospace")
					}
				}
			}
			if tc.code && !colored {
				t.Fatal("source lost highlighting")
			}
			if !tc.code && !strings.Contains(text.String(), "# Café *literal* [link](https://example.com)") {
				t.Fatal(text.String())
			}
			eng, doc := l.typesetEngine, l.typesetDocument
			q.TypesetWidth, q.Generation = 300, 1
			if r = l.Load(q); r.Err != nil || l.typesetEngine != eng || l.typesetDocument != doc {
				t.Fatal("resize discarded preparation", r.Err)
			}
			q.Typeset, q.Generation = false, 2
			if r = l.Load(q); r.Err != nil || r.Markdown == nil || r.Image != nil || r.Kind != tc.kind {
				t.Fatal("text mode failed", r.Err)
			}
		})
	}
}

func TestTypesetTextLimits(t *testing.T) {
	for _, data := range [][]byte{bytes.Repeat([]byte("x"), MaxMarkdownBytes+1), {0xff}, []byte(strings.Repeat("line\n", 8193))} {
		l := &Loader{Files: fstest.MapFS{"input.txt": {Data: data}}}
		r := l.Load(Request{Path: "input.txt", Type: "text", Typeset: true})
		if r.Err == nil || l.typesetEngine != nil {
			t.Fatal("over-budget input reached renderer")
		}
		l.Close()
	}
}

func TestTypesetTextContentChange(t *testing.T) {
	files := fstest.MapFS{"input.txt": {Data: []byte("before")}}
	l := &Loader{Files: files}
	defer l.Close()
	q := Request{Path: "input.txt", Typeset: true}
	if r := l.Load(q); r.Err != nil {
		t.Fatal(r.Err)
	}
	old := l.typesetDocument
	files["input.txt"].Data = []byte("after")
	q.Generation++
	if r := l.Load(q); r.Err != nil || r.Text != "after\n" || l.typesetDocument == old {
		t.Fatal("stale content", r.Err)
	}
}
