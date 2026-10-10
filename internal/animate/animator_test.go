package animate

import (
	"context"
	"image"
	"image/color"
	"image/draw"
	"testing"
	"time"

	"github.com/NimbleMarkets/ntcharts/v2/picture"
)

func TestAnimatorClockEpochAndVisibility(t *testing.T) {
	_, player, err := DecodeGIF(gifTestData(t, -1), true)
	if err != nil {
		t.Fatal(err)
	}
	pic := picture.New()
	var waited time.Duration
	a := New(&pic, Config{Wait: func(ctx context.Context, d time.Duration) bool { waited = d; return ctx.Err() == nil }})
	a.SetPlayer(player)
	cmd := a.Schedule(true)
	if cmd == nil || a.Schedule(true) != nil {
		t.Fatal("expected exactly one timer")
	}
	tick := cmd()
	if waited != 100*time.Millisecond {
		t.Fatal(waited)
	}
	compose, ok := a.Update(tick)
	if !ok || compose == nil {
		t.Fatal("no composition")
	}
	stale := compose()
	a.Schedule(false)
	a.Update(stale)
	if a.Player.Frame() != 1 || a.Pending {
		t.Fatal("hidden animation accepted stale frame")
	}
	cmd = a.Schedule(true)
	compose, _ = a.Update(cmd())
	a.Update(compose())
	if a.Player.Frame() != 2 {
		t.Fatal("did not resume")
	}
	a.SetSpeed(2)
	if a.Delay() != 25*time.Millisecond {
		t.Fatal(a.Delay())
	}
	a.Pause(true)
	if a.Schedule(true) != nil {
		t.Fatal("paused timer")
	}
	a.Pause(false)
	a.Hide()
	if a.HasImage() || a.Player.Frame() != 2 {
		t.Fatal("hide lost position or retained image")
	}
	a.Clear()
	if a.Source != nil || a.Pending {
		t.Fatal("clear retained playback")
	}
}

func TestAnimatorKittyIDsPacingAndRetirement(t *testing.T) {
	picture.ForceKittyCapability(picture.KittyCapabilitySupported)
	defer picture.ForceKittyCapability(picture.KittyCapabilityUnknown)
	next := 1000
	cfg := Config{NextID: func() int { next += 1000; return next }}
	pic, other := picture.New(), picture.New()
	pic.Toggle()
	other.Toggle()
	a, b := New(&pic, cfg), New(&other, cfg)
	_, p, _ := DecodeGIF(gifTestData(t, 0), true)
	a.SetPlayer(p)
	b.SetPlayer(p)
	a.Show(p.Image())
	b.Show(p.Image())
	if a.ID == b.ID {
		t.Fatal("ID collision")
	}
	if a.Schedule(true) != nil {
		t.Fatal("scheduled before transmission")
	}
	a.Update(Presented{a, a.ID})
	front := a.Front
	if front == nil || a.Awaiting {
		t.Fatal("not presented")
	}
	a.Show(p.Image())
	if a.Front != front || !front.Flight.Active.Load() {
		t.Fatal("retired front too early")
	}
	a.Update(Presented{a, a.ID})
	if front.Flight.Active.Load() {
		t.Fatal("old front not retired")
	}
	pending := a.Flight
	a.Hide()
	if pending.Active.Load() || a.Front != nil {
		t.Fatal("hide retained placement")
	}
}

func rgbaPlayer(t *testing.T, d time.Duration) *Player {
	t.Helper()
	r := image.Rect(0, 0, 2, 2)
	red, transparent := image.NewRGBA(r), image.NewRGBA(r)
	draw.Draw(red, r, image.NewUniform(color.RGBA{255, 0, 0, 255}), image.Point{}, draw.Src)
	p, err := NewPlayer(&Clip{Bounds: r, Frames: []ClipFrame{{Image: red, Delay: d, Blend: draw.Src}, {Image: transparent, Delay: d, Blend: draw.Src}}, LoopCount: 0})
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func TestCompositeTimingTransparencyAndReflow(t *testing.T) {
	base := image.NewRGBA(image.Rect(0, 0, 8, 4))
	draw.Draw(base, base.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	regions := []Region{{image.Rect(0, 0, 2, 2), rgbaPlayer(t, 20*time.Millisecond), "a"}, {image.Rect(4, 0, 6, 2), rgbaPlayer(t, 50*time.Millisecond), "b"}}
	first := NewComposite(base, regions, nil)
	s := first.Clone().(*CompositeSource)
	s.Next()
	if s.positions["a"].player.Frame() != 2 || s.positions["b"].player.Frame() != 1 || s.Delay() != 20*time.Millisecond {
		t.Fatal("independent timing lost")
	}
	if s.Image().At(0, 0) != base.At(0, 0) || s.Image().At(7, 3) != base.At(7, 3) {
		t.Fatal("transparent frame retained old pixels or changed outside")
	}
	if first.positions["a"].player.Frame() != 1 {
		t.Fatal("mutated source snapshot")
	}
	s.Next()
	s.Next()
	if s.positions["b"].player.Frame() != 2 {
		t.Fatal("slower region not advanced")
	}
	hidden := NewComposite(base, nil, s)
	if hidden.Delay() != 0 {
		t.Fatal("offscreen animation running")
	}
	regions[0].Rect = image.Rect(1, 1, 3, 3)
	reflow := NewComposite(base, regions, hidden)
	if reflow.positions["b"].player != s.positions["b"].player {
		t.Fatal("reflow restarted playback")
	}
	reflow.Restart()
	if reflow.positions["b"].player.Frame() != 1 {
		t.Fatal("restart")
	}
}
func TestClipStorageBudget(t *testing.T) {
	// Shared backing storage still counts per frame: future RGBA decoders cannot
	// accidentally treat the GIF's byte budget as a four-times-larger pixel budget.
	img := image.NewRGBA(image.Rect(0, 0, 1024, 1024))
	c := &Clip{Bounds: img.Bounds()}
	for range 16 {
		c.Frames = append(c.Frames, ClipFrame{Image: img, Delay: time.Second, Blend: draw.Src})
	}
	if _, err := NewPlayer(c); err != nil {
		t.Fatal(err)
	}
	c.Frames = append(c.Frames, c.Frames[0])
	if _, err := NewPlayer(c); err == nil {
		t.Fatal("accepted over-budget RGBA clip")
	}
}
