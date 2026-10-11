package app

import (
	tea "charm.land/bubbletea/v2"
	"fmt"
	"github.com/charmbracelet/x/ansi"
	"strings"
	"testing"
)

func TestThemePickerPreviewCancelAndSave(t *testing.T) {
	m := themedModel(t, "auto", false)
	saved := ""
	m.opts.SaveTheme = func(ref string) error { saved = ref; return nil }
	press := func(key string) {
		t.Helper()
		cmd, _ := m.key(tea.KeyPressMsg{Code: []rune(key)[0], Text: key})
		drainAnimation(t, m, cmd)
	}
	press("T")
	if m.layer != layerTheme || m.themePicker == nil {
		t.Fatal("picker not opened")
	}
	m.themePicker.cursor = 0
	drainAnimation(t, m, m.themePickerKey("down"))
	preview := m.theme
	if preview == "dark" || saved != "" {
		t.Fatal("no live preview or premature save", preview, saved)
	}
	if !strings.Contains(ansi.Strip(m.View().Content), "Themes") {
		t.Fatal("missing overlay")
	}
	drainAnimation(t, m, m.themePickerKey("esc"))
	if m.theme != "dark" || m.opts.Theme != "auto" || m.layer != layerNone || saved != "" {
		t.Fatal("cancel failed")
	}
	press("T")
	m.themePicker.cursor = 0
	drainAnimation(t, m, m.themePickerKey("down"))
	drainAnimation(t, m, m.themePickerKey("enter"))
	if saved != preview || m.opts.Theme != preview || m.themePicker != nil {
		t.Fatal("confirm failed", saved)
	}
	// Explicit selection must ignore terminal background updates.
	m.opts.SaveTheme = func(string) error { return fmt.Errorf("read-only preferences") }
	press("T")
	drainAnimation(t, m, m.themePickerKey("enter"))
	if !strings.Contains(m.note, "read-only preferences") {
		t.Fatal("save failure hidden")
	}
}
func TestThemePickerNarrowAndHostBoundary(t *testing.T) {
	m := themedModel(t, "nord", true)
	m.openThemePicker()
	for _, size := range [][2]int{{10, 4}, {24, 8}, {40, 15}} {
		s := m.themePickerView(size[0], size[1])
		if len(strings.Split(s, "\n")) > size[1] || ansi.StringWidth(s) > size[0]*size[1] {
			t.Fatalf("oversized theme overlay %v: %d rows, %d width: %q", size, len(strings.Split(s, "\n")), ansi.StringWidth(s), ansi.Strip(s))
		}
	}
	drainAnimation(t, m, m.themePickerKey("enter")) // No persistence callback in web hosts.
	if m.opts.Theme != "nord" || m.theme != "nord" {
		t.Fatal("host theme lost")
	}
}
