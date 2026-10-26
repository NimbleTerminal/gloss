//go:build js

package chart

import (
	"context"
	"errors"
	"image"
)

// Raster is native only: the browser build leaves the renderer out, and
// its chart page draws with ECharts itself.
func (c *Compiler) Raster(context.Context, Document, int, int) (image.Image, error) {
	return nil, errors.New("chart graphics are not available in the browser: ctrl+o opens the chart page")
}
