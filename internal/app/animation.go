package app

import (
	"image"
	"math"

	tea "charm.land/bubbletea/v2"
	"github.com/NimbleMarkets/ntcharts/v2/picture"
	"nimbleterminal.dev/gloss/internal/animate"
	ntpicture "nimbleterminal.dev/ntpicture"
	ntanimation "nimbleterminal.dev/ntpicture/animation"
	"nimbleterminal.dev/ntpicture/playback"
)

// Standalone controls stay in Gloss; Controller owns playback and presentation.
type animation struct {
	*playback.Controller
	speed    int
	target   int
	dragging bool
}

type animationPresented = animate.Presented

func (a *animation) rate() float64 { return a.Status().Rate }
func (a *animation) player() *ntanimation.Player {
	s := a.Status()
	if s.PresentedPlayer != nil {
		return s.PresentedPlayer
	}
	return s.Player
}
func (a *animation) setSpeed(n int) tea.Cmd {
	a.speed = max(-2, min(2, n))
	return a.SetRate(math.Exp2(float64(a.speed)))
}
func (m *Model) retireAnimationPicture(cmd tea.Cmd) { m.presentation.Retire(cmd) }
func (m *Model) clearAnimationFront() tea.Cmd       { return m.presentation.ClearFront() }
func (m *Model) pictureView() string {
	if m.playback != nil {
		return m.playback.View()
	}
	if m.typesetLayers != nil {
		return m.typesetLayers.View()
	}
	if m.still != nil {
		return m.still.view()
	}
	if m.quitting {
		return m.pic.View().Content
	}
	m.presentation.SetSize(m.width, m.bodyHeight())
	return m.presentation.View()
}
func (m *Model) resizeAnimation() tea.Cmd { return m.refreshImage() }
func (m *Model) keepAnimationPicture() tea.Cmd {
	if m.playback != nil {
		if m.keepOnMainScreen {
			return m.playback.KeepOnMainScreen()
		}
		return m.playback.Keep()
	}
	if m.typesetLayers != nil {
		if m.keepOnMainScreen {
			return m.typesetLayers.KeepOnMainScreen()
		}
		return m.typesetLayers.Keep()
	}
	if m.still != nil {
		if m.keepOnMainScreen {
			return m.still.pic.KeepOnMainScreen()
		}
		return m.still.pic.Keep()
	}
	if f := m.presentation.Front; f != nil {
		m.source = f.Image
	}
	cmd := m.presentation.Keep()
	m.picID = m.presentation.ID
	return cmd
}

// Stop controls and composition while retaining the displayed frame during load.
func (m *Model) stopAnimation() tea.Cmd {
	var cmd tea.Cmd
	if m.playback != nil {
		cmd = m.playback.Stop()
	}
	m.presentation.Stop()
	m.animation = nil
	return cmd
}
func (m *Model) clearStandalone() tea.Cmd {
	if m.playback == nil {
		return nil
	}
	c := m.playback
	m.playback, m.animation = nil, nil
	return c.Close()
}
func (m *Model) startAnimation(player *ntanimation.Player) tea.Cmd {
	cleanup := tea.Batch(m.clearStandalone(), m.clearStill(), m.presentation.Clear())
	c := playback.New(nil, playback.Options{Picture: ntpicture.Options{
		Session: m.terminal, HostGeometry: true,
		Render: ntpicture.Config{KittyZ: -1, Background: m.pictureBackground(), KittyFormat: ntpicture.KittyFormatZlib},
		NextID: func() (int, error) { id := nextKittyID(); m.picID = id; return id, nil },
	}})
	m.playback = c
	m.animation = &animation{Controller: c}
	return tea.Batch(cleanup, m.syncStandalone(), c.SetPlayer(player))
}
func (m *Model) syncStandalone() tea.Cmd {
	c := m.playback
	if c == nil {
		return nil
	}
	cw, ch := m.cellPixelSize()
	var rect image.Rectangle
	if m.zoom > 0 && m.source != nil {
		rect = cropRect(m.source.Bounds(), m.zoom, m.panX, m.panY)
	}
	return tea.Batch(c.SetTerminal(m.pic.Mode() == picture.PictureKitty, cw, ch),
		c.SetSize(max(1, m.width), max(1, m.bodyHeight())),
		c.SetCrop(rect), c.SetBackground(m.pictureBackground()))
}
func (m *Model) updateStandalone(msg tea.Msg) tea.Cmd {
	c := m.playback
	if c == nil {
		return nil
	}
	if m.quitting || m.loading {
		return c.Update(msg)
	}
	// Hidden screens release placements and preserve playback position.
	visibility := c.SetVisible(m.mediaVisible())
	cmd := tea.Batch(visibility, m.syncStandalone(), c.Update(msg))
	state := c.Status()
	if m.animation != nil {
		if state.PresentedPlayer != nil {
			m.source = state.PresentedPlayer.Image()
		}
		if state.Err != nil {
			m.note = "animation: " + safe(state.Err.Error())
		}
		if state.Err != nil || !state.Busy && state.Player != nil && state.Player.Frame() == m.animation.target {
			m.animation.target = 0
		}
	}
	return cmd
}
func (m *Model) animationVisible() bool { return m.animation != nil && m.mediaVisible() }
func (m *Model) mediaVisible() bool {
	return m.openWithPicker == nil && !m.isPreview && !m.loading && !m.suspended && !m.quitting && !m.help && m.screen == screenDocument && m.markdown == nil && (m.typeset == nil || (m.typesetActive == 0 && m.typesetMatches()))
}
func (m *Model) toggleAnimation() tea.Cmd {
	a := m.animation
	if a == nil {
		return nil
	}
	state := a.Status()
	a.target = 0
	return a.Pause(!state.Paused && !state.Finished)
}
func (m *Model) animationTransmission(msg tea.Msg, cmd tea.Cmd) tea.Cmd {
	return m.presentation.Transmission(msg, cmd)
}
func (m *Model) clearPicture() tea.Cmd {
	var layers tea.Cmd
	if m.typesetLayers != nil {
		layers = m.typesetLayers.SetVisible(false)
	}
	return tea.Batch(layers, m.clearStill(), m.presentation.Clear())
}
