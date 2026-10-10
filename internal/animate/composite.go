package animate

import (
	"image"
	"image/draw"
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
		p := s.positions[r.Destination]
		xdraw.CatmullRom.Scale(out, r.Rect, p.player.Image(), p.player.Image().Bounds(), draw.Over, nil)
	}
	return out
}
