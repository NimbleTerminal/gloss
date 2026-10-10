// Package animate provides bounded decoded clips and terminal playback. It does
// not read the terminal, choose capabilities, or depend on an application.
package animate

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"time"
)

const (
	DisposalNone       byte = 1
	DisposalBackground byte = 2
	DisposalPrevious   byte = 3
)

// Clip and its frames are immutable after construction. A decoder owns the
// image storage; callers must not mutate it while a Player can reference it.
type Clip struct {
	Bounds     image.Rectangle
	Frames     []ClipFrame
	LoopCount  int // -1 once, 0 forever, positive repeats after the first pass.
	Background color.Color
}

type ClipFrame struct {
	Image    image.Image // Its bounds are its placement rectangle on the canvas.
	Delay    time.Duration
	Disposal byte // 0/1 retain, 2 background, 3 previous.
	Blend    draw.Op
}

type Stats struct {
	Bounds        image.Rectangle
	Frames, Bytes int
}

// NewPlayer validates format-neutral decoded clips, counting paletted storage
// at one byte per pixel and RGBA/NRGBA at four. GIF decoding preflights first.
func NewPlayer(c *Clip) (*Player, error) {
	if c == nil || c.LoopCount < -1 || c.Bounds.Min != (image.Point{}) || c.Bounds.Empty() || c.Bounds.Dx() > MaxPixels/c.Bounds.Dy() || len(c.Frames) == 0 || len(c.Frames) > MaxGIFFrames {
		return nil, fmt.Errorf("invalid animation canvas or frame count")
	}
	bytes := 0
	for _, f := range c.Frames {
		if f.Image == nil || f.Image.Bounds().Empty() || !f.Image.Bounds().In(c.Bounds) || f.Disposal > 3 || (f.Blend != draw.Over && f.Blend != draw.Src) || f.Delay <= 0 {
			return nil, fmt.Errorf("invalid animation frame")
		}
		var storage int
		switch img := f.Image.(type) {
		case *image.Paletted:
			storage = len(img.Pix)
		case *image.RGBA:
			storage = len(img.Pix)
		case *image.NRGBA:
			storage = len(img.Pix)
		default:
			return nil, fmt.Errorf("animation frames must be paletted or RGBA")
		}
		if storage > MaxGIFPixels-bytes {
			return nil, fmt.Errorf("animation exceeds 64 MiB decoded frame storage")
		}
		bytes += storage
	}
	copy := *c
	copy.Frames = append([]ClipFrame(nil), c.Frames...)
	if copy.Background == nil {
		copy.Background = color.Transparent
	}
	return newPlayer(&copy), nil
}
