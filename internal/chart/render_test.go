package chart

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
)

func TestRenderTextFillsTheArea(t *testing.T) {
	res, err := Default.Compile(context.Background(), sniffed(t, flintDoc), 40, 12)
	if err != nil {
		t.Fatal(err)
	}
	view, err := RenderText(res.Spec)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(view, "\n")
	if len(lines) != 12 {
		t.Fatalf("%d lines:\n%s", len(lines), view)
	}
	for _, l := range lines {
		if ansi.StringWidth(l) > 40 {
			t.Fatalf("line wider than 40: %q", l)
		}
	}
	if !strings.Contains(ansi.Strip(view), "x") {
		t.Fatalf("category label missing:\n%s", view)
	}
}

func ansiStrip(s string) string { return ansi.Strip(s) }

// The widest sheet inference allows compiles well inside the deadline.
func TestCompileWideSheetWithinDeadline(t *testing.T) {
	header := []string{"day"}
	for i := 0; i < MaxSeries; i++ {
		header = append(header, "m"+strings.Repeat("x", i))
	}
	rows := make([][]string, MaxRows)
	for r := range rows {
		rows[r] = make([]string, len(header))
		rows[r][0] = "2026-01-01"
		for c := 1; c < len(header); c++ {
			rows[r][c] = "1"
		}
	}
	s := Analyze(header, rows, visible(len(header)))
	doc, err := s.Document(TemplateLine)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if _, err := Default.Compile(context.Background(), doc, 80, 20); err != nil {
		t.Fatal(err)
	}
	if d := time.Since(start); d > Deadline/2 {
		t.Fatalf("took %v", d)
	}
}

// Every template inference offers compiles and draws.
func TestEveryTemplateDraws(t *testing.T) {
	ohlc := [][]string{{"day", "open", "high", "low", "close"}}
	for i := 1; i <= 6; i++ {
		ohlc = append(ohlc, []string{"2026-01-0" + string(rune('0'+i)), "1", "3", "0.5", "2"})
	}
	for _, tc := range []struct {
		name   string
		header []string
		rows   [][]string
	}{
		{"sales", sales[0], sales[1:]},
		{"bar", []string{"region", "revenue"}, [][]string{{"N", "1"}, {"S", "2"}, {"N", "3"}}},
		{"heatmap", []string{"region", "quarter", "revenue"}, [][]string{{"N", "Q1", "1"}, {"S", "Q1", "2"}, {"N", "Q2", "3"}}},
		{"scatter", []string{"units", "revenue"}, [][]string{{"1", "2"}, {"3", "4"}, {"5", "1"}}},
		{"single", []string{"revenue"}, [][]string{{"1"}, {"2"}, {"2"}, {"5"}}},
		{"ohlc", ohlc[0], ohlc[1:]},
	} {
		s := Analyze(tc.header, tc.rows, visible(len(tc.header)))
		for _, template := range s.Templates() {
			doc, err := s.Document(template)
			if err != nil {
				t.Fatalf("%s %s: %v", tc.name, template, err)
			}
			res, err := Default.Compile(context.Background(), doc, 60, 16)
			if err != nil {
				t.Fatalf("%s %s: compile: %v", tc.name, template, err)
			}
			if _, err := RenderText(res.Spec); err != nil {
				t.Fatalf("%s %s: render: %v", tc.name, template, err)
			}
		}
	}
}
