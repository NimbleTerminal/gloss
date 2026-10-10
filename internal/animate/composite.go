package animate

import (
	"image"
	"image/draw"
	"math"
	"time"

	xdraw "golang.org/x/image/draw"
)

type Region struct {
	Rect        image.Rectangle // full destination, possibly partly outside the viewport
	Player      *Player
	Destination string
}
type position struct {
	player   *Player
	due      time.Duration
	finished bool
}

// CompositeSource owns an immutable image of the page without its animated
// regions. This prevents transparent/disposed pixels from retaining frame one.
type CompositeSource struct {
	Base      image.Image
	Regions   []Region
	positions map[string]position
	image     image.Image
}

func NewComposite(base image.Image, regions []Region, previous *CompositeSource) *CompositeSource {
	s := &CompositeSource{Base: base, Regions: append([]Region(nil), regions...), positions: map[string]position{}}
	if previous != nil {
		for k, p := range previous.positions {
			s.positions[k] = p
		}
	}
	for _, r := range regions {
		if _, ok := s.positions[r.Destination]; !ok {
			s.positions[r.Destination] = position{player: r.Player, due: r.Player.Delay()}
		}
	}
	if len(regions) == 0 {
		s.image = base // Nothing to draw over it: the base is immutable and serves as the frame.
		return s
	}
	s.image = s.compose()
	return s
}
func (s *CompositeSource) Clone() Source {
	n := *s
	n.positions = map[string]position{}
	for k, p := range s.positions {
		n.positions[k] = p
	}
	return &n
}
func (s *CompositeSource) Image() image.Image { return s.image }

// Window is the part of s inside r, in r's own coordinates: the base under r
// and the regions reaching into r, playing on from s's positions. A viewer
// animates windows the size of the regions' cells, so a frame is the GIF's
// area rather than the whole page. A size other than r's resamples the base
// once and places the regions at that scale; a region within two pixels of
// its frames' own size takes exactly that size, so frames are copied, not
// resampled. A viewer whose page is drawn above a GIF's resolution asks for
// the window at the GIF's and lets the terminal enlarge it.
func (s *CompositeSource) Window(r image.Rectangle, size image.Point) *CompositeSource {
	r = r.Intersect(s.Base.Bounds())
	if size == (image.Point{}) {
		size = r.Size()
	}
	base := image.NewRGBA(image.Rect(0, 0, size.X, size.Y))
	switch {
	case size == r.Size():
		draw.Draw(base, base.Bounds(), s.Base, r.Min, draw.Src)
	case 2*size.X >= r.Dx() && size.X <= 2*r.Dx() && 2*size.Y >= r.Dy() && size.Y <= 2*r.Dy():
		xdraw.ApproxBiLinear.Scale(base, base.Bounds(), s.Base, r, draw.Src, nil) // Once per window, at a mild ratio.
	default:
		xdraw.CatmullRom.Scale(base, base.Bounds(), s.Base, r, draw.Src, nil)
	}
	fx, fy := float64(size.X)/float64(max(1, r.Dx())), float64(size.Y)/float64(max(1, r.Dy()))
	var regions []Region
	for _, region := range s.Regions {
		if !region.Rect.Overlaps(r) {
			continue
		}
		rr := region.Rect.Sub(r.Min)
		rect := image.Rect(int(math.Round(float64(rr.Min.X)*fx)), int(math.Round(float64(rr.Min.Y)*fy)), int(math.Round(float64(rr.Max.X)*fx)), int(math.Round(float64(rr.Max.Y)*fy)))
		if own := region.Player.Image().Bounds().Size(); abs(rect.Dx()-own.X) <= 2 && abs(rect.Dy()-own.Y) <= 2 {
			rect.Max = rect.Min.Add(own)
		}
		regions = append(regions, Region{Rect: rect, Player: region.Player, Destination: region.Destination})
	}
	return NewComposite(base, regions, s)
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// WithPositions is s at the playback positions of others, later ones
// winning, without recomposing: a page laid out again starts its regions
// where the windows showing them have reached.
func (s *CompositeSource) WithPositions(others ...*CompositeSource) *CompositeSource {
	n := *s
	n.positions = map[string]position{}
	for k, p := range s.positions {
		n.positions[k] = p
	}
	for _, o := range others {
		for k, p := range o.positions {
			n.positions[k] = p
		}
	}
	return &n
}
func (s *CompositeSource) active() map[string]bool {
	out := map[string]bool{}
	for _, r := range s.Regions {
		out[r.Destination] = true
	}
	return out
}
func (s *CompositeSource) Delay() time.Duration {
	d := time.Duration(1<<63 - 1)
	for name := range s.active() {
		p := s.positions[name]
		if !p.finished {
			d = min(d, p.due)
		}
	}
	if d == time.Duration(1<<63-1) {
		return 0
	}
	return d
}
func (s *CompositeSource) Next() (image.Image, time.Duration, bool) {
	delta := s.Delay()
	if delta == 0 {
		return s.image, 0, false
	}
	more := false
	for name := range s.active() {
		p := s.positions[name]
		if p.finished {
			continue
		}
		p.due -= delta
		if p.due <= 0 {
			next, ok := p.player.Next()
			if ok {
				p.player = next
				p.due = next.Delay()
				more = true
			} else {
				p.finished = true
			}
		}
		s.positions[name] = p
	}
	if !more && s.Delay() == 0 {
		return s.image, 0, false
	}
	s.image = s.compose()
	return s.image, s.Delay(), true
}
func (s *CompositeSource) Restart() {
	for name, p := range s.positions {
		p.player = p.player.Restart()
		p.due = p.player.Delay()
		p.finished = false
		s.positions[name] = p
	}
	s.image = s.compose()
}
func (s *CompositeSource) compose() image.Image {
	out := image.NewRGBA(s.Base.Bounds())
	draw.Draw(out, out.Bounds(), s.Base, s.Base.Bounds().Min, draw.Src)
	for _, r := range s.Regions {
		frame := s.positions[r.Destination].player.Image()
		fb := frame.Bounds()
		switch {
		case fb.Dx() == r.Rect.Dx() && fb.Dy() == r.Rect.Dy():
			draw.Draw(out, r.Rect, frame, fb.Min, draw.Over) // Resampling at 1:1 would change nothing and cost a scratch buffer.
		case 2*r.Rect.Dx() >= fb.Dx() && r.Rect.Dx() <= 2*fb.Dx() && 2*r.Rect.Dy() >= fb.Dy() && r.Rect.Dy() <= 2*fb.Dy():
			// Near its own size a frame is drawn every tick with the cheap scaler:
			// the kernel one costs a scratch buffer four times the frame and most
			// of a tick, for a difference the eye does not see at this ratio.
			xdraw.ApproxBiLinear.Scale(out, r.Rect, frame, fb, draw.Over, nil)
		default:
			xdraw.CatmullRom.Scale(out, r.Rect, frame, fb, draw.Over, nil)
		}
	}
	return out
}
