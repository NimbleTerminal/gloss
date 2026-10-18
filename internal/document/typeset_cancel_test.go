package document

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"
)

// countdown is a context that is cancelled after a number of checks, so a
// request can be given up at a chosen point inside layout.
type countdown struct {
	context.Context
	left *int
}

func (c countdown) Err() error {
	if *c.left <= 0 {
		return context.Canceled
	}
	*c.left--
	return nil
}

func TestTypesetCancelledRequestKeepsLayout(t *testing.T) {
	source := "# Title\n\n" + strings.Repeat("A paragraph of a few words that wraps.\n\n", 60)
	l := &Loader{Files: fstest.MapFS{"doc.md": {Data: []byte(source)}}}
	defer l.Close()
	q := Request{Path: "doc.md", Typeset: true, TypesetWidth: 192, TypesetView: &TypesetViewport{Height: 600}}
	if r := l.Load(q); r.Err != nil {
		t.Fatal(r.Err)
	}
	lay := l.direct
	// A request given up before it starts leaves the layout as it is.
	done, cancel := context.WithCancel(context.Background())
	cancel()
	q.TypesetView = &TypesetViewport{Top: 3000, Height: 600}
	q.Context = done
	if r := l.Load(q); r.Err == nil || l.direct != lay {
		t.Fatalf("cancelled request: err %v, layout kept %v", r.Err, l.direct == lay)
	}
	// One given up inside a block leaves the layout as it was before the
	// block, and the next request takes it up from there.
	left := 40
	panicked := func() (p any) {
		defer func() { p = recover() }()
		lay.ensure(countdown{context.Background(), &left}, 1125*MaxPages)
		return nil
	}()
	if panicked == nil {
		t.Fatal("countdown did not cancel the layout")
	}
	if len(lay.starts) != lay.next || lay.next >= len(lay.p.blocks) {
		t.Fatalf("layout inconsistent after cancel: %d starts, %d of %d blocks", len(lay.starts), lay.next, len(lay.p.blocks))
	}
	partial := lay.next
	q.Context = nil
	r := l.Load(q)
	if r.Err != nil || l.direct != lay || lay.next <= partial {
		t.Fatalf("resumed request: err %v, same layout %v, blocks %d then %d", r.Err, l.direct == lay, partial, lay.next)
	}
	// The resumed layout is the one a fresh layout makes.
	fresh := &Loader{Files: fstest.MapFS{"doc.md": {Data: []byte(source)}}}
	defer fresh.Close()
	f := fresh.Load(q)
	if f.Err != nil || len(fresh.direct.ops) != len(lay.ops) {
		t.Fatalf("resumed layout has %d ops, fresh %d", len(lay.ops), len(fresh.direct.ops))
	}
	for i := range lay.ops {
		if lay.ops[i].rect != fresh.direct.ops[i].rect {
			t.Fatalf("op %d differs: %v vs %v", i, lay.ops[i].rect, fresh.direct.ops[i].rect)
		}
	}
	// A layout that fails of itself is still dropped.
	bad := &Loader{Files: fstest.MapFS{"doc.md": {Data: []byte("# Title\n\n" + strings.Repeat("x", 20000) + "\n")}}}
	defer bad.Close()
	if r := bad.Load(Request{Path: "doc.md", Typeset: true, TypesetWidth: 192, TypesetView: &TypesetViewport{Height: 600}}); r.Err == nil || bad.direct != nil {
		t.Fatalf("failed layout kept: %v", r.Err)
	}
}
