package chart

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/NimbleMarkets/ntcharts/v2/spec"
)

// Kind says what a chart document is.
type Kind int

const (
	KindNone     Kind = iota
	KindFlint         // ChartAssemblyInput: has chart_spec.
	KindEnvelope      // A compiler envelope: has spec.
	KindSpec          // A raw ntcharts spec: a known type, with data or heat.
	KindError         // {"error":{"message":...}} from an upstream compiler.
)

func (k Kind) String() string {
	switch k {
	case KindFlint:
		return "flint"
	case KindEnvelope:
		return "envelope"
	case KindSpec:
		return "spec"
	case KindError:
		return "error"
	}
	return ""
}

// MaxDocumentBytes bounds a chart document.
const MaxDocumentBytes = 16 << 20

// Document is a sniffed chart document.
type Document struct {
	Kind    Kind
	Raw     []byte
	Unbound bool // A Flint document whose data.values is empty: the host supplies rows.
}

// isSpec reports whether an object is an ntcharts spec: a known chart type
// with data or heat to draw.
func isSpec(obj map[string]json.RawMessage) bool {
	var kind string
	if obj["type"] == nil || json.Unmarshal(obj["type"], &kind) != nil {
		return false
	}
	return spec.ChartType(kind).IsKnown() && (obj["data"] != nil || obj["heat"] != nil)
}

// ErrNotChart says a JSON document is not a chart of any kind gloss knows.
var ErrNotChart = errors.New("not a chart document")

// Sniff classifies raw with flint-tui's rules, tightened so that ordinary
// JSON with a "type" key is not taken for a chart: a raw spec needs a known
// chart type and data or heat, and an envelope holds a spec object. Flint
// documents may not fetch data.
func Sniff(raw []byte) (Document, error) {
	if len(raw) > MaxDocumentBytes {
		return Document{}, fmt.Errorf("chart document exceeds 16 MiB")
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		return Document{}, ErrNotChart
	}
	doc := Document{Raw: raw}
	switch {
	case probe["chart_spec"] != nil:
		var chartSpec struct {
			ChartType string `json:"chartType"`
		}
		if json.Unmarshal(probe["chart_spec"], &chartSpec) != nil || chartSpec.ChartType == "" {
			return Document{}, ErrNotChart
		}
		var data struct {
			Values []json.RawMessage `json:"values"`
			URL    string            `json:"url"`
		}
		if probe["data"] != nil {
			if err := json.Unmarshal(probe["data"], &data); err != nil {
				return Document{}, fmt.Errorf("flint data: %w", err)
			}
		}
		if data.URL != "" {
			return Document{}, fmt.Errorf("flint data.url is not fetched; give the rows inline")
		}
		doc.Kind, doc.Unbound = KindFlint, len(data.Values) == 0
	case probe["spec"] != nil:
		var inner map[string]json.RawMessage
		if json.Unmarshal(probe["spec"], &inner) != nil || !isSpec(inner) {
			return Document{}, ErrNotChart
		}
		doc.Kind = KindEnvelope
	case probe["error"] != nil:
		// A compiler's error envelope holds nothing beyond the envelope's
		// own keys; an API's error body ({"error":…,"status":404}) is JSON.
		for k := range probe {
			if k != "error" && k != "warnings" && k != "size" {
				return Document{}, ErrNotChart
			}
		}
		var failure struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(probe["error"], &failure) != nil || failure.Message == "" {
			return Document{}, ErrNotChart
		}
		doc.Kind = KindError
	case isSpec(probe):
		doc.Kind = KindSpec
	default:
		return Document{}, ErrNotChart
	}
	return doc, nil
}
