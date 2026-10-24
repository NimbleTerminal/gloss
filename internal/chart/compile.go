package chart

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/NimbleMarkets/flint-ntcharts/envelope"
	"github.com/NimbleMarkets/ntcharts/v2/spec"
)

// Deadline bounds one compile, as the parser workers are bounded.
const Deadline = 15 * time.Second

// DefaultPalette colours series that name no colour of their own: the
// Tableau 10 that Flint's own charts use.
var DefaultPalette = []string{"#4e79a7", "#f28e2c", "#e15759", "#76b7b2", "#59a14f", "#edc949", "#af7aa1", "#ff9da7", "#9c755f", "#bab0ab"}

// Warning is what the Flint compiler says about a document short of refusing it.
type Warning = envelope.Warning

// Result is a chart ready to draw: a Spec sized to the cells it was compiled for.
type Result struct {
	Spec     spec.Spec
	Warnings []Warning // Never nil.
}

// Compiler runs Flint documents through the Flint compiler: the embedded
// wasm module natively, the page's bundle in a browser. The engine is made
// on first use and shared; Compile may be called from any goroutine.
type Compiler struct {
	once   sync.Once
	engine engine
	err    error
}

// Default is the process's compiler.
var Default = &Compiler{}

// ready makes the engine once. It is built under its own context: a
// cancelled first caller must not leave every later compile broken.
func (c *Compiler) ready() (engine, error) {
	c.once.Do(func() { c.engine, c.err = newEngine(context.Background()) })
	return c.engine, c.err
}

// Close frees the engine, if one was made.
func (c *Compiler) Close() {
	if c.engine != nil {
		c.engine.close()
	}
}

// compile runs input through the Flint compiler with opts applied first and
// returns the raw envelope, for envelope.Parse or envelope.ParseResult.
// The input is held to MaxDocumentBytes, as a file is: a document bound to
// a sheet's rows can outgrow the file it came from.
func (c *Compiler) compile(ctx context.Context, input []byte, opts ...envelope.Option) ([]byte, error) {
	if len(input) > MaxDocumentBytes {
		return nil, errors.New("chart data exceeds 16 MiB: show fewer columns or rows")
	}
	e, err := c.ready()
	if err != nil {
		return nil, fmt.Errorf("flint compiler: %w", err)
	}
	return e.compile(ctx, input, opts...)
}

// Compile turns doc into a Spec width by height cells. A Flint document is
// compiled with that size as both its base and its ceiling, so the chart
// fits the pane; a Spec or envelope is resized and validated.
func (c *Compiler) Compile(ctx context.Context, doc Document, width, height int) (Result, error) {
	ctx, cancel := context.WithTimeout(ctx, Deadline)
	defer cancel()
	res := Result{Warnings: []Warning{}}
	switch doc.Kind {
	case KindFlint:
		if doc.Unbound {
			return res, errors.New("flint document has no data rows")
		}
		raw, err := aggregate(doc.Raw)
		if err != nil {
			return res, err
		}
		out, err := c.compile(ctx, raw, envelope.WithBaseSize(width, height), envelope.WithCanvasSize(width, height), envelope.WithRenderer("text"))
		if err != nil {
			return res, err
		}
		s, warnings, err := envelope.Parse(out)
		if err != nil {
			return res, err
		}
		res.Spec, res.Warnings = s, warnings
	case KindEnvelope:
		s, warnings, err := envelope.Parse(doc.Raw)
		if err != nil {
			return res, err
		}
		res.Spec, res.Warnings = s, warnings
	case KindSpec:
		if err := json.Unmarshal(doc.Raw, &res.Spec); err != nil {
			return res, fmt.Errorf("ntcharts spec: %w", err)
		}
	case KindError:
		var env struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal(doc.Raw, &env)
		return res, fmt.Errorf("chart compiler: %s", env.Error.Message)
	default:
		return res, ErrNotChart
	}
	res.Spec.Width, res.Spec.Height = width, height
	cleanSpec(&res.Spec)
	// A series without a colour draws nothing visible on a bar chart: the
	// bars are coloured cells. Give the chart Flint's palette to draw from.
	if len(res.Spec.Theme.Palette) == 0 {
		res.Spec.Theme.Palette = append([]string(nil), DefaultPalette...)
	}
	if res.Warnings == nil {
		res.Warnings = []Warning{}
	}
	if err := res.Spec.Validate(); err != nil {
		return res, err
	}
	return res, nil
}
