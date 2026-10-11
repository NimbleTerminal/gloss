package document

import (
	"image"
	"nimbleterminal.dev/gloss/internal/animate"
)

const MaxGIFFrames = animate.MaxGIFFrames
const MaxGIFPixels = animate.MaxGIFPixels

type GIFPlayer = animate.Player

func decodeGIF(data []byte, play bool) (image.Image, *GIFPlayer, error) {
	if len(data) > MaxFileBytes {
		return nil, nil, ErrTooLarge
	}
	return animate.DecodeGIF(data, play)
}
