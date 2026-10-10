package animate

import (
	"context"
	"image"
	"image/color"
	"strings"
	"sync/atomic"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/NimbleMarkets/ntcharts/v2/picture"
	"github.com/charmbracelet/x/ansi"
)

// Wait is injectable for deterministic clocks. It must return promptly on cancel.
type Wait func(context.Context, time.Duration) bool
type Config struct {
	NextID     func() int // each allocation reserves 1,000 IDs
	Background color.Color
	Wait       Wait
}
type Tick struct {
	Owner *Animator
	Epoch uint64
}
type Frame struct {
	Tick
	Source Source
	Image  image.Image
	More   bool
}
type Presented struct {
	Owner *Animator
	ID    int
}

type Placement struct {
	Active  atomic.Bool
	cleanup tea.Cmd
}
type Front struct {
	Picture picture.Model
	ID      int
	View    string
	Image   image.Image
	Flight  *Placement
}

// Animator is owned by one event loop. Source snapshots and rendered images
// passed to commands are immutable. Capability/geometry messages belong to the
// host, which passes its picture model by pointer.
type Animator struct {
	Picture                             *picture.Model
	Source                              Source
	Player                              *Player // current standalone PlayerSource position, nil for composites
	Paused, Finished, Pending, Awaiting bool
	Epoch                               uint64
	Speed                               int
	ID                                  int
	Front                               *Front
	Flight                              *Placement
	Transform                           func(image.Image) image.Image // captured per composition, optional
	cfg                                 Config
	width, height                       int
	remaining, nextID                   int
	cancel                              context.CancelFunc
	image                               image.Image
	visible                             bool
}

func New(pic *picture.Model, cfg Config) *Animator {
	if cfg.Wait == nil {
		cfg.Wait = func(ctx context.Context, d time.Duration) bool {
			timer := time.NewTimer(d)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return false
			case <-timer.C:
				return true
			}
		}
	}
	return &Animator{Picture: pic, cfg: cfg, width: 1, height: 1}
}
func (a *Animator) SetSource(s Source) {
	a.CancelPending()
	a.Source = s
	a.Player = nil
	a.Finished = false
	if p, ok := s.(*PlayerSource); ok {
		a.Player = p.Player
		p.Speed = a.Speed
	}
}
func (a *Animator) CancelPending() {
	if a.cancel != nil {
		a.cancel()
		a.cancel = nil
	}
	a.Epoch++
	a.Pending = false
}
func (a *Animator) Rate() float64 { return float64(int(1)<<uint(a.Speed+2)) / 4 }
func (a *Animator) Delay() time.Duration {
	if a.Source == nil {
		return 0
	}
	return a.Source.Delay()
}
func (a *Animator) SetSpeed(speed int) {
	speed = max(-2, min(2, speed))
	if speed == a.Speed {
		return
	}
	a.CancelPending()
	a.Speed = speed
	if s, ok := a.Source.(*PlayerSource); ok {
		n := *s
		n.Speed = speed
		a.Source = &n
	}
}
func (a *Animator) SetPlayer(p *Player) { a.SetSource(&PlayerSource{Player: p, Speed: a.Speed}) }
func (a *Animator) Pause(paused bool)   { a.CancelPending(); a.Paused = paused }
func (a *Animator) Toggle() tea.Cmd {
	a.CancelPending()
	if a.Finished && a.Source != nil {
		a.Paused, a.Finished, a.Pending = false, false, true
		tick, s, transform := Tick{a, a.Epoch}, a.Source.Clone(), a.Transform
		return func() tea.Msg {
			s.Restart()
			img := s.Image()
			if transform != nil {
				img = transform(img)
			}
			return Frame{tick, s, img, true}
		}
	}
	a.Paused = !a.Paused
	return nil
}
func (a *Animator) Schedule(visible bool) tea.Cmd {
	a.visible = visible
	if !visible {
		if a.Pending {
			a.CancelPending()
		}
		return nil
	}
	if a.Source == nil || a.Paused || a.Finished || a.Pending || (a.Awaiting && a.Picture.Mode() == picture.PictureKitty) {
		return nil
	}
	if a.Source.Delay() <= 0 {
		a.Finished = true
		return nil
	}
	a.Pending = true
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	tick, delay, wait := Tick{a, a.Epoch}, a.Source.Delay(), a.cfg.Wait
	return func() tea.Msg {
		if wait(ctx, delay) {
			return tick
		}
		return nil
	}
}
func (a *Animator) Update(msg tea.Msg) (tea.Cmd, bool) {
	switch v := msg.(type) {
	case Tick:
		if v.Owner != a {
			return nil, false
		}
		if v.Epoch != a.Epoch {
			return nil, true
		}
		if !a.visible || a.Paused || a.Source == nil || (a.Awaiting && a.Picture.Mode() == picture.PictureKitty) {
			a.Pending = false
			return nil, true
		}
		s, transform := a.Source.Clone(), a.Transform
		return func() tea.Msg {
			img, _, more := s.Next()
			if more && transform != nil {
				img = transform(img)
			}
			return Frame{v, s, img, more}
		}, true
	case Frame:
		if v.Owner != a {
			return nil, false
		}
		if v.Epoch != a.Epoch {
			return nil, true
		}
		a.Pending = false
		if !a.visible || a.Paused {
			return nil, true
		}
		if !v.More {
			a.Finished = true
			return nil, true
		}
		a.Source = v.Source
		if p, ok := v.Source.(*PlayerSource); ok {
			a.Player = p.Player
		}
		return a.Show(v.Image), true
	case Presented:
		if v.Owner != a {
			return nil, false
		}
		if v.ID != a.ID || a.Picture.Mode() != picture.PictureKitty || a.Flight == nil || !a.Flight.Active.Load() {
			return nil, true
		}
		var cleanup tea.Cmd
		if a.Front != nil && a.Front.ID != a.ID {
			cleanup = a.ClearFront()
		}
		img := a.image
		if a.Source != nil {
			img = a.Source.Image()
		}
		a.Front = &Front{*a.Picture, a.ID, a.Picture.View().Content, img, a.Flight}
		a.Awaiting = false
		return cleanup, true
	}
	return nil, false
}

