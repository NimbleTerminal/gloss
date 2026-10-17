package app

import (
	"fmt"
	"image"
	"regexp"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	ntpicture "nimbleterminal.dev/ntpicture"
)

// Each picture shown with Kitty graphics goes under an id of its own: the
// browser app's terminal would otherwise keep showing the last picture of the
// same size. Glyphs have no ids to change.
func TestEachKittyPictureHasItsOwnID(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 64, 48))
	seen := map[int]bool{}
	last, err := ntpicture.NextImageID()
	if err != nil {
		t.Fatal(err)
	}
	capture := func(msg tea.Msg) {
		raw, ok := msg.(tea.RawMsg)
		if !ok {
			return
		}
		for _, header := range regexp.MustCompile(`\x1b_G([^;]+);`).FindAllStringSubmatch(fmt.Sprint(raw.Msg), -1) {
			if !strings.Contains(header[1], "a=T") {
				continue
			}
			match := regexp.MustCompile(`(?:^|,)i=(\d+)`).FindStringSubmatch(header[1])
			if len(match) != 2 {
				t.Fatal("transmission missing image ID")
			}
			id, _ := strconv.Atoi(match[1])
			if id <= last || id >= 1<<24 || seen[id] {
				t.Fatalf("invalid or reused image ID %d after %d", id, last)
			}
			seen[id] = true
			last = id
		}
	}
	glyph := New(Options{Render: "glyph"})
	defer glyph.Close()
	drainAnimation(t, glyph, glyph.showPicture(img), capture)
	if len(seen) != 0 {
		t.Fatal("glyph picture transmitted an image")
	}
	// New pagers and previews must not reset the allocator after a kept picture.
	for range 2 {
		m := New(Options{Render: "kitty"})
		m.Update(tea.WindowSizeMsg{Width: 40, Height: 12})
		drainAnimation(t, m, m.showPicture(img), capture)
		m.source, m.zoom = img, 1
		drainAnimation(t, m, m.refreshImage(), capture)
		drainAnimation(t, m, m.keepAnimationPicture(), capture)
		m.Close()
		id, err := ntpicture.NextImageID()
		if err != nil {
			t.Fatal(err)
		}
		if id <= last || id >= 1<<24 {
			t.Fatalf("shared allocator diverged: %d after %d", id, last)
		}
		last = id
	}
	if len(seen) != 4 {
		t.Fatalf("got %d picture transmissions, want 4", len(seen))
	}
}

// Every Kitty picture sends zlib-compressed raw pixels: the page, the Markdown
// view's inline pictures and a typeset overlay alike.
func TestKittyPicturesSendZlibFrames(t *testing.T) {
	m := typesetGIFModel(t, "kitty", 80, 30)

	if m.typesetLayers == nil || len(m.typesetLayers.Layers()) == 0 || m.typesetLayers.Layers()[0].Status.Picture.Format != ntpicture.KittyFormatZlib {
		t.Fatal("overlay format")
	}
	cmd, _ := m.documentKey("t") // Normal Markdown rendering, with the GIF as an inline picture.
	drainStill(t, m, cmd)
	if m.markdown == nil {
		t.Fatal("missing normal Markdown")
	}
	m.markdown.animators[0].Pause(true)
	drainAnimation(t, m, m.markdown.syncPictures(true))
	if m.markdown == nil || len(m.markdown.pictures) == 0 || m.markdown.animators[0].Status().Picture.Format != ntpicture.KittyFormatZlib {
		t.Fatalf("inline picture format: markdown %v", m.markdown != nil)
	}
}
