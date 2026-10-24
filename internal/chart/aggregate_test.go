package chart

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// The terminal backend returns zeros for aggregate encodings (it writes the
// value into the field and reads field_op), so gloss folds the rows itself.
func TestCompileAggregatesRowsItself(t *testing.T) {
	s := Analyze(sales[0], sales[1:], []int{0, 3})
	doc, err := s.Document(TemplateBar)
	if err != nil {
		t.Fatal(err)
	}
	res, err := Default.Compile(context.Background(), doc, 40, 8)
	if err != nil {
		t.Fatal(err)
	}
	values := res.Spec.Data.Series[0].Values
	if len(values) != 2 || values[0].Y != 100.5 || values[1].Y != 81.6 {
		t.Fatalf("summed values: %+v", values)
	}
	if labels := res.Spec.XAxis.Labels; strings.Join(labels, ",") != "North,South" {
		t.Fatalf("labels %v", labels)
	}
}

func TestAggregateOps(t *testing.T) {
	raw := `{"data":{"values":[{"k":"a","v":1},{"k":"a","v":3},{"k":"b","v":5},{"k":"b","v":"x"}]},"chart_spec":{"chartType":"Bar Chart","encodings":{"x":{"field":"k"},"y":{"field":"v","aggregate":"OP"}}}}`
	for op, want := range map[string][2]float64{"sum": {4, 5}, "mean": {2, 5}, "average": {2, 5}, "count": {2, 2}, "min": {1, 5}, "max": {3, 5}, "median": {2, 5}} {
		out, err := aggregate([]byte(strings.Replace(raw, "OP", op, 1)))
		if err != nil {
			t.Fatalf("%s: %v", op, err)
		}
		var parsed struct {
			Data struct {
				Values []map[string]any `json:"values"`
			} `json:"data"`
			Spec struct {
				Encodings map[string]map[string]any `json:"encodings"`
			} `json:"chart_spec"`
		}
		if err := json.Unmarshal(out, &parsed); err != nil {
			t.Fatal(err)
		}
		if len(parsed.Data.Values) != 2 || parsed.Data.Values[0]["v"] != want[0] || parsed.Data.Values[1]["v"] != want[1] {
			t.Fatalf("%s: %v", op, parsed.Data.Values)
		}
		if _, still := parsed.Spec.Encodings["y"]["aggregate"]; still {
			t.Fatalf("%s: aggregate left for the compiler", op)
		}
	}
	// Nothing to aggregate: the document comes back as it was.
	plain := []byte(flintDoc)
	if out, err := aggregate(plain); err != nil || string(out) != flintDoc {
		t.Fatalf("%s %v", out, err)
	}
}
