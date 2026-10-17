package document

import (
	"context"
	"image"

	"nimbleterminal.dev/ntpicture/animation"
)

// decodeAnimation opens a GIF or WebP, playing it only when asked and when it
// has more than one frame; otherwise it yields the first composited frame.
func decodeAnimation(ctx context.Context, data []byte, play bool) (image.Image, *animation.Player, error) {
	if len(data) > MaxFileBytes {
		return nil, nil, ErrTooLarge
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return animation.DecodeContext(ctx, data, animation.Options{Animate: play})
}
