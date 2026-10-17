package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/ansi/kitty"
)

func TestGIFScrubberKeysAndResume(t *testing.T) {
	for _, render := range []string{"glyph", "kitty"} {
		t.Run(render, func(t *testing.T) {
			m := animated(t, render, -1)
			a := m.animation
			a.setSpeed(1)
			for _, tt := range []struct {
				key   string
				frame int
			}{{".", 2}, {".", 2}, {",", 1}, {",", 1}, {"end", 2}, {"home", 1}} {
				_, cmd := m.Update(press(tt.key))
				drainAnimation(t, m, cmd)
				if !a.Status().Paused || a.Status().Finished || a.player().Frame() != tt.frame || a.rate() != 2 {
					t.Fatalf("%s: wrong seek state", tt.key)
				}
				if m.source != a.player().Image() {
					t.Fatal("export source did not follow seek")
				}
				if render == "kitty" && (!strings.ContainsRune(m.pictureView(), kitty.Placeholder) || a.Status().Picture.Busy) {
					t.Fatal("seek not presented in Kitty")
				}
			}
			cmd := m.toggleAnimation()
			if a.Status().Paused || cmd == nil {
				t.Fatal("Space did not resume sought frame")
			}
			a.Pause(true)
			drainAnimation(t, m, cmd)
		})
	}
}

func TestGIFScrubberCoalescesAndRejectsLateResults(t *testing.T) {
	m := animated(t, "glyph", -1)
	first := m.seekAnimation(2)
	latest := m.seekAnimation(1)
	_, resize := m.Update(tea.WindowSizeMsg{Width: 60, Height: 15})
	drainAnimation(t, m, tea.Batch(resize, latest, first))
	if m.animation.player().Frame() != 1 || m.animation.target != 0 {
		t.Fatal("latest seek did not win")
	}
	late := m.seekAnimation(2)
	m.stopAnimation()
	before := m.source
	drainAnimation(t, m, late)
	if m.source != before || m.animation != nil {
		t.Fatal("late seek survived leaving file")
	}
}

func TestGIFScrubberMouseAndLayout(t *testing.T) {
	for _, top := range []bool{false, true} {
		m := animated(t, "glyph", -1)
		m.opts.Prompt = "Choose a frame"
		m.opts.PromptTop = top
		start, cells, _ := m.scrubberGeometry()
		_, cmd := m.Update(tea.MouseClickMsg{X: start + cells - 1, Y: m.height - 1, Button: tea.MouseLeft})
		drainAnimation(t, m, cmd)
		if m.animation.player().Frame() != 2 || !m.animation.Status().Paused {
			t.Fatal("click did not seek")
		}
		_, cmd = m.Update(tea.MouseMotionMsg{X: -20, Y: 0, Button: tea.MouseLeft})
		drainAnimation(t, m, cmd)
		if m.animation.player().Frame() != 1 {
			t.Fatal("drag did not clamp to first frame")
		}
		_, cmd = m.Update(tea.MouseReleaseMsg{X: 1000, Y: 0, Button: tea.MouseLeft})
		drainAnimation(t, m, cmd)
		if m.animation.player().Frame() != 2 || m.animation.dragging {
			t.Fatal("release did not settle drag")
		}
		for _, width := range []int{8, 25, 50, 100} {
			m.Update(tea.WindowSizeMsg{Width: width, Height: 15})
			if ansi.StringWidth(m.scrubberView()) > width {
				t.Fatal("scrubber overflows viewport")
			}
			view := m.View()
			if view.MouseMode != tea.MouseModeCellMotion || !strings.Contains(view.Content, "●") {
				t.Fatal("scrubber not interactive")
			}
		}
		m.help = true
		if m.scrubberVisible() {
			t.Fatal("scrubber covers help")
		}
		if _, took := m.scrubberMouse(tea.MouseClickMsg{X: start, Y: m.height - 1, Button: tea.MouseLeft}); took {
			t.Fatal("help click sought")
		}
	}
}
