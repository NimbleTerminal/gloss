package document

import (
	"github.com/NimbleTerminal/gloss/internal/animate"
	"image"
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
