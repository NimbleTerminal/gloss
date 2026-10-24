package chart

import (
	"context"
	"os"
	"testing"

	"github.com/NimbleMarkets/flint-ntcharts/envelope"
)

func mustRead(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// The native engine and the browser's bridge receive the same input: the
// options are applied in Go before either compiles.
func TestEngineAppliesOptionsBeforeCompiling(t *testing.T) {
	e, err := newEngine(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer e.close()
	doc := sniffed(t, string(mustRead(t, "testdata/sales-bar.json")))
	opts := []envelope.Option{envelope.WithBaseSize(40, 12), envelope.WithCanvasSize(40, 12), envelope.WithRenderer("text")}
	applied, err := envelope.Apply(doc.Raw, opts)
	if err != nil {
		t.Fatal(err)
	}
	direct, err := e.compile(context.Background(), applied)
	if err != nil {
		t.Fatal(err)
	}
	viaOptions, err := e.compile(context.Background(), doc.Raw, opts...)
	if err != nil {
		t.Fatal(err)
	}
	if string(direct) != string(viaOptions) {
		t.Fatalf("options changed the result:\n%s\n%s", direct, viaOptions)
	}
	if s, _, err := envelope.Parse(direct); err != nil || s.Width != 40 {
		t.Fatalf("spec %v %v", s.Width, err)
	}
}
