//go:build !js

package chart

import (
	"context"
	"errors"
	"testing"
)

func TestRasterDrawsAFlintChart(t *testing.T) {
	s := Analyze([]string{"region", "revenue"}, [][]string{{"North", "48.5"}, {"East", "61"}, {"South", "39.9"}, {"West", "30"}}, []int{0, 1})
	doc, err := s.Document(TemplateBar)
	if err != nil {
		t.Fatal(err)
	}
	img, err := Default.Raster(context.Background(), doc, 480, 300)
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 480 || b.Dy() != 300 {
		t.Fatalf("size %v", b)
	}
}

func TestRasterBoundsItsSize(t *testing.T) {
	if w, h := rasterSize(6000, 3000); w != MaxRasterEdge || h != MaxRasterEdge/2 {
		t.Fatalf("%dx%d", w, h)
	}
	if w, h := rasterSize(800, 600); w != 800 || h != 600 {
		t.Fatalf("%dx%d", w, h)
	}
	if _, err := Default.Raster(context.Background(), Document{}, 0, 10); err == nil {
		t.Fatal("a zero size drew")
	}
}

func TestRasterReportsBlankCharts(t *testing.T) {
	// go-analyze draws heatmaps as an empty plot.
	s := Analyze([]string{"region", "quarter", "units"}, [][]string{{"North", "Q1", "1"}, {"South", "Q1", "2"}, {"North", "Q2", "3"}, {"South", "Q2", "4"}}, []int{0, 1, 2})
	doc, err := s.Document(TemplateHeatmap)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Default.Raster(context.Background(), doc, 480, 300); !errors.Is(err, ErrBlank) {
		t.Fatalf("err = %v, want ErrBlank", err)
	}
}
