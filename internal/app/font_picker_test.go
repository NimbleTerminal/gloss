package app

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi/kitty"
	"nimbleterminal.dev/gloss/internal/document"
	ntpicture "nimbleterminal.dev/ntpicture"
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

	m := themedModel(t, "light", true, "kitty")
	_, cmd := m.Update(press("f"))
	// The preview and Kitty encoding are finite work, not timers. Wait for
	// both even when rendering exceeds pump's patience on a loaded CI runner.
	var pickerID string
	capture := func(msg tea.Msg) {
		if raw, ok := msg.(tea.RawMsg); ok {
			for _, header := range regexp.MustCompile(`\x1b_G([^;]+);`).FindAllStringSubmatch(fmt.Sprint(raw.Msg), -1) {
				if p := m.fontPicker; p != nil && strings.Contains(header[1], "a=T") &&
					strings.Contains(header[1]+",", fmt.Sprintf("c=%d,", p.width)) &&
					strings.Contains(header[1]+",", fmt.Sprintf("r=%d,", p.height)) {
					id := regexp.MustCompile(`(?:^|,)i=(\d+)`).FindStringSubmatch(header[1])
					if len(id) == 2 {
						pickerID = id[1]
					}
				}
			}
		}
	}
	drainAnimation(t, m, cmd, capture)
	if pickerID == "" {
		t.Fatal("missing chooser transmission")
	}
	if m.fontPicker == nil || m.fontPicker.pic.Mode() != ntpicture.PictureKitty {
		t.Fatal("picker ignored capability")
	}
	old := m.fontPicker
	if old.base == nil || old.err != nil || !strings.ContainsRune(old.pic.View(), kitty.Placeholder) {
		t.Fatalf("picker frame not ready: %v", old.err)
	}
	_, cmd = m.Update(tea.WindowSizeMsg{Width: 12, Height: 6})
	raw := collectRaw(cmd)
	if !strings.Contains(raw, "a=d,d=I,i="+pickerID) {
		t.Fatal("resize did not clean graphics", raw)
	}
	_, cmd = m.Update(press("esc"))
	drainAnimation(t, m, cmd)
	_, cmd = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	drainAnimation(t, m, cmd)
	_, cmd = m.Update(press("f"))
	pickerID = ""
	drainAnimation(t, m, cmd, capture)
	if pickerID == "" {
		t.Fatal("missing reopened chooser transmission")
	}
	if m.fontPicker.owner == old.owner {
		t.Fatal("reused picker image ID")
	}
	reopened := m.fontPicker
	if reopened.base == nil || reopened.err != nil || !strings.ContainsRune(reopened.pic.View(), kitty.Placeholder) {
		t.Fatalf("reopened picker frame not ready: %v", reopened.err)
	}
	raw = collectRaw(m.quit())
	if !strings.Contains(raw, "a=d,d=I,i="+pickerID) {
		t.Fatal("quit did not clean picker")
	}
}