// Transmission wraps only an accepted picture command. Retired placements get
// a second ID-specific delete if a transmission completed after removal.
func (a *Animator) Transmission(msg tea.Msg, cmd tea.Cmd) tea.Cmd {
	f, ok := msg.(picture.KittyFrameMsg)
	if !ok || f.ID != a.ID || cmd == nil {
		return cmd
	}
	if a.Flight == nil {
		a.Flight = &Placement{}
		a.Flight.Active.Store(true)
	}
	id, flight := a.ID, a.Flight
	return tea.Sequence(cmd, func() tea.Msg {
		if !flight.Active.Load() && flight.cleanup != nil {
			return flight.cleanup()
		}
		return Presented{a, id}
	})
}
func retire(f *Placement, cmd tea.Cmd) {
	if f != nil && f.Active.Load() {
		f.cleanup = cmd
		f.Active.Store(false)
	}
}
func (a *Animator) Retire(cmd tea.Cmd) { retire(a.Flight, cmd); a.Flight = nil }
func (a *Animator) ClearFront() tea.Cmd {
	f := a.Front
	if f == nil {
		return nil
	}
	a.Front = nil
	cmd := f.Picture.SetImage(nil)
	retire(f.Flight, cmd)
	if a.Flight == f.Flight {
		a.Flight = nil
	}
	return cmd
}
func (a *Animator) Show(img image.Image) tea.Cmd {
	a.image = img
	if a.Picture.Mode() != picture.PictureKitty {
		a.Awaiting = false
		return tea.Batch(a.ClearFront(), a.Picture.SetSize(a.width, a.height), a.Picture.SetImage(img))
	}
	old := *a.Picture
	cw, ch := old.CellPixelSize()
	var cleanup tea.Cmd
	if a.Front != nil && a.Front.ID == a.ID {
		a.Flight = nil
	} else {
		cleanup = old.SetImage(nil)
		a.Retire(cleanup)
	}
	a.ID = a.NextImageID()
	if a.ID >= 1<<24 {
		a.Paused = true
		return cleanup
	}
	a.Awaiting = true
	a.Flight = &Placement{}
	a.Flight.Active.Store(true)
	*a.Picture = picture.NewWithConfig(picture.Config{KittyID: a.ID, KittyZ: -1, Background: a.cfg.Background, CellPixelWidth: cw, CellPixelHeight: ch, KittyResolutionFactor: old.KittyResolutionFactor(), KittyFormat: old.KittyFormat(), KittyMedium: old.KittyMedium()})
	return tea.Sequence(cleanup, a.Picture.SetSize(a.width, a.height), a.Picture.Toggle(), a.Picture.SetImage(img))
}
func (a *Animator) SetSize(w, h int)            { a.width, a.height = max(1, w), max(1, h) }
func (a *Animator) SetBackground(c color.Color) { a.cfg.Background = c }
func (a *Animator) Resize(w, h int) tea.Cmd {
	a.CancelPending()
	a.SetSize(w, h)
	if a.image == nil {
		return a.Picture.SetSize(w, h)
	}
	return a.Show(a.image)
}
func (a *Animator) View() string {
	if a.Picture.Mode() != picture.PictureKitty {
		return a.Picture.View().Content
	}
	if a.Front == nil {
		return ansi.Truncate("Preparing image…", a.width, "")
	}
	lines := strings.Split(a.Front.View, "\n")
	lines = lines[:min(len(lines), a.height)]
	for i := range lines {
		lines[i] = ansi.Truncate(lines[i], a.width, "")
	}
	return strings.Join(lines, "\n")
}
func (a *Animator) Stop() { a.CancelPending(); a.Source = nil; a.Player = nil; a.visible = false }
func (a *Animator) Clear() tea.Cmd {
	a.Stop()
	return a.Hide()
}
func (a *Animator) Keep() tea.Cmd {
	a.Stop()
	if a.Front == nil || a.Picture.Mode() != picture.PictureKitty {
		return a.ClearFront()
	}
	var cleanup tea.Cmd
	if a.Front.ID != a.ID {
		cleanup = a.Picture.SetImage(nil)
		a.Retire(cleanup)
	}
	*a.Picture, a.ID = a.Front.Picture, a.Front.ID
	a.Front, a.Flight = nil, nil
	return cleanup
}

func (a *Animator) NextImageID() int {
	if a.remaining == 0 {
		a.nextID, a.remaining = a.cfg.NextID(), 1000
	}
	id := a.nextID
	a.nextID++
	a.remaining--
	return id
}

// SetVisible updates lifecycle state without scheduling a timer.
func (a *Animator) SetVisible(visible bool) {
	a.visible = visible
	if !visible && a.Pending {
		a.CancelPending()
	}
}
func (a *Animator) Size() (int, int) { return a.width, a.height }

// Hide retires placements and pending work while preserving the playback position.
func (a *Animator) Hide() tea.Cmd {
	a.SetVisible(false)
	a.image = nil
	a.Awaiting = false
	front := a.ClearFront()
	pending := a.Picture.SetImage(nil)
	a.Retire(pending)
	return tea.Batch(front, pending)
}
func (a *Animator) HasImage() bool { return a.image != nil }
