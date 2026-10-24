package chart

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/NimbleMarkets/ntcharts/v2/spec"
)

func sniffed(t *testing.T, raw string) Document {
	t.Helper()
	doc, err := Sniff([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func TestCompileFlintToSpecAtSize(t *testing.T) {
	res, err := Default.Compile(context.Background(), sniffed(t, flintDoc), 60, 15)
	if err != nil {
		t.Fatal(err)
	}
	if res.Spec.Type != spec.ChartTypeBar || res.Spec.Width != 60 || res.Spec.Height != 15 || len(res.Spec.Data.Series) != 1 || res.Warnings == nil {
		t.Fatalf("%+v", res)
	}
}

func TestCompilePassesSpecsThrough(t *testing.T) {
	for _, raw := range []string{specDoc, `{"spec":` + specDoc + `,"warnings":[{"severity":"info","code":"x","message":"m"}],"size":{"width":40,"height":10}}`} {
		res, err := Default.Compile(context.Background(), sniffed(t, raw), 30, 8)
		if err != nil || res.Spec.Type != spec.ChartTypeBar || res.Spec.Width != 30 || res.Spec.Height != 8 {
			t.Fatalf("%s: %+v %v", raw, res, err)
		}
		if strings.HasPrefix(raw, `{"spec"`) && (len(res.Warnings) != 1 || res.Warnings[0].Message != "m") {
			t.Fatalf("envelope warnings lost: %+v", res.Warnings)
		}
	}
}

func TestCompileErrors(t *testing.T) {
	ctx := context.Background()
	if _, err := Default.Compile(ctx, sniffed(t, `{"error":{"message":"upstream said no"}}`), 10, 5); err == nil || !strings.Contains(err.Error(), "upstream said no") {
		t.Fatalf("error envelope: %v", err)
	}
	if _, err := Default.Compile(ctx, sniffed(t, `{"data":{"values":[]},"chart_spec":{"chartType":"Bar Chart","encodings":{}}}`), 10, 5); err == nil || !strings.Contains(err.Error(), "no data rows") {
		t.Fatalf("unbound: %v", err)
	}
	bad := strings.Replace(flintDoc, "Bar Chart", "Rose Chart", 1)
	if _, err := Default.Compile(ctx, sniffed(t, bad), 10, 5); err == nil || !strings.Contains(err.Error(), "Rose Chart") {
		t.Fatalf("unknown template: %v", err)
	}
	if _, err := Default.Compile(ctx, sniffed(t, `{"type":"bar","data":{"series":[]}}`), 10, 5); err == nil {
		t.Fatal("empty spec validated")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := Default.Compile(cancelled, sniffed(t, flintDoc), 10, 5); err == nil {
		t.Fatal("cancelled context compiled")
	}
}

// Cell text and chart JSON are untrusted: what reaches the view carries no
// control sequences, however it got into the chart.
func TestCompileCleansStrings(t *testing.T) {
	hostile := `N\u001b]52;c;QUJD\u0007`
	for _, raw := range []string{
		`{"data":{"values":[{"a":"` + hostile + `","b":1}]},"chart_spec":{"title":"\u001b[2J","chartType":"Bar Chart","encodings":{"x":{"field":"a"},"y":{"field":"b"}}}}`,
		`{"type":"bar","title":"\u001b[2J","x_axis":{"type":"category","labels":["` + hostile + `"]},"data":{"series":[{"name":"` + hostile + `","values":[{"y":1}]}]}}`,
	} {
		res, err := Default.Compile(context.Background(), sniffed(t, raw), 60, 12)
		if err != nil {
			t.Fatal(err)
		}
		view, err := RenderText(res.Spec)
		if err != nil {
			t.Fatal(err)
		}
		for _, bad := range []string{"\x1b]", "\x07", "[2J"} {
			if strings.Contains(view, bad) {
				t.Fatalf("%q reached the view:\n%s", bad, view)
			}
		}
		if !strings.Contains(ansiStrip(view), "N") {
			t.Fatalf("label lost entirely:\n%s", view)
		}
	}
}

// Bars are drawn as coloured cells: a series without a colour of its own
// takes one from the palette, or it is invisible.
func TestCompileGivesSeriesColours(t *testing.T) {
	s := Analyze(sales[0], sales[1:], []int{0, 3})
	doc, err := s.Document(TemplateBar)
	if err != nil {
		t.Fatal(err)
	}
	res, err := Default.Compile(context.Background(), doc, 40, 8)
	if err != nil {
		t.Fatal(err)
	}
	view, err := RenderText(res.Spec)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(view, "\x1b[38;2;") || !strings.Contains(view, "█") {
		t.Fatalf("bars have no colour:\n%q", view)
	}
	// A raw spec with an explicit series colour keeps it.
	res, _ = Default.Compile(context.Background(), sniffed(t, `{"type":"bar","x_axis":{"labels":["a"]},"data":{"series":[{"name":"s","color":"#123456","values":[{"y":1}]}]}}`), 20, 6)
	if res.Spec.Data.Series[0].Color != "#123456" {
		t.Fatalf("colour replaced: %q", res.Spec.Data.Series[0].Color)
	}
}

func TestCompileLargeDocumentWithinDeadline(t *testing.T) {
	var b strings.Builder
	b.WriteString(`{"data":{"values":[`)
	for i := 0; i < 10000; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(`{"t":"2026-01-01","v":` + strings.Repeat("1", 1+i%5) + `}`)
	}
	b.WriteString(`]},"semantic_types":{"t":"Date","v":"Quantity"},"chart_spec":{"chartType":"Line Chart","encodings":{"x":{"field":"t"},"y":{"field":"v"}}}}`)
	start := time.Now()
	if _, err := Default.Compile(context.Background(), sniffed(t, b.String()), 80, 20); err != nil {
		t.Fatal(err)
	}
	if d := time.Since(start); d > Deadline {
		t.Fatalf("took %v", d)
	}
}

func TestCompileRefusesBoundDocumentOverCap(t *testing.T) {
	doc := sniffed(t, `{"chart_spec":{"chartType":"Bar Chart","encodings":{"x":{"field":"name"},"y":{"field":"n"}}}}`)
	long := strings.Repeat("x", 4<<10)
	rows := make([][]string, 5000) // 20 MB of names, past the cap once bound.
	for i := range rows {
		rows[i] = []string{long, "1"}
	}
	bound, _, err := Bind(doc, []string{"name", "n"}, rows, []int{0, 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(bound.Raw) <= MaxDocumentBytes {
		t.Fatalf("bound document is %d bytes; the test needs it past the cap", len(bound.Raw))
	}
	if _, err := Default.Compile(context.Background(), bound, 80, 20); err == nil || !strings.Contains(err.Error(), "16 MiB") {
		t.Fatalf("got %v, want the 16 MiB refusal", err)
	}
	if _, err := Default.Option(context.Background(), bound); err == nil || !strings.Contains(err.Error(), "16 MiB") {
		t.Fatalf("Option got %v, want the 16 MiB refusal", err)
	}
}
