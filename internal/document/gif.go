package document

import (
	"image"

	"nimbleterminal.dev/gloss/internal/animate"
)

const MaxGIFFrames = animate.MaxGIFFrames
const MaxGIFPixels = animate.MaxGIFPixels

type GIFPlayer = animate.Player

// decodeAnimation opens a GIF or WebP, playing it only when asked and when it
// has more than one frame; otherwise it yields the first composited frame.
func decodeAnimation(data []byte, play bool) (image.Image, *GIFPlayer, error) {
	if len(data) > MaxFileBytes {
		return nil, nil, ErrTooLarge
	}
	return animate.Decode(data, play)
}
