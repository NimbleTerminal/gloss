package chart

import "errors"

// ErrBlank is Raster's answer for a chart whose graphics draw nothing: the
// renderer cannot draw that kind of chart, and the text chart stands in.
var ErrBlank = errors.New("this chart cannot be drawn as graphics")

// MaxRasterEdge bounds the longer side of a chart drawn as graphics, in
// pixels; a larger pane gets the picture scaled up.
const MaxRasterEdge = 2400

// rasterSize fits w×h within MaxRasterEdge, keeping its shape.
func rasterSize(w, h int) (int, int) {
	if edge := max(w, h); edge > MaxRasterEdge {
		w, h = max(1, w*MaxRasterEdge/edge), max(1, h*MaxRasterEdge/edge)
	}
	return w, h
}
