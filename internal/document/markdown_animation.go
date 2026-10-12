package document

import (
	"context"
	"image"

	"nimbleterminal.dev/gloss/internal/animate"
)

const MaxMarkdownAnimations = 8
const MaxMarkdownAnimationBytes = 64 << 20

// Shared by both Markdown backends. Preflight counts are reserved before
// frames are decoded; duplicate destinations call this only once. A still
// GIF or WebP is not counted.
type markdownAnimations struct{ seen, eligible, over, bytes int }

func (b *markdownAnimations) decode(ctx context.Context, data []byte, play bool) (image.Image, *animate.Player, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
	}
	stats, err := animate.Preflight(data)
	if err != nil {
		b.seen++
		b.over++
		return decodeAnimation(data, false)
	}
	if stats.Frames < 2 {
		return decodeAnimation(data, false)
	}
	b.seen++
	if b.eligible >= MaxMarkdownAnimations || stats.Bytes > MaxMarkdownAnimationBytes-b.bytes {
		b.over++
		return decodeAnimation(data, false)
	}
	b.eligible++
	b.bytes += stats.Bytes
	return decodeAnimation(data, play)
}
func (b markdownAnimations) apply(md *Markdown) {
	md.Animations, md.Animated, md.AnimationOverBudget = b.seen, b.eligible, b.over
}
