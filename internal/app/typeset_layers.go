package app

import (
	"image"

	tea "charm.land/bubbletea/v2"
	ntpicture "nimbleterminal.dev/ntpicture"
	ntanimation "nimbleterminal.dev/ntpicture/animation"
	"nimbleterminal.dev/ntpicture/playback"
)

// Gloss owns viewport layout and the scroll-settle policy. Layers owns the
// base picture, animated windows, glyph composition, and their lifetimes.
func (m *Model) showTypesetLayers(composite *ntanimation.Composite) tea.Cmd {
	var cleanup tea.Cmd
	if m.typesetLayers == nil {
		cleanup = tea.Batch(m.clearStandalone(), m.clearStill())
		m.typesetPaused = false
		m.typesetLayers = playback.NewLayers(nil, playback.Options{
			Picture: ntpicture.Options{
				Session:      m.terminal,
				HostGeometry: true,
				Render:       ntpicture.Config{KittyZ: -1, Background: m.pictureBackground(), KittyFormat: ntpicture.KittyFormatZlib},
				NextID:       func() (int, error) { return nextKittyID(), nil },
			},
		})
	}
	l := m.typesetLayers
	return tea.Batch(cleanup, m.pauseTypesetLayers(),
		l.SetActive(m.typesetPending == nil), m.syncTypesetLayers(), l.SetComposite(composite))
}

func (m *Model) syncTypesetLayers() tea.Cmd {
	if m.typesetLayers == nil {
		return nil
	}
	cw, ch := m.cellPixelSize()
	l := m.typesetLayers
	return tea.Batch(
		l.SetTerminal(m.kitty, cw, ch),
		l.SetVisible(m.markdown == nil),
		m.typesetLayers.SetSize(max(1, m.width), max(1, m.bodyHeight())),
		m.typesetLayers.SetBackground(m.pictureBackground()),
	)
}

func (m *Model) updateTypesetLayers(msg tea.Msg) tea.Cmd {
	if m.typesetLayers == nil {
		return nil
	}
	if m.quitting {
		return m.typesetLayers.Update(msg)
	}
	return tea.Batch(m.syncTypesetLayers(),
		m.pauseTypesetLayers(), m.typesetLayers.Update(msg))
}

func (m *Model) clearTypesetLayers() tea.Cmd {
	if m.typesetLayers == nil {
		return nil
	}
	l := m.typesetLayers
	m.typesetLayers = nil
	return l.Close()
}

// Snapshot exports the committed base and overlay frames, including a retained
// viewport while its replacement is still being prepared.
func (m *Model) typesetExportFrame() image.Image {
	if m.typesetLayers != nil {
		return m.typesetLayers.Snapshot()
	}
	if m.still != nil && m.still.front != nil {
		return m.still.front
	}
	return m.source
}

// Pause is a playback control that cancels pending work even when repeated.
// Apply visibility policy only on transitions, so ordinary messages cannot
// cancel the very animation result they are delivering.
func (m *Model) pauseTypesetLayers() tea.Cmd {
	paused := !m.mediaVisible()
	if m.typesetPaused == paused {
		return nil
	}
	// Pause(false) restarts finished playback. A visibility change must not
	// restart a clip whose loop count has already been exhausted.
	if paused && m.typesetLayers.Status().Finished {
		return nil
	}
	m.typesetPaused = paused
	return m.typesetLayers.Pause(paused)
}
