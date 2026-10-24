package handoff

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"nimbleterminal.dev/gloss/internal/app"
)

// unboundChart is a Flint bar chart of region and revenue, with its rows
// left for the table to supply.
const unboundChart = `{"data":{"values":[]},"chart_spec":{"chartType":"Bar Chart","encodings":{"x":{"field":"region"},"y":{"field":"revenue","aggregate":"sum"}}}}`

func postChart(t *testing.T, url, body string, tweak func(*http.Request)) (int, map[string]any) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if tweak != nil {
		tweak(req)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var reply map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&reply)
	return resp.StatusCode, reply
}

func TestServedChartPushDrawsAndReplies(t *testing.T) {
	s := serving(t, app.Options{Files: []string{"../../examples/sales.csv"}})
	term := connect(t, s)
	term.Until("sales.csv")
	code, reply := postChart(t, s.URL+"chart", unboundChart, nil)
	if code != http.StatusOK || reply["type"] != "bar" || reply["series"] != 1.0 || reply["rows"] != 8.0 || reply["warnings"] == nil || reply["error"] != nil {
		t.Fatalf("%d %v", code, reply)
	}
	term.Until("chart · flint")
}

func TestServedChartPushRejects(t *testing.T) {
	s := serving(t, app.Options{Files: []string{"../../examples/sales.csv"}})
	term := connect(t, s)
	term.Until("sales.csv")
	if code, reply := postChart(t, s.URL+"chart", `{"a":1}`, nil); code != http.StatusBadRequest || reply["error"] == nil {
		t.Errorf("plain JSON: %d %v", code, reply)
	}
	cross := func(r *http.Request) { r.Header.Set("Origin", "https://evil.example") }
	if code, _ := postChart(t, s.URL+"chart", unboundChart, cross); code != http.StatusForbidden {
		t.Errorf("foreign origin: %d", code)
	}
	if code, reply := postChart(t, s.URL+"chart", `{"error":{"message":"upstream said no"}}`, nil); code != http.StatusUnprocessableEntity || !strings.Contains(reply["error"].(string), "upstream said no") {
		t.Errorf("error envelope: %d %v", code, reply)
	}
	if code, _ := get(t, s.URL+"chart", nil); code != http.StatusNotFound {
		t.Errorf("GET chart before one is shown: %d", code)
	}
}

func TestServedChartPageFollowsThePane(t *testing.T) {
	s := serving(t, app.Options{Files: []string{"../../examples/sales.csv"}})
	term := connect(t, s)
	term.Until("sales.csv")
	if code, body := get(t, s.URL+"chart", nil); code != http.StatusNotFound || !strings.Contains(body, "no chart") {
		t.Fatalf("before: %d %q", code, body)
	}
	if code, reply := postChart(t, s.URL+"chart", unboundChart, nil); code != http.StatusOK {
		t.Fatalf("push: %d %v", code, reply)
	}
	resp, err := http.Get(s.URL + "chart")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/html") || resp.Header.Get("Content-Security-Policy") == "" {
		t.Fatalf("page: %d %v", resp.StatusCode, resp.Header)
	}
	if !strings.Contains(string(body), "echarts.init") || !strings.Contains(string(body), `"type":"bar"`) {
		t.Fatalf("page body %.200s", body)
	}
	// A terminal-only Spec is drawn in the terminal and refused here, by name.
	if code, _ := postChart(t, s.URL+"chart", `{"type":"heatmap","width":4,"height":2,"heat":{"matrix":[[1,2],[3,4]]}}`, nil); code != http.StatusOK {
		t.Fatalf("heatmap push: %d", code)
	}
	if code, body := get(t, s.URL+"chart", nil); code != http.StatusUnprocessableEntity || !strings.Contains(body, "heatmap") {
		t.Fatalf("heatmap page: %d %q", code, body)
	}
	if code, body := get(t, s.URL, nil); code != http.StatusOK || !strings.Contains(body, `href="chart"`) {
		t.Fatalf("viewer page offers no chart link: %d", code)
	}
}
