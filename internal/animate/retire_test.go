package animate

import (
	"testing"
	"time"

	"github.com/NimbleMarkets/ntcharts/v2/picture"
)

func init() { RetireDelay = 0 }

// The picture a presented one replaces is deleted only after the frame
// naming the new one has had time to go out.
func TestReplacedFrontIsRetiredAfterTheFrame(t *testing.T) {
	picture.ForceKittyCapability(picture.KittyCapabilitySupported)
	defer picture.ForceKittyCapability(picture.KittyCapabilityUnknown)
	RetireDelay = 20 * time.Millisecond
	defer func() { RetireDelay = 0 }()
	next := 1000
	pic := picture.New()
	pic.Toggle()
	a := New(&pic, Config{NextID: func() int { next += 1000; return next }})
	_, p, _ := DecodeGIF(gifTestData(t, 0), true)
	a.SetPlayer(p)
	a.Show(p.Image())
	a.Update(Presented{a, a.ID})
	first := a.Front
	a.Show(p.Image())
	cleanup, _ := a.Update(Presented{a, a.ID})
	if cleanup == nil || a.Front == first || first.Flight.Active.Load() {
		t.Fatal("second presented picture did not replace the first")
	}
	start := time.Now()
	if cleanup() == nil {
		t.Fatal("old picture never deleted")
	}
	if time.Since(start) < RetireDelay/2 {
		t.Fatal("old picture deleted before the frame could go out")
	}
}
