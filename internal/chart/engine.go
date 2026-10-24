package chart

import (
	"context"

	"github.com/NimbleMarkets/flint-ntcharts/envelope"
)

// engine runs the Flint compiler: the embedded wasm module under wazero on
// native hosts, the bundle the page loads in a browser. Options are applied
// to the input before it compiles, so both see the same document; the
// result is a raw envelope for envelope.Parse or envelope.ParseResult.
type engine interface {
	compile(ctx context.Context, input []byte, opts ...envelope.Option) ([]byte, error)
	close()
}
