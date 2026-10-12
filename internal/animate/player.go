package animate

import (
	"context"
	"image"
	"image/draw"
	"time"
)

// Player is an immutable playback position. Next composes a fresh canvas;
// render/export commands may retain any earlier Image without a data race.
// The decoded palettes and frame rectangles are shared between positions.
type Player struct {
	data           *Clip
	canvas         *image.RGBA
	restore        *image.RGBA
	frame, repeats int
}

func (p *Player) Image() image.Image { return p.canvas }
func (p *Player) Frame() int         { return p.frame + 1 }
func (p *Player) Frames() int        { return len(p.data.Frames) }
func (p *Player) Delay() time.Duration {
	return p.data.Frames[p.frame].Delay
}

func (p *Player) Restart() *Player { return newPlayer(p.data) }

// Seek composes a one-based frame and resets the loop count. It owns its
// canvases, leaving earlier playback/export positions immutable. Replaying
// disposal uses at most two canvases, not a cache of every composed frame.
func (p *Player) Seek(ctx context.Context, frame int) (*Player, error) {
	frame = max(1, min(frame, p.Frames()))
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var n *Player
	if frame >= p.Frame() {
		copy := *p
		n = &copy
		n.repeats = 0
		n.canvas = image.NewRGBA(p.canvas.Bounds())
		draw.Draw(n.canvas, n.canvas.Bounds(), p.canvas, image.Point{}, draw.Src)
		if p.restore != nil {
			n.restore = image.NewRGBA(p.restore.Bounds())
			draw.Draw(n.restore, n.restore.Bounds(), p.restore, image.Point{}, draw.Src)
		}
	} else {
		n = newPlayer(p.data)
	}
	snapshot := n.restore
	for n.Frame() < frame {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		switch n.data.Frames[n.frame].Disposal {
		case DisposalBackground:
			draw.Draw(n.canvas, n.data.Frames[n.frame].Rect, image.NewUniform(n.data.Background), image.Point{}, draw.Src)
		case DisposalPrevious:
			if n.restore != nil {
				draw.Draw(n.canvas, n.canvas.Bounds(), n.restore, image.Point{}, draw.Src)
			}
		}
		n.frame++
		n.restore = nil
		if n.data.Frames[n.frame].Disposal == DisposalPrevious {
			if snapshot == nil {
				snapshot = image.NewRGBA(n.canvas.Bounds())
			}
			draw.Draw(snapshot, snapshot.Bounds(), n.canvas, image.Point{}, draw.Src)
			n.restore = snapshot
		}
		n.data.Frames[n.frame].paint(n.canvas)
	}
	return n, ctx.Err()
}

// Next honors Go's GIF loop semantics: -1 plays once, 0 repeats forever,
// and a positive count repeats that many times after the first pass.
func (p *Player) Next() (*Player, bool) {
	n := *p
	n.frame++
	if n.frame == len(p.data.Frames) {
		if p.data.LoopCount < 0 || (p.data.LoopCount > 0 && p.repeats >= p.data.LoopCount) {
			return p, false
		}
		n = *newPlayer(p.data)
		n.repeats = p.repeats + 1
		return &n, true
	}
	n.canvas = image.NewRGBA(p.canvas.Bounds())
	draw.Draw(n.canvas, n.canvas.Bounds(), p.canvas, image.Point{}, draw.Src)
	switch p.data.Frames[p.frame].Disposal {
	case DisposalBackground:
		draw.Draw(n.canvas, p.data.Frames[p.frame].Rect, image.NewUniform(p.data.Background), image.Point{}, draw.Src)
	case DisposalPrevious:
		if p.restore != nil {
			draw.Draw(n.canvas, n.canvas.Bounds(), p.restore, image.Point{}, draw.Src)
		}
	}
	n.paint()
	return &n, true
}

func newPlayer(data *Clip) *Player {
	p := &Player{data: data, canvas: image.NewRGBA(data.Bounds)}
	draw.Draw(p.canvas, p.canvas.Bounds(), image.NewUniform(data.Background), image.Point{}, draw.Src)
	p.paint()
	return p
}

func (p *Player) paint() {
	p.restore = nil
	if p.data.Frames[p.frame].Disposal == DisposalPrevious {
		p.restore = image.NewRGBA(p.canvas.Bounds())
		draw.Draw(p.restore, p.restore.Bounds(), p.canvas, image.Point{}, draw.Src)
	}
	p.data.Frames[p.frame].paint(p.canvas)
}

// paint draws the frame at its placement, decoding it first when it is held
// compressed. A frame that fails to decode, or decodes to the wrong size,
// leaves the canvas as it was; its delay and disposal still apply.
func (f ClipFrame) paint(canvas draw.Image) {
	img := f.Image
	if img == nil {
		decoded, err := f.Load()
		if err != nil || decoded == nil || decoded.Bounds().Size() != f.Rect.Size() {
			return
		}
		img = decoded
	}
	draw.Draw(canvas, f.Rect, img, img.Bounds().Min, f.Blend)
}
