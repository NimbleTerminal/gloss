package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/NimbleMarkets/ntcharts/v2/picture"
	"github.com/NimbleTerminal/gloss/internal/document"
)

func TestFontPickerPreviewCancelConfirm(t *testing.T) {
	m := themedModel(t, "light", true)
	saved := ""
	m.opts.SaveFont = func(name, mono string) error { saved = name + "/" + mono; return nil }
	_, cmd := m.Update(press("f"))
	drainAnimation(t, m, cmd)
	if m.fontPicker == nil || m.fontPicker.base == nil || m.layer != layerFont {
		t.Fatal("missing font samples")
	}
	id := m.fontPicker.owner
	_, cmd = m.Update(press("down"))
	drainAnimation(t, m, cmd)
	if m.opts.Font != "editorial" || m.typesetFont != document.FontKey("editorial", "") || saved != "" {
		t.Fatal("live preview failed")
	}
	_, cmd = m.Update(press("+"))
	drainAnimation(t, m, cmd)
	if m.zoom != 1 {
		t.Fatal("zoom captured")
	}
	_, cmd = m.Update(press("esc"))
	drainAnimation(t, m, cmd)
	if document.FontKey(m.opts.Font, m.opts.Mono) != "readable/" || m.fontPicker != nil || saved != "" {
		t.Fatal("cancel did not restore")
	}
	_, cmd = m.Update(fontPreviewResult{owner: id, revision: 1})
	drainAnimation(t, m, cmd)
	if m.fontPicker != nil {
		t.Fatal("stale sample restored chooser")
	}
	_, cmd = m.Update(press("f"))
	drainAnimation(t, m, cmd)
	_, cmd = m.Update(press("down"))
	drainAnimation(t, m, cmd)
	_, cmd = m.Update(press("enter"))
	drainAnimation(t, m, cmd)
	if saved != "editorial/" || m.fontPicker != nil {
		t.Fatal("font not saved", saved)
	}
}
func TestFontPickerKittyCleanupAndResize(t *testing.T) {
	t.Cleanup(func() { picture.ForceKittyCapability(picture.KittyCapabilityUnknown) })
	m := themedModel(t, "light", true)
	picture.ForceKittyCapability(picture.KittyCapabilitySupported)
	m.pic.Toggle()
	_, cmd := m.Update(press("f"))
	pump(m, cmd, 0)
	if m.fontPicker == nil || m.fontPicker.pic.Mode() != picture.PictureKitty {
		t.Fatal("picker ignored capability")
	}
	old := m.fontPicker
	_, cmd = m.Update(tea.WindowSizeMsg{Width: 12, Height: 6})
	raw := collectRaw(cmd)
	if !strings.Contains(raw, "a=d,d=I,i=") {
		t.Fatal("resize did not clean graphics", raw)
	}
	_, cmd = m.Update(press("esc"))
	drainAnimation(t, m, cmd)
	_, cmd = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	drainAnimation(t, m, cmd)
	_, cmd = m.Update(press("f"))
	pump(m, cmd, 0)
	if m.fontPicker.owner == old.owner {
		t.Fatal("reused picker image ID")
	}
	raw = collectRaw(m.quit())
	if !strings.Contains(raw, "a=d,d=I,i=") {
		t.Fatal("quit did not clean picker")
	}
}
