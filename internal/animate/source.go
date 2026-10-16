package animate

import (
	"image"
	"time"
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
