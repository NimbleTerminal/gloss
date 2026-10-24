package chart

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/NimbleMarkets/flint-ntcharts/envelope"
	"github.com/NimbleMarkets/ntcharts/spec/echarts/v2"
	"github.com/NimbleMarkets/ntcharts/v2/spec"
)

// OptionWidth and OptionHeight are the canvas, in pixels, a Flint document
// is laid out for; the page scales it to the window.
const OptionWidth, OptionHeight = 960, 540

// Option is the ECharts option that draws doc in a browser: Flint's own
// option for a Flint document, the ntcharts ECharts module's for a bar or
// timeseries Spec. Other Specs are terminal-only and an error. Strings are
// cleaned as Compile cleans them.
func (c *Compiler) Option(ctx context.Context, doc Document) (json.RawMessage, error) {
	ctx, cancel := context.WithTimeout(ctx, Deadline)
	defer cancel()
	switch doc.Kind {
	case KindFlint:
		if doc.Unbound {
			return nil, errors.New("flint document has no data rows")
		}
		raw, err := aggregate(doc.Raw)
		if err != nil {
			return nil, err
		}
		out, err := c.compile(ctx, raw, envelope.WithBaseSize(OptionWidth, OptionHeight), envelope.WithCanvasSize(OptionWidth, OptionHeight), envelope.WithRenderer("raster"))
		if err != nil {
			return nil, err
		}
		res, err := envelope.ParseResult(out)
		if err != nil {
			return nil, err
		}
		if res.ECharts == nil {
			return nil, errors.New("flint compiler returned no ECharts option")
		}
		return cleanJSON(res.ECharts)
	case KindEnvelope:
		// A document's own ECharts option never reaches the page: only the
		// compiler's output and the ntcharts module's do, as with Compile.
		s, _, err := envelope.Parse(doc.Raw)
		if err != nil {
			return nil, err
		}
		return specOption(s)
	case KindSpec:
		var s spec.Spec
		if err := json.Unmarshal(doc.Raw, &s); err != nil {
			return nil, fmt.Errorf("ntcharts spec: %w", err)
		}
		return specOption(s)
	case KindError:
		var env struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal(doc.Raw, &env)
		return nil, fmt.Errorf("chart compiler: %s", env.Error.Message)
	}
	return nil, ErrNotChart
}

// specOption draws a Spec with the ntcharts ECharts module, which knows bar
// and timeseries charts.
func specOption(s spec.Spec) (json.RawMessage, error) {
	if s.Width == 0 || s.Height == 0 {
		s.Width, s.Height = 80, 24
	}
	cleanSpec(&s)
	if len(s.Theme.Palette) == 0 {
		s.Theme.Palette = append([]string(nil), DefaultPalette...)
	}
	if err := s.Validate(); err != nil {
		return nil, err
	}
	switch s.Type {
	case spec.ChartTypeBar, spec.ChartTypeTimeSeries:
	default:
		return nil, fmt.Errorf("a %s chart is terminal-only: the browser draws bar and timeseries specs, or any Flint document", s.Type)
	}
	chart, err := echarts.ToECharts(s)
	if err != nil {
		return nil, err
	}
	if v, ok := chart.(interface{ Validate() }); ok {
		v.Validate() // Fills go-echarts' defaults, as rendering would.
	}
	with, ok := chart.(interface{ JSON() map[string]any })
	if !ok {
		return nil, fmt.Errorf("echarts: no option for a %s chart", s.Type)
	}
	raw, err := json.Marshal(with.JSON())
	if err != nil {
		return nil, err
	}
	return cleanJSON(raw)
}

// cleanJSON cleans every string in an option as clean cleans text, and
// hardens it: the tooltip is drawn on the canvas, with ECharts' own
// formatter, and the option forms that would carry a tooltip of their own
// over the top level (baseOption, media, options, timeline) are dropped.
// The page's policy keeps any script from running anyway; this keeps a
// document's strings out of the DOM.
func cleanJSON(raw json.RawMessage) (json.RawMessage, error) {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, fmt.Errorf("echarts option: %w", err)
	}
	option, ok := cleanAny(v).(map[string]any)
	if !ok {
		return nil, errors.New("echarts option: not an object")
	}
	for _, key := range []string{"baseOption", "media", "options", "timeline"} {
		delete(option, key)
	}
	tooltip, _ := option["tooltip"].(map[string]any)
	if tooltip == nil {
		tooltip = map[string]any{}
	}
	delete(tooltip, "formatter")
	tooltip["renderMode"] = "richText"
	option["tooltip"] = tooltip
	return json.Marshal(option)
}

func cleanAny(v any) any {
	switch v := v.(type) {
	case string:
		return clean(v)
	case []any:
		for i := range v {
			v[i] = cleanAny(v[i])
		}
	case map[string]any:
		for k, e := range v {
			v[k] = cleanAny(e)
		}
	}
	return v
}
