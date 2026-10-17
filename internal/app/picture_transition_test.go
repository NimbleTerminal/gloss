package app

import (
	"image"
	"testing"

	tea "charm.land/bubbletea/v2"
	"nimbleterminal.dev/gloss/internal/document"
)

func stillKitty(t *testing.T) *Model {
	t.Helper()
	m := New(Options{Files: []string{"page.png"}, Render: "kitty", Page: 1})
	t.Cleanup(func() { m.Close() })
	m.Update(tea.WindowSizeMsg{Width: 40, Height: 12})
	m.source = image.NewRGBA(image.Rect(0, 0, 100, 100))
	draw := m.refreshImage()
	if glyphs(m.pictureView()) {
		t.Fatal("initial image flashed glyphs")
	}
	drainAnimation(t, m, draw)
	if !m.hasPictureFront() || glyphs(m.pictureView()) {
		t.Fatal("initial image was not presented")
	}
	return m
}

func TestStillKittyRetainsFrontAcrossReplacements(t *testing.T) {
	m := stillKitty(t)
	front := m.pictureView()
	draw := m.refreshImage()
	m.zoom = 1
	zoom := m.refreshImage()
	if m.pictureView() != front || !m.pictureBusy() {
		t.Fatal("replacement lost retained front")
	}
	// Picture coalesces the latest request behind the one active preparation.
	drainAnimation(t, m, draw)
	drainAnimation(t, m, zoom)
	if m.pictureView() == front || m.pictureBusy() || glyphs(m.pictureView()) {
		t.Fatal("latest replacement did not settle")
	}
	status := m.still.pic.Status()
	if status.Presented != status.Generation {
		t.Fatal("obsolete generation presented")
	}
}

func TestStillKittyPageLoadRetainsFront(t *testing.T) {
	m := stillKitty(t)
	front := m.pictureView()
	m.loading = true
	body, _ := m.body(m.width, m.bodyHeight())
	if glyphs(body) || body == "" {
		t.Fatal("loading did not retain the image")
	}
	cmd := m.loaded(document.Result{Generation: m.generation, Kind: "pdf", Image: m.source, Page: 2, Pages: 3})
	if m.pictureView() != front {
		t.Fatal("page load retired the image before replacement")
	}
	drainAnimation(t, m, cmd)
	if m.pictureView() == front || m.pictureBusy() {
		t.Fatal("page replacement did not settle")
	}
}

func TestStillKittyPendingCleanup(t *testing.T) {
	for _, action := range []string{"clear", "glyph", "resize", "keep"} {
		t.Run(action, func(t *testing.T) {
			m := stillKitty(t)
			old := m.still.pic
			front := m.pictureView()
			draw := m.refreshImage()
			var cmd tea.Cmd
			switch action {
			case "clear":
				cmd = m.clearPicture()
			case "glyph":
				_, cmd = m.Update(tea.KeyPressMsg{Code: 'g', Text: "g"})
			case "resize":
				_, cmd = m.Update(tea.WindowSizeMsg{Width: 20, Height: 8})
			case "keep":
				cmd = m.keepAnimationPicture()
			}
			drainAnimation(t, m, cmd)
			drainAnimation(t, m, draw)
			if old.Status().Busy {
				t.Fatal("late work did not settle")
			}
			switch action {
			case "clear":
				if m.hasPictureFront() || old.View() != "" {
					t.Fatal("late replacement restored cleared image")
				}
			case "glyph":
				if !glyphs(m.pictureView()) {
					t.Fatal("glyph fallback did not render")
				}
			case "resize":
				if !m.pictureExact(20, m.bodyHeight()) || glyphs(m.pictureView()) {
					t.Fatal("resize did not present Kitty image")
				}
			case "keep":
				if m.pictureView() != front {
					t.Fatal("late replacement changed kept image")
				}
			}
		})
	}
}

// Export metadata follows the committed picture, not a newer source still
// being prepared. This is also the source retained across a coalesced redraw.
func TestStillExportSourceFollowsPresentation(t *testing.T) {
	m := stillKitty(t)
	first := m.source
	second := image.NewRGBA(image.Rect(0, 0, 80, 60))
	m.source = second
	cmd := m.refreshImage()
	if m.still.front != first {
		t.Fatal("pending redraw changed export source")
	}
	drainAnimation(t, m, cmd)
	if m.still.front != second {
		t.Fatal("presented redraw did not update export source")
	}
}

func TestStillAndPlaybackTransferOwnership(t *testing.T) {
	m := animated(t, "kitty", 0)
	player := m.animation.player()
	oldPlayback := m.playback
	m.stopAnimation()
	drainAnimation(t, m, m.showPicture(m.source))
	if m.still == nil || !m.hasPictureFront() || m.playback != nil || !oldPlayback.Closed() {
		t.Fatal("still did not replace playback")
	}
	old := m.still.pic
	pending := m.refreshImage()
	cmd := m.startAnimation(player)
	m.animation.Pause(true)
	drainAnimation(t, m, cmd)
	drainAnimation(t, m, pending)
	if m.playback.Status().Presented == nil || m.still != nil || old.View() != "" {
		t.Fatal("late still work changed playback")
	}
}
