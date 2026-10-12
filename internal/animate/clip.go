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
// A frame is either decoded up front (Image) or when it is painted (Load),
// which keeps a long, large animation's memory at its canvases and one frame.
type Clip struct {
	Bounds     image.Rectangle
	Frames     []ClipFrame
	LoopCount  int // -1 once, 0 forever, positive repeats after the first pass.
	Background color.Color
}

type ClipFrame struct {
	Image    image.Image                 // Decoded pixels, or nil when Load supplies them; its bounds are Rect.
	Rect     image.Rectangle             // Placement on the canvas.
	Load     func() (image.Image, error) // Decodes the frame as it is painted; safe to call concurrently. A failure leaves the canvas as it was.
	Delay    time.Duration
	Disposal byte // 0/1 retain, 2 background, 3 previous.
	Blend    draw.Op
}

type Stats struct {
	Bounds        image.Rectangle
	Frames, Bytes int
}

// NewPlayer validates format-neutral decoded clips, counting paletted storage
// at one byte per pixel and RGBA/NRGBA at four; frames decoded on demand hold
// none. A decoded frame's Rect may be left for its image to fill in.
func NewPlayer(c *Clip) (*Player, error) {
	if c == nil || c.LoopCount < -1 || c.Bounds.Min != (image.Point{}) || c.Bounds.Empty() || c.Bounds.Dx() > MaxPixels/c.Bounds.Dy() || len(c.Frames) == 0 || len(c.Frames) > MaxGIFFrames {
		return nil, fmt.Errorf("invalid animation canvas or frame count")
	}
	copy := *c
	copy.Frames = append([]ClipFrame(nil), c.Frames...)
	bytes := 0
	for i := range copy.Frames {
		f := &copy.Frames[i]
		if f.Image != nil && f.Rect == (image.Rectangle{}) {
			f.Rect = f.Image.Bounds()
		}
		if f.Rect.Empty() || !f.Rect.In(c.Bounds) || f.Disposal > 3 || (f.Blend != draw.Over && f.Blend != draw.Src) || f.Delay <= 0 {
			return nil, fmt.Errorf("invalid animation frame")
		}
		if f.Image == nil {
			if f.Load == nil {
				return nil, fmt.Errorf("animation frame has no image or loader")
			}
			continue
		}
		if f.Image.Bounds() != f.Rect {
			return nil, fmt.Errorf("animation frame image does not match its placement")
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
	if copy.Background == nil {
		copy.Background = color.Transparent
	}
	return newPlayer(&copy), nil
}
