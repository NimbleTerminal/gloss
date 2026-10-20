package document

import (
	"context"
	"errors"
	"strings"
	"testing"
	"testing/fstest"
)

func TestTypesetCancelledRequestKeepsLayout(t *testing.T) {
	source := "# Title\n\n" + strings.Repeat("A paragraph of a few words that wraps.\n\n", 60)
	l := &Loader{Files: fstest.MapFS{"doc.md": {Data: []byte(source)}}}
	defer l.Close()
	q := Request{Theme: testTheme("light"), Path: "doc.md", Typeset: true, TypesetWidth: 192, TypesetView: &TypesetViewport{Height: 600}}
	if r := l.Load(q); r.Err != nil {
		t.Fatal(r.Err)
	}
	lay := l.direct
	laid := lay.Laid()
	// A request given up before it starts leaves the layout as it is.
	done, cancel := context.WithCancel(context.Background())
	cancel()
	q.TypesetView = &TypesetViewport{Top: 3000, Height: 600}
	q.Context = done
	if r := l.Load(q); !errors.Is(r.Err, context.Canceled) || l.direct != lay || lay.Laid() != laid {
		t.Fatalf("cancelled request: err %v, layout kept %v", r.Err, l.direct == lay)
	}
	// The next request takes the kept layout up from where it was.
	partial := lay.Laid()
	q.Context = nil
	if r := l.Load(q); r.Err != nil || l.direct != lay || lay.Laid() <= partial {
		t.Fatalf("resumed request: err %v, same layout %v, blocks %d then %d", r.Err, l.direct == lay, partial, lay.Laid())
	}
	// A layout that fails of itself is still dropped.
	bad := &Loader{Files: fstest.MapFS{"doc.md": {Data: []byte("# Title\n\n" + strings.Repeat("x", 20000) + "\n")}}}
	defer bad.Close()
	if r := bad.Load(Request{Theme: testTheme("light"), Path: "doc.md", Typeset: true, TypesetWidth: 192, TypesetView: &TypesetViewport{Height: 600}}); r.Err == nil || bad.direct != nil {
		t.Fatalf("failed layout kept: %v", r.Err)
	}
}
