package document

import (
	"context"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"nimbleterminal.dev/gloss/internal/chart"
)

func TestRenderChartDrawsTheCompiledFrame(t *testing.T) {
	doc, err := chart.Sniff([]byte(flintFile))
	if err != nil {
		t.Fatal(err)
	}
	img, res, err := RenderChart(context.Background(), doc, 60, 18, ChartCellWidth, ChartCellHeight)
	if err != nil || res.Spec.Type != "bar" {
		t.Fatalf("%v %+v", err, res.Spec.Type)
	}
	b := img.Bounds()
	if b.Dx() != 60*ChartCellWidth || b.Dy() != 18*ChartCellHeight {
		t.Fatalf("%v", b)
	}
	// Something was drawn: not every pixel is white.
	white := true
	rgba := img.(interface{ RGBAAt(int, int) color.RGBA })
	for y := 0; y < b.Dy() && white; y += 3 {
		for x := 0; x < b.Dx(); x += 3 {
			if rgba.RGBAAt(x, y) != (color.RGBA{255, 255, 255, 255}) {
				white = false
				break
			}
		}
	}
	if white {
		t.Fatal("blank chart")
	}
	if dir := os.Getenv("CHART_PNG_DIR"); dir != "" {
		f, err := os.Create(filepath.Join(dir, "render-chart.png"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		_ = png.Encode(f, img)
	}
}

func TestExportImageOfAChart(t *testing.T) {
	doc, _ := chart.Sniff([]byte(flintFile))
	img, err := ExportImage(Result{Kind: "chart", Chart: &doc}, 512)
	if err != nil || max(img.Bounds().Dx(), img.Bounds().Dy()) != 512 {
		t.Fatalf("%v %v", err, img.Bounds())
	}
	bad, _ := chart.Sniff([]byte(`{"error":{"message":"upstream said no"}}`))
	if _, err := ExportImage(Result{Kind: "chart", Chart: &bad}, 512); err == nil {
		t.Fatal("an error envelope exported")
	}
}
