package document

import (
	"os"
	"path/filepath"
	"testing"

	"nimbleterminal.dev/gloss/internal/chart"
)

const flintFile = `{"data":{"values":[{"a":"x","b":1}]},"chart_spec":{"chartType":"Bar Chart","encodings":{"x":{"field":"a"},"y":{"field":"b"}}}}`

func TestChartKind(t *testing.T) {
	for _, name := range []string{"sales.json", "sales.chart", "undecorated"} {
		path := filepath.Join(t.TempDir(), name)
		if err := os.WriteFile(path, []byte(flintFile), 0o600); err != nil {
			t.Fatal(err)
		}
		if kind, err := Probe(path, ""); err != nil || kind != "chart" {
			t.Fatalf("%s: %s %v", name, kind, err)
		}
		l := &Loader{}
		r := l.Load(Request{Path: path, Generation: 1})
		l.Close()
		if r.Err != nil || r.Kind != "chart" || r.Chart == nil || r.Chart.Kind != chart.KindFlint {
			t.Fatalf("%s: %+v", name, r)
		}
		if field(r.Info, "Format") != "Flint chart" {
			t.Fatalf("info: %+v", r.Info)
		}
	}
	if kind, err := Detect("x.json", []byte(`{"type":"bar","width":1,"height":1,"data":{"series":[]}}`), "chart"); err != nil || kind != "chart" {
		t.Fatalf("forced: %s %v", kind, err)
	}
}

// A chart file is still JSON to --text and to --accept.
func TestChartTextAndAccept(t *testing.T) {
	doc, err := chart.Sniff([]byte(flintFile))
	if err != nil {
		t.Fatal(err)
	}
	text, ext, err := Text(Result{Kind: "chart", Chart: &doc})
	if err != nil || ext != ".json" || string(text) != flintFile {
		t.Fatalf("%q %s %v", text, ext, err)
	}
	for _, accept := range []string{"chart", "json", "json,chart"} {
		filter, err := ParseAccept(accept)
		if err != nil {
			t.Fatal(err)
		}
		if err := filter.Check("sales.json", []byte(flintFile)); err != nil {
			t.Fatalf("--accept %s: %v", accept, err)
		}
	}
	if filter, _ := ParseAccept("pdf"); filter.Check("sales.json", []byte(flintFile)) == nil {
		t.Fatal("--accept pdf took a chart")
	}
}

func TestChartKindLeavesJSONAlone(t *testing.T) {
	for _, raw := range []string{`{"type":"FeatureCollection","features":[]}`, `{"spec":"not a chart"}`, `[{"a":1}]`, `{"error":"not found","status":404}`, `{"spec":{"type":"ClusterIP"}}`} {
		if kind, err := Detect("data.json", []byte(raw), ""); err != nil || kind != "json" {
			t.Fatalf("%s: %s %v", raw, kind, err)
		}
	}
}
