//go:build !js

package chart

import (
	"context"
	"errors"
	"fmt"
	"image"

	"github.com/NimbleMarkets/flint-ntcharts/raster"
)

// Raster draws doc as a picture of w×h pixels (within MaxRasterEdge): its
// ECharts option, as Option makes it, drawn by flint-ntcharts' raster
// renderer (go-analyze). A chart that draws nothing is ErrBlank.
func (c *Compiler) Raster(ctx context.Context, doc Document, w, h int) (image.Image, error) {
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("chart graphics: bad size %dx%d", w, h)
	}
	option, err := c.Option(ctx, doc)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	w, h = rasterSize(w, h)
	img, err := raster.Render(option, w, h)
	if errors.Is(err, raster.ErrBlank) {
		return nil, ErrBlank
	}
	return img, err
}
