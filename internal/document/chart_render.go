package document

import (
	"context"
	"image"
	"image/color"

	"nimbleterminal.dev/gloss/internal/chart"
)

// RenderChart compiles doc for a cols-by-rows grid and draws the frame
// with ChartImage at cellW by cellH pixels a cell, dark on white, as an
// export wants it.
func RenderChart(ctx context.Context, doc chart.Document, cols, rows, cellW, cellH int) (image.Image, chart.Result, error) {
	res, err := chart.Default.Compile(ctx, doc, cols, rows)
	if err != nil {
		return nil, res, err
	}
	frame, err := chart.RenderText(res.Spec)
	if err != nil {
		return nil, res, err
	}
	return ChartImage(frame, cols, rows, cellW, cellH, color.RGBA{32, 32, 32, 255}, color.RGBA{255, 255, 255, 255}), res, nil
}
