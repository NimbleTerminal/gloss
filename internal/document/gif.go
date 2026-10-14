package document

import (
	"context"
	"image"

	animate "nimbleterminal.dev/ntpicture/animation"
)

const MaxGIFFrames = animate.MaxGIFFrames
const MaxGIFPixels = animate.MaxGIFPixels

type GIFPlayer = animate.Player

// decodeAnimation opens a GIF or WebP, playing it only when asked and when it
// has more than one frame; otherwise it yields the first composited frame.
func decodeAnimation(ctx context.Context, data []byte, play bool) (image.Image, *GIFPlayer, error) {
	if len(data) > MaxFileBytes {
		return nil, nil, ErrTooLarge
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return animate.DecodeContext(ctx, data, animate.Options{Animate: play})
}
