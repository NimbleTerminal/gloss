package animate

import (
	"context"
	"image"
	"time"

	"nimbleterminal.dev/ntpicture/animation"
)

// Source snapshots isolate off-loop composition from the event loop. Clone must
// retain immutable images and copy mutable playback state.
type Source interface {
	Next() (image.Image, time.Duration, bool)
	Restart()
	Clone() Source
	Image() image.Image
	Delay() time.Duration
}

type PlayerSource struct {
	Player *Player
	Speed  int // powers of two, -2..2
}

// PictureSource bridges ntpicture's immutable composite positions to the
// legacy Gloss scheduler while the event-loop controller migration proceeds.
type PictureSource struct{ Composite *animation.Composite }

func NewPictureSource(c *animation.Composite) *PictureSource { return &PictureSource{Composite: c} }
func (s *PictureSource) Clone() Source {
	if s == nil || s.Composite == nil {
		return &PictureSource{}
	}
	return &PictureSource{Composite: s.Composite}
}
func (s *PictureSource) Image() image.Image   { return s.Composite.Image() }
func (s *PictureSource) Delay() time.Duration { return s.Composite.Delay() }
func (s *PictureSource) Next() (image.Image, time.Duration, bool) {
	n, more, err := s.Composite.Advance(context.Background())
	if err != nil {
		return s.Image(), 0, false
	}
	s.Composite = n.(*animation.Composite)
	return s.Image(), s.Delay(), more
}
func (s *PictureSource) Restart() {
	n, err := s.Composite.Reset(context.Background())
	if err == nil {
		s.Composite = n.(*animation.Composite)
	}
}

func (s *PlayerSource) Clone() Source      { n := *s; return &n }
func (s *PlayerSource) Image() image.Image { return s.Player.Image() }
func (s *PlayerSource) Delay() time.Duration {
	return time.Duration(float64(s.Player.Delay()) / (float64(int(1)<<uint(max(-2, min(2, s.Speed))+2)) / 4))
}
func (s *PlayerSource) Next() (image.Image, time.Duration, bool) {
	p, more := s.Player.Next()
	s.Player = p
	return p.Image(), s.Delay(), more
}
func (s *PlayerSource) Restart() { s.Player = s.Player.Restart() }
