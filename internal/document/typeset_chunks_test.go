package document

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"
)

func TestTypesetChunksFullHistory(t *testing.T) {
	const lines = 100001
	data := []byte(strings.Repeat("+ a change in the history of this repository\n", lines-1) + "+ FINAL HISTORY LINE\n")
	l := &Loader{Files: fstest.MapFS{"history.diff": {Data: data}}}
	defer l.Close()
	q := Request{Path: "history.diff", Typeset: true, Theme: testTheme("light"), TypesetLineNumbers: true, TypesetView: &TypesetViewport{Height: 400}}
	load := func() Result {
		t.Helper()
		r := l.Load(q)
		if r.Err != nil {
			t.Fatal(r.Err)
		}
		return r
	}
	r := load()
	if r.TypesetView.Chunk != 0 || r.TypesetView.Chunks < 2 || r.TypesetView.Lines != lines || l.direct.Laid() >= 256 {
		t.Fatalf("first viewport %+v, laid %d", r.TypesetView, l.direct.Laid())
	}
	// End goes straight to the final source chunk, beyond the former 2 MiB cap.
	q.TypesetView = &TypesetViewport{Height: 400, End: true}
	r = load()
	if !r.TypesetDone || r.TypesetView.Chunk != r.TypesetView.Chunks-1 {
		t.Fatalf("end %+v", r.TypesetView)
	}
	blocks := l.typesetDocument.Blocks
	if got := blocks[len(blocks)-1].Spans[0].Text; got != "+ FINAL HISTORY LINE" {
		t.Fatalf("last text %q", got)
	}
	// Go to a global line, with a globally numbered gutter.
	q.TypesetView = &TypesetViewport{Height: 400, Line: 90001}
	r = load()
	if r.TypesetView.Line != 90001 || l.directOptions.LineOffset == 0 || l.directOptions.LineCount != lines {
		t.Fatalf("global line %+v options %+v", r.TypesetView, l.directOptions)
	}
	// Home returns to the first chunk.
	q.TypesetView = &TypesetViewport{Height: 400}
	r = load()
	if r.TypesetView.Chunk != 0 || r.TypesetView.Line != 1 {
		t.Fatalf("home %+v", r.TypesetView)
	}
	// Complete just this chunk, and ensure a scroll past the bottom first shows
	// its last lines, rather than dropping them at the boundary.
	if err := l.direct.Complete(context.Background()); err != nil {
		t.Fatal(err)
	}
	bottom := l.direct.Extent() - 400
	q.TypesetView = &TypesetViewport{Height: 400, Top: bottom + 50}
	r = load()
	if r.TypesetView.Chunk != 0 || r.TypesetView.Top != bottom {
		t.Fatalf("bottom skipped %+v", r.TypesetView)
	}
	q.TypesetView = &TypesetViewport{Height: 400, Top: bottom + 50}
	r = load()
	if r.TypesetView.Chunk != 1 || r.TypesetView.Line != 257 {
		t.Fatalf("next chunk %+v", r.TypesetView)
	}
	q.TypesetView = &TypesetViewport{Height: 400, Chunk: 1, Top: -75}
	r = load()
	if r.TypesetView.Chunk != 0 || r.TypesetView.Top != bottom {
		t.Fatalf("previous chunk %+v", r.TypesetView)
	}
	// Reflow keeps the same source block visible, even when earlier long lines
	// wrap differently. Selection positions cannot migrate to another chunk.
	q.TypesetView = &TypesetViewport{Height: 400, Line: 90001}
	r = load()
	q.TypesetView = &TypesetViewport{Height: 400, Chunk: r.TypesetView.Chunk, Top: r.TypesetView.Top}
	q.TypesetWidth = 240
	r = load()
	if r.TypesetView.Line < 90000 || r.TypesetView.Line > 90001 {
		t.Fatalf("reflow lost anchor %+v", r.TypesetView)
	}
	q.TypesetSelect = &TypesetSelection{Anchor: TypesetPlace{Placed: true, Block: 0, Para: 0, Rune: 0}, Focus: TypesetPlace{Placed: true, Block: 0, Para: 0, Rune: 4}}
	q.TypesetView = &TypesetViewport{Height: 400, Chunk: r.TypesetView.Chunk, End: true}
	r = load()
	if r.TypesetSelection != nil || r.TypesetSelected != "" || r.TypesetAsked == nil {
		t.Fatal("selection leaked into another chunk")
	}
}
