package chart

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

const unboundDoc = `{"data":{"values":[]},"semantic_types":{"revenue":{"semanticType":"Price","unit":"USD"}},"chart_spec":{"chartType":"Bar Chart","encodings":{"x":{"field":"region"},"y":{"field":"revenue","aggregate":"sum"}}}}`

func TestBindFillsRowsAndTypes(t *testing.T) {
	doc, notes, err := Bind(sniffed(t, unboundDoc), sales[0], sales[1:], visible(5))
	if err != nil || len(notes) != 0 || doc.Unbound || doc.Kind != KindFlint {
		t.Fatalf("%+v %v %v", doc, notes, err)
	}
	var parsed struct {
		Data struct {
			Values []map[string]any `json:"values"`
		} `json:"data"`
		Types map[string]any  `json:"semantic_types"`
		Spec  json.RawMessage `json:"chart_spec"`
	}
	if err := json.Unmarshal(doc.Raw, &parsed); err != nil {
		t.Fatal(err)
	}
	if len(parsed.Data.Values) != 4 || parsed.Data.Values[0]["units"] != 120.0 || parsed.Data.Values[0]["region"] != "North" || parsed.Data.Values[0]["booked"] != "2026-03-31" {
		t.Fatalf("rows %v", parsed.Data.Values)
	}
	// The document's own types stay; the sheet supplies the rest.
	if _, ok := parsed.Types["revenue"].(map[string]any); !ok || parsed.Types["region"] != "Category" || parsed.Types["units"] != "Count" || parsed.Types["booked"] != "Date" {
		t.Fatalf("types %v", parsed.Types)
	}
	if !strings.Contains(string(parsed.Spec), `"aggregate":"sum"`) {
		t.Fatal("chart_spec changed")
	}
	res, err := Default.Compile(context.Background(), doc, 60, 15)
	if err != nil || len(res.Spec.Data.Series) != 1 {
		t.Fatalf("compile: %v", err)
	}
}

func TestBindLeavesBoundAndOtherDocuments(t *testing.T) {
	for _, raw := range []string{flintDoc, specDoc} {
		doc := sniffed(t, raw)
		bound, notes, err := Bind(doc, sales[0], sales[1:], visible(5))
		if err != nil || len(notes) != 0 || string(bound.Raw) != raw {
			t.Fatalf("%s: changed to %s (%v %v)", raw, bound.Raw, notes, err)
		}
	}
}

func TestBindCapsRowsAndHidesColumns(t *testing.T) {
	rows := make([][]string, MaxRows+3)
	for i := range rows {
		rows[i] = []string{"N", "Q1", "1", "2", "2026-01-01"}
	}
	doc, notes, err := Bind(sniffed(t, unboundDoc), sales[0], rows, []int{0, 3})
	if err != nil || len(notes) != 1 || !strings.Contains(string(notes[0]), "3 rows") {
		t.Fatalf("%v %v", notes, err)
	}
	if strings.Contains(string(doc.Raw), `"units"`) || strings.Count(string(doc.Raw), `"region":"N"`) != MaxRows {
		t.Fatalf("hidden column bound, or rows not capped: %d bytes", len(doc.Raw))
	}
}

// A field the sheet lacks is refused by name, with the columns there are:
// the compiler would chart it as nothing.
func TestBindRefusesUnknownFields(t *testing.T) {
	_, _, err := Bind(sniffed(t, strings.Replace(unboundDoc, `"field":"region"`, `"field":"planet"`, 1)), sales[0], sales[1:], visible(5))
	if err == nil || !strings.Contains(err.Error(), `"planet"`) || !strings.Contains(err.Error(), "booked, quarter, region, revenue, units") {
		t.Fatalf("missing field: %v", err)
	}
	// A hidden column counts as missing.
	if _, _, err := Bind(sniffed(t, unboundDoc), sales[0], sales[1:], []int{1, 2}); err == nil || !strings.Contains(err.Error(), `"region"`) {
		t.Fatalf("hidden field: %v", err)
	}
	// With no column shown, the message says so instead of trailing off.
	if _, _, err := Bind(sniffed(t, unboundDoc), sales[0], sales[1:], nil); err == nil || !strings.Contains(err.Error(), "no column is shown") {
		t.Fatalf("no columns: %v", err)
	}
	// Shorthand encodings ("x": "region") are checked too.
	short := `{"data":{"values":[]},"chart_spec":{"chartType":"Bar Chart","encodings":{"x":"planet","y":"revenue"}}}`
	if _, _, err := Bind(sniffed(t, short), sales[0], sales[1:], visible(5)); err == nil || !strings.Contains(err.Error(), `"planet"`) {
		t.Fatalf("shorthand: %v", err)
	}
}
