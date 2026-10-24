package chart

import (
	"bytes"
	"strings"
	"testing"
)

const flintDoc = `{"data":{"values":[{"a":"x","b":1}]},"chart_spec":{"chartType":"Bar Chart","encodings":{"x":{"field":"a"},"y":{"field":"b"}}}}`
const specDoc = `{"type":"bar","width":40,"height":10,"data":{"series":[{"name":"s","values":[{"x":"a","y":1}]}]}}`

func TestSniffKinds(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		kind Kind
	}{
		{flintDoc, KindFlint},
		{`{"spec":` + specDoc + `,"warnings":[],"size":{"width":40,"height":10}}`, KindEnvelope},
		{specDoc, KindSpec},
		{`{"type":"heatmap","width":4,"height":2,"heat":{"matrix":[[1,2],[3,4]]}}`, KindSpec},
		{`{"error":{"message":"no"}}`, KindError},
	} {
		doc, err := Sniff([]byte(tc.raw))
		if err != nil || doc.Kind != tc.kind || !bytes.Equal(doc.Raw, []byte(tc.raw)) {
			t.Fatalf("%s: %v %v", tc.raw, doc.Kind, err)
		}
	}
}

func TestSniffUnboundFlint(t *testing.T) {
	doc, err := Sniff([]byte(`{"data":{"values":[]},"chart_spec":{"chartType":"Bar Chart","encodings":{}}}`))
	if err != nil || !doc.Unbound {
		t.Fatalf("empty values should be unbound: %+v %v", doc, err)
	}
	if doc, _ := Sniff([]byte(flintDoc)); doc.Unbound {
		t.Fatal("rows present but unbound")
	}
}

func TestSniffRejectsPlainJSON(t *testing.T) {
	for _, raw := range []string{
		`{"type":"FeatureCollection","features":[]}`, // GeoJSON: a type, but not a chart.
		`{"type":"bar"}`,                           // No data.
		`{"spec":"not a chart"}`,                   // An envelope holds an object.
		`{"spec":{"type":"ClusterIP","ports":[]}}`, // A Kubernetes manifest: a type, but no chart.
		`{"error":"not found","status":404}`,       // A saved API error: not a compiler's.
		`{"error":{"code":7}}`,                     // A compiler's error carries a message.
		`{"chart_spec":"x"}`, `{"chart_spec":{}}`,  // Flint names a chartType.
		`[1,2,3]`, `"x"`, `{not json`, `{}`,
		`{"data":{"url":"https://example.test/a.csv"},"chart_spec":{"chartType":"Bar Chart","encodings":{}}}`, // Remote data.
	} {
		if doc, err := Sniff([]byte(raw)); err == nil || doc.Kind != KindNone {
			t.Fatalf("accepted %s as %v", raw, doc.Kind)
		}
	}
	if _, err := Sniff(append([]byte(`{"chart_spec":{},"pad":"`), append(bytes.Repeat([]byte("x"), MaxDocumentBytes), '"', '}')...)); err == nil || !strings.Contains(err.Error(), "16 MiB") {
		t.Fatalf("oversize accepted: %v", err)
	}
}
