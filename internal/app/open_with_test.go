//go:build !js

package app

import (
	"fmt"
	"github.com/charmbracelet/x/ansi"
	"nimbleterminal.dev/gloss/internal/openwith"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeOpenHost struct {
	launched []openwith.Target
	ids      []string
	saved    string
}

func (h *fakeOpenHost) Choices(t openwith.Target) ([]openwith.Choice, error) {
	return []openwith.Choice{{ID: "system", Name: "System default", Available: true, Default: true}, {ID: "editor", Name: "Editor", Available: true}, {ID: "reveal", Name: "Reveal", Available: true}}, nil
}
func (h *fakeOpenHost) Launch(t openwith.Target, id string) (string, error) {
	h.launched = append(h.launched, t)
	h.ids = append(h.ids, id)
	return "Editor", nil
}
func (h *fakeOpenHost) SetDefault(kind, id string) error { h.saved = kind + ":" + id; return nil }
func TestOpenWithPickerAndLaunch(t *testing.T) {
	m := themedModel(t, "dark", false)
	h := &fakeOpenHost{}
	file := filepath.Join(t.TempDir(), "note.md")
	if err := os.WriteFile(file, []byte("hello"), 0600); err != nil {
		t.Fatal(err)
	}
	m.opts.Files = []string{file}
	m.opts.OpenWith = h
	m.kind = "markdown"
	_, cmd := m.Update(press("W"))
	drainAnimation(t, m, cmd)
	if !strings.Contains(m.View().Content, "Open with") {
		t.Fatal("chooser not shown")
	}
	_, cmd = m.Update(press("down"))
	drainAnimation(t, m, cmd)
	_, cmd = m.Update(press("d"))
	drainAnimation(t, m, cmd)
	if h.saved != "markdown:editor" || len(h.launched) != 0 {
		t.Fatalf("save %q launches %v", h.saved, h.launched)
	}
	_, cmd = m.Update(press("enter"))
	drainAnimation(t, m, cmd)
	if len(h.launched) != 1 || h.launched[0].Path != file || h.ids[0] != "editor" {
		t.Fatalf("launch %+v %v", h.launched, h.ids)
	}
	_, cmd = m.Update(press("w"))
	drainAnimation(t, m, cmd)
	if len(h.launched) != 2 || h.ids[1] != "" {
		t.Fatal("default launch missing")
	}
}
func TestOpenWithUnavailable(t *testing.T) {
	m := themedModel(t, "dark", false)
	_, cmd := m.Update(press("W"))
	drainAnimation(t, m, cmd)
	if !strings.Contains(m.note, "not available") {
		t.Fatal(m.note)
	}
}

func TestOpenWithSelectionAndCancellation(t *testing.T) {
	for _, screen := range []screen{screenList, screenGrid} {
		m := themedModel(t, "dark", false)
		h := &fakeOpenHost{}
		dir := t.TempDir()
		a, b := filepath.Join(dir, "a.txt"), filepath.Join(dir, "b.txt")
		for _, p := range []string{a, b} {
			if err := os.WriteFile(p, []byte("text"), 0600); err != nil {
				t.Fatal(err)
			}
		}
		m.opts.Files = []string{a, b}
		m.opts.OpenWith = h
		m.screen = screen
		m.index = 0
		m.selection = 1
		_, cmd := m.Update(press("W"))
		drainAnimation(t, m, cmd)
		if m.openWithPicker == nil || m.openWithPicker.target.Path != b {
			t.Fatal("wrong selected target")
		}
		_, cmd = m.Update(press("esc"))
		drainAnimation(t, m, cmd)
		if m.openWithPicker != nil || len(h.launched) != 0 || m.selection != 1 {
			t.Fatal("cancel changed selection or launched")
		}
	}
}
func TestOpenWithStaleLoadClosesPicker(t *testing.T) {
	m := themedModel(t, "dark", false)
	m.opts.OpenWith = &fakeOpenHost{}
	m.openWithPicker = &openWithPicker{operation: 1, generation: m.generation}
	old := m.generation
	m.generation++
	_, cmd := m.Update(openWithLoaded{operation: 1, generation: old})
	drainAnimation(t, m, cmd)
	if m.openWithPicker != nil {
		t.Fatal("stale picker remained stuck")
	}
}

func TestOpenWithIgnoresMouseAndDoubleSubmit(t *testing.T) {
	m := themedModel(t, "dark", false)
	h := &fakeOpenHost{}
	m.opts.OpenWith = h
	m.openWithPicker = &openWithPicker{operation: 1, generation: m.generation, target: openwith.Target{Path: "/tmp/test", Kind: "text"}, choices: []openwith.Choice{{ID: "system", Available: true}}}
	m.layer = layerOpenWith
	_, first := m.Update(press("enter"))
	_, second := m.Update(press("enter"))
	drainAnimation(t, m, second)
	if len(h.launched) != 0 {
		t.Fatal("double Enter launched before first command")
	}
	drainAnimation(t, m, first)
	if len(h.launched) != 1 {
		t.Fatal("expected single launch", h.launched)
	}
}
func TestOpenWithPausesAnimationAndRestoresView(t *testing.T) {
	m := animated(t, "glyph", 0)
	m.animation.Paused = true
	h := &fakeOpenHost{}
	m.opts.OpenWith = h
	m.openWithPicker = &openWithPicker{operation: 1, generation: m.generation, busy: true}
	m.layer = layerOpenWith
	if m.mediaVisible() {
		t.Fatal("animation active under chooser")
	}
	if !strings.Contains(m.View().Content, "Open with") {
		t.Fatal("chooser missing")
	}
	_, cmd := m.Update(press("esc"))
	drainAnimation(t, m, cmd)
	if !m.mediaVisible() || m.openWithPicker != nil {
		t.Fatal("view not restored")
	}
}

func TestOpenWithSmallScreenDoesNotAct(t *testing.T) {
	m := themedModel(t, "dark", false)
	h := &fakeOpenHost{}
	m.opts.OpenWith = h
	m.width = 20
	m.height = 5
	m.openWithPicker = &openWithPicker{operation: 1, generation: m.generation, target: openwith.Target{Path: "/tmp/test", Kind: "text"}, choices: []openwith.Choice{{ID: "system", Available: true}}}
	m.layer = layerOpenWith
	for _, key := range []string{"enter", "d"} {
		_, cmd := m.Update(press(key))
		drainAnimation(t, m, cmd)
	}
	if len(h.launched) > 0 || h.saved != "" {
		t.Fatal("acted on hidden choice")
	}
}
func TestOpenWithFooterFits(t *testing.T) {
	m := themedModel(t, "dark", false)
	m.openWithPicker = &openWithPicker{busy: true}
	m.layer = layerOpenWith
	for _, w := range []int{24, 30, 50, 80} {
		m.width = w
		m.height = 12
		view := m.View().Content
		for _, line := range strings.Split(view, "\n") {
			if ansi.StringWidth(line) > w {
				t.Fatalf("row exceeds %d: %q", w, line)
			}
		}
		if len(strings.Split(view, "\n")) > m.height {
			t.Fatalf("too many rows at %d", w)
		}
	}
}

func TestOpenWithBrowserOwnsKeys(t *testing.T) {
	m := browsing(t, folder(t))
	h := &fakeOpenHost{}
	m.opts.OpenWith = h
	pump(m, m.opener.picker.SetFilter("alpha.png"), 0)
	before := m.opener.picker.FilterValue()
	send(m, press("alt+enter"))
	if m.openWithPicker == nil || m.openWithPicker.target.Kind != "image/*" {
		t.Fatal("browser intent not adapted")
	}
	send(m, press("w"))
	if m.opener.picker.FilterValue() != before {
		t.Fatal("chooser key reached filter")
	}
	send(m, press("esc"))
	if !m.browsing() || m.opener.picker.FilterValue() != before {
		t.Fatal("cancel changed browser state")
	}
}
func TestOpenWithCanceledReplyAndSaveFailure(t *testing.T) {
	m := themedModel(t, "dark", false)
	m.opts.OpenWith = &fakeOpenHost{}
	m.openWithPicker = &openWithPicker{operation: 1, generation: m.generation}
	m.layer = layerOpenWith
	send(m, press("esc"))
	send(m, openWithLoaded{operation: 1, generation: m.generation, choices: []openwith.Choice{{ID: "system"}}})
	if m.openWithPicker != nil {
		t.Fatal("late reply reopened chooser")
	}
	m.openWithPicker = &openWithPicker{operation: 2, generation: m.generation, busy: true}
	m.layer = layerOpenWith
	send(m, openWithFinished{operation: 2, generation: m.generation, saved: true, err: fmt.Errorf("read-only config")})
	if m.openWithPicker == nil || m.openWithPicker.busy || !strings.Contains(m.note, "read-only config") {
		t.Fatal("save failure not reported")
	}
}
func TestOpenWithPreservesOriginalKindAndTemporaryProvenance(t *testing.T) {
	m := themedModel(t, "dark", false)
	h := &fakeOpenHost{}
	m.opts.OpenWith = h
	file := filepath.Join(t.TempDir(), "gloss-fetch-ordinary.html")
	if err := os.WriteFile(file, []byte("<p>Hello</p>"), 0600); err != nil {
		t.Fatal(err)
	}
	m.opts.Files = []string{file}
	m.kind = "html"
	send(m, press("w"))
	if h.launched[0].Kind != "html" || h.launched[0].Temporary {
		t.Fatal("lost original kind or inferred temp by prefix")
	}
	m.opts.IsTemporary = func(p string) bool { return p == file }
	send(m, press("w"))
	if !h.launched[1].Temporary || !strings.Contains(m.note, "temporary file") {
		t.Fatal("lost temporary provenance")
	}
}
