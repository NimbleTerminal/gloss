package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const unboundChart = `{"data":{"values":[]},"chart_spec":{"chartType":"Bar Chart","encodings":{"x":{"field":"region"},"y":{"field":"revenue","aggregate":"sum"}}}}`

func chartFile(t *testing.T, raw string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "chart.json")
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestChartOptionParses(t *testing.T) {
	opts, _, err := parse([]string{"--chart", chartFile(t, unboundChart), "../../examples/sales.csv"}, &bytes.Buffer{})
	if err != nil || opts.Chart == nil || !opts.Chart.Unbound || opts.ChartPath == "" {
		t.Fatalf("%+v %v", opts.Chart, err)
	}
	if _, _, err := parse([]string{"--chart", chartFile(t, `{"a":1}`), "x.csv"}, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "--chart") {
		t.Fatalf("plain JSON accepted: %v", err)
	}
	if _, _, err := parse([]string{"--chart", filepath.Join(t.TempDir(), "missing.json"), "x.csv"}, &bytes.Buffer{}); err == nil {
		t.Fatal("missing file accepted")
	}
}

func TestExportSheetWithChart(t *testing.T) {
	out := filepath.Join(t.TempDir(), "sales.png")
	opts, _, err := parse([]string{"--chart", chartFile(t, unboundChart), "--output", out, "--max-edge", "600", "../../examples/sales.csv"}, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if err := exportFiles(opts, &stdout, &stderr); err != nil {
		t.Fatalf("%v\n%s", err, stderr.String())
	}
	info, err := os.Stat(out)
	if err != nil || info.Size() == 0 || !strings.Contains(stderr.String(), "600×") {
		t.Fatalf("%v %s", err, stderr.String())
	}
	if dir := os.Getenv("CHART_PNG_DIR"); dir != "" {
		data, _ := os.ReadFile(out)
		_ = os.WriteFile(filepath.Join(dir, "sales-chart.png"), data, 0o644)
	}
}

func TestExportSheetWithoutChartStillRefuses(t *testing.T) {
	opts, _, _ := parse([]string{"--output", filepath.Join(t.TempDir(), "x.png"), "../../examples/sales.csv"}, &bytes.Buffer{})
	var stdout, stderr bytes.Buffer
	if err := exportFiles(opts, &stdout, &stderr); err == nil || !strings.Contains(stderr.String(), "no image") {
		t.Fatalf("%v %s", err, stderr.String())
	}
}

func TestChartOptionReportsMissingColumn(t *testing.T) {
	bad := strings.Replace(unboundChart, `"field":"region"`, `"field":"planet"`, 1)
	opts, _, _ := parse([]string{"--chart", chartFile(t, bad), "--output", filepath.Join(t.TempDir(), "x.png"), "../../examples/sales.csv"}, &bytes.Buffer{})
	var stdout, stderr bytes.Buffer
	if err := exportFiles(opts, &stdout, &stderr); err == nil || !strings.Contains(stderr.String(), "planet") {
		t.Fatalf("%v %s", err, stderr.String())
	}
}

func TestInfoDescribesTheChart(t *testing.T) {
	opts, _, _ := parse([]string{"--info", "--json", "--chart", chartFile(t, unboundChart), "../../examples/sales.csv"}, &bytes.Buffer{})
	var stdout, stderr bytes.Buffer
	if err := describe(opts, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	var files []struct {
		Details map[string]map[string]string `json:"details"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &files); err != nil {
		t.Fatal(err)
	}
	section := files[0].Details["Chart"]
	if section["Type"] != "bar" || section["Series"] != "1" || section["Rows"] != "8" || !strings.Contains(section["Source"], "chart.json") {
		t.Fatalf("%v", section)
	}
	// A chart file of its own is described too.
	opts, _, _ = parse([]string{"--info", "--json", chartFile(t, `{"data":{"values":[{"a":"x","b":1}]},"chart_spec":{"chartType":"Bar Chart","encodings":{"x":{"field":"a"},"y":{"field":"b"}}}}`)}, &bytes.Buffer{})
	stdout.Reset()
	if err := describe(opts, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(stdout.Bytes(), &files); err != nil || files[0].Details["Chart"]["Type"] != "bar" {
		t.Fatalf("%v %v", err, files[0].Details)
	}
	// As text, one Chart heading holds both what the file says and what it draws.
	opts.JSON = false
	stdout.Reset()
	if err := describe(opts, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(stdout.String(), "  Chart\n"); n != 1 || !strings.Contains(stdout.String(), "Type") || strings.Contains(stdout.String(), "Source") {
		t.Fatalf("%d Chart headings:\n%s", n, stdout.String())
	}
}
