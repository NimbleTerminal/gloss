package document

import (
	"context"
	"image"

	"nimbleterminal.dev/gloss/internal/animate"
)

const MaxMarkdownAnimations = 8
const MaxMarkdownAnimationBytes = 64 << 20

// Shared by both Markdown backends. Preflight counts are reserved before
// DecodeAll; duplicate destinations call this only once.
type markdownAnimations struct{ gifs, eligible, over, bytes int }

func (b *markdownAnimations) decode(ctx context.Context, data []byte, play bool) (image.Image, *animate.Player, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
	}
	stats, err := animate.PreflightGIF(data)
	if err != nil {
		b.gifs++
		b.over++
		return decodeGIF(data, false)
	}
	if stats.Frames < 2 {
		return decodeGIF(data, false)
	}
	b.gifs++
	if b.eligible >= MaxMarkdownAnimations || stats.Bytes > MaxMarkdownAnimationBytes-b.bytes {
		b.over++
		return decodeGIF(data, false)
	}
	b.eligible++
	b.bytes += stats.Bytes
	return decodeGIF(data, play)
}
func (b markdownAnimations) apply(md *Markdown) {
	md.GIFs, md.Animated, md.AnimationOverBudget = b.gifs, b.eligible, b.over
}
