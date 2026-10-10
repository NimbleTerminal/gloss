package app

import (
	"image"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/NimbleMarkets/ntcharts/v2/picture"
	"github.com/NimbleTerminal/gloss/internal/document"
)

func stillKitty(t *testing.T) *Model {
	t.Helper()
	m := New(Options{Files: []string{"page.png"}, Render: "kitty", Page: 1})
	t.Cleanup(func() { m.Close(); picture.ForceKittyCapability(picture.KittyCapabilityUnknown) })
	m.Update(tea.WindowSizeMsg{Width: 40, Height: 12})
	m.source = image.NewRGBA(image.Rect(0, 0, 100, 100))
	draw := m.refreshImage()
	if glyphs(m.pictureView()) {
		t.Fatal("initial image flashed glyphs")
	}
	drainAnimation(t, m, draw)
	if m.presentation.Front == nil || glyphs(m.pictureView()) {
		t.Fatal("initial image was not presented")
	}
	return m
}

func TestStillKittyRetainsFrontAcrossReplacements(t *testing.T) {
	m := stillKitty(t)
	front := m.presentation.Front
	draw := m.refreshImage()
	obsolete := m.presentation.Flight
	m.zoom = 1
	zoom := m.refreshImage()
	if obsolete.Active.Load() || m.pictureView() != front.View {
		t.Fatal("replacement lost the front or retained obsolete work")
	}
	drainAnimation(t, m, draw)
	if m.presentation.Front != front {
		t.Fatal("obsolete replacement was presented")
	}
	drainAnimation(t, m, zoom)
	if m.presentation.Front == front || front.Flight.Active.Load() || glyphs(m.pictureView()) {
		t.Fatal("replacement failed to retire and swap the front")
	}
}

func TestStillKittyPageLoadRetainsFront(t *testing.T) {
	m := stillKitty(t)
	front := m.presentation.Front
	m.loading = true
	body, _ := m.body(m.width, m.bodyHeight())
	if glyphs(body) || body == "" {
		t.Fatal("loading did not retain the image")
	}
	cmd := m.loaded(document.Result{Generation: m.generation, Kind: "pdf", Image: m.source, Page: 2, Pages: 3})
	if m.presentation.Front != front || !front.Flight.Active.Load() {
		t.Fatal("page load retired the image before replacement")
	}
	drainAnimation(t, m, cmd)
	if m.presentation.Front == front || front.Flight.Active.Load() {
		t.Fatal("page replacement did not settle")
	}
}

func TestStillKittyPendingCleanup(t *testing.T) {
	for _, action := range []string{"clear", "glyph", "resize"} {
		t.Run(action, func(t *testing.T) {
			m := stillKitty(t)
			draw := m.refreshImage()
			pending := m.presentation.Flight
			var cmd tea.Cmd
			switch action {
			case "clear":
				cmd = m.clearPicture()
			case "glyph":
				_, cmd = m.Update(tea.KeyPressMsg{Code: 'g', Text: "g"})
			case "resize":
				_, cmd = m.Update(tea.WindowSizeMsg{Width: 20, Height: 8})
			}
			drainAnimation(t, m, cmd)
			front := m.presentation.Front
			drainAnimation(t, m, draw)
			if pending.Active.Load() || m.presentation.Front != front {
				t.Fatal("late replacement restored a retired placement")
			}
			if action == "glyph" && !glyphs(m.pictureView()) {
				t.Fatal("explicit glyph fallback did not render")
			}
			if action == "resize" && (front == nil || glyphs(m.pictureView())) {
				t.Fatal("resize did not present a Kitty image")
			}
		})
	}
}
