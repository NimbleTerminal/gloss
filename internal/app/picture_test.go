package app

import (
	"image"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/NimbleMarkets/ntcharts/v2/picture"
	ntpicture "nimbleterminal.dev/ntpicture"
)

// Each picture shown with Kitty graphics goes under an id of its own: the
// browser app's terminal would otherwise keep showing the last picture of the
// same size. Glyphs have no ids to change.
func TestEachKittyPictureHasItsOwnID(t *testing.T) {
	t.Cleanup(func() { picture.ForceKittyCapability(picture.KittyCapabilityUnknown) })
	img := image.NewRGBA(image.Rect(0, 0, 64, 48))
	glyph := New(Options{Files: []string{"a.png"}, Render: "glyph", Page: 1, DPI: 72})
	defer glyph.Close()
	first := glyph.picID
	if glyph.showPicture(img); glyph.picID != first {
		t.Fatal("a glyph picture changed its id")
	}
	m := New(Options{Files: []string{"a.png"}, Render: "kitty", Page: 1, DPI: 72})
	defer m.Close()
	m.Update(tea.WindowSizeMsg{Width: 40, Height: 12})
	first = m.picID
	drainAnimation(t, m, m.showPicture(img))
	second := m.picID
	m.source, m.zoom = img, 1
	drainAnimation(t, m, m.refreshImage())
	if second == first || m.picID == second || m.pic.Mode() != picture.PictureKitty {
		t.Fatalf("ids %d, %d, %d; mode %v", first, second, m.picID, m.pic.Mode())
	}
	if w, h := m.pic.CellPixelSize(); w <= 0 || h <= 0 {
		t.Fatalf("cell size lost: %d×%d", w, h)
	}
}

// Every Kitty picture sends zlib-compressed raw pixels: the page, the Markdown
// view's inline pictures and a typeset overlay alike.
func TestKittyPicturesSendZlibFrames(t *testing.T) {
	m := typesetGIFModel(t, "kitty", 80, 30)
	if m.pic.KittyFormat() != picture.KittyFormatZlib {
		t.Fatalf("page format %v", m.pic.KittyFormat())
	}
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
