//go:build js

package chart

import (
	"context"

	"github.com/NimbleMarkets/flint-ntcharts/envelope"
	"nimbleterminal.dev/ntshim/flintchart"
)

// bridgeEngine compiles through the flint-ntcharts bundle the page loads
// (ntshim's flintchart shim), so the browser build carries no wasm
// runtime of its own. A compile failure comes back as an error envelope,
// which the parsers turn into an error; only a broken bundle is an error
// here.
type bridgeEngine struct{}

func newEngine(context.Context) (engine, error) { return bridgeEngine{}, nil }

func (bridgeEngine) compile(ctx context.Context, input []byte, opts ...envelope.Option) ([]byte, error) {
	input, err := envelope.Apply(input, opts)
	if err != nil {
		return nil, err
	}
	return flintchart.Compile(ctx, input)
}

func (bridgeEngine) close() {}
