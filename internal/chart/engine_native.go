//go:build !js

package chart

import (
	"context"

	"github.com/NimbleMarkets/flint-ntcharts/compile"
	"github.com/NimbleMarkets/flint-ntcharts/envelope"
)

// wasmEngine runs flint-ntcharts' embedded compiler under wazero.
type wasmEngine struct{ runner *compile.Runner }

func newEngine(ctx context.Context) (engine, error) {
	runner, err := compile.New(ctx)
	if err != nil {
		return nil, err
	}
	return &wasmEngine{runner}, nil
}

func (e *wasmEngine) compile(ctx context.Context, input []byte, opts ...envelope.Option) ([]byte, error) {
	return e.runner.CompileRaw(ctx, input, opts...)
}

func (e *wasmEngine) close() { _ = e.runner.Close(context.Background()) }
