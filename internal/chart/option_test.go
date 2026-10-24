package chart

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

const heatmapSpec = `{"type":"heatmap","width":4,"height":2,"heat":{"matrix":[[1,2],[3,4]]}}`

func optionOf(t *testing.T, doc Document) map[string]any {
	t.Helper()
	raw, err := Default.Option(context.Background(), doc)
	if err != nil {
		t.Fatal(err)
	}
	var option map[string]any
	if err := json.Unmarshal(raw, &option); err != nil {
		t.Fatalf("%v in %s", err, raw)
	}
	return option
}

func TestOptionFromFlint(t *testing.T) {
	raw, err := os.ReadFile("testdata/sales-bar.json")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := Sniff(raw)
	if err != nil {
		t.Fatal(err)
	}
	option := optionOf(t, doc)
	series, _ := option["series"].([]any)
	if len(series) == 0 {
		t.Fatalf("no series in %v", option)
	}
	if first, _ := series[0].(map[string]any); first["type"] != "bar" {
		t.Fatalf("series %v", series[0])
	}
}

func TestOptionFromBarSpec(t *testing.T) {
	doc := sniffed(t, `{"type":"bar","title":"Sales","x_axis":{"labels":["a","b"]},"data":{"series":[{"name":"units","values":[{"y":1},{"y":2}]}]}}`)
	option := optionOf(t, doc)
	series, _ := option["series"].([]any)
	if len(series) != 1 {
		t.Fatalf("series %v", option["series"])
	}
}

func TestOptionRefusesTerminalOnlySpec(t *testing.T) {
	_, err := Default.Option(context.Background(), sniffed(t, heatmapSpec))
	if err == nil || !strings.Contains(err.Error(), "heatmap") {
		t.Fatalf("err %v", err)
	}
}

func TestOptionCleansStrings(t *testing.T) {
	doc := sniffed(t, "{\"type\":\"bar\",\"title\":\"Sa\\u001b[31mles\",\"x_axis\":{\"labels\":[\"a\"]},\"data\":{\"series\":[{\"name\":\"u\\u0007nits\",\"values\":[{\"y\":1}]}]}}")
	raw, err := Default.Option(context.Background(), doc)
	if err != nil {
		t.Fatal(err)
	}
	if strings.ContainsAny(string(raw), "\x1b\x07") || !strings.Contains(string(raw), "Sa[31mles") || !strings.Contains(string(raw), "units") {
		t.Fatalf("option %s", raw)
	}
}

// An option is hardened in Go, not only on the page: ECharts reads a
// tooltip from baseOption, media or options over the top level, and an
// HTML tooltip would put a formatter into the DOM.
func TestCleanJSONHardensTheOption(t *testing.T) {
	raw, err := cleanJSON(json.RawMessage(`{"baseOption":{"tooltip":{"renderMode":"html"}},"media":[{}],"options":[{}],"timeline":{},"tooltip":{"renderMode":"html","formatter":"<img src=x onerror=alert(1)>"},"series":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	var option map[string]any
	if err := json.Unmarshal(raw, &option); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"baseOption", "media", "options", "timeline"} {
		if _, there := option[key]; there {
			t.Errorf("%s survived", key)
		}
	}
	tooltip, _ := option["tooltip"].(map[string]any)
	if tooltip["renderMode"] != "richText" || tooltip["formatter"] != nil {
		t.Errorf("tooltip %v", tooltip)
	}
	if _, there := option["series"]; !there {
		t.Error("series lost")
	}
}

// A document's own ECharts option never reaches the page: like Compile,
// Option refuses a raster envelope.
func TestOptionRefusesRasterEnvelopes(t *testing.T) {
	_, err := Default.Option(context.Background(), Document{Kind: KindEnvelope, Raw: []byte(`{"echarts":{"series":[]},"warnings":[]}`)})
	if err == nil || !strings.Contains(err.Error(), "raster") {
		t.Fatalf("err %v", err)
	}
}

func TestOptionErrors(t *testing.T) {
	if _, err := Default.Option(context.Background(), sniffed(t, `{"error":{"message":"no"}}`)); err == nil || !strings.Contains(err.Error(), "no") {
		t.Errorf("error envelope: %v", err)
	}
	if _, err := Default.Option(context.Background(), Document{Kind: KindFlint, Unbound: true, Raw: []byte(`{"data":{"values":[]},"chart_spec":{"chartType":"Bar Chart","encodings":{}}}`)}); err == nil {
		t.Error("unbound Flint document compiled")
	}
}
