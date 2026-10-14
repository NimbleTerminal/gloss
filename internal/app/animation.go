package app

import (
	tea "charm.land/bubbletea/v2"
	"nimbleterminal.dev/gloss/internal/animate"
	"time"
)

// Only standalone controls belong to the application. Playback, scheduling and
// picture placement are shared with Markdown's per-image animators.
type animation struct {
	*animate.Animator
	seek     *animationSeek
	target   int
	dragging bool
}
type animationPlacement = animate.Placement
type animationTick = animate.Tick
type animationFrame = animate.Frame
type animationPresented = animate.Presented

func (a *animation) rate() float64                  { return a.Rate() }
func (a *animation) delay() time.Duration           { return a.Delay() }
func (a *animation) setSpeed(n int)                 { a.SetSpeed(n) }
func (a *animation) cancelPending()                 { a.CancelPending() }
func (m *Model) retireAnimationPicture(cmd tea.Cmd) { m.presentation.Retire(cmd) }
func (m *Model) clearAnimationFront() tea.Cmd       { return m.presentation.ClearFront() }
func (m *Model) pictureView() string {
	if m.still != nil {
		return m.still.view()
	}
	if m.quitting {
		return m.pic.View().Content
	}
	m.presentation.SetSize(m.width, m.bodyHeight())
	return m.presentation.View()
}
func (m *Model) resizeAnimation() tea.Cmd { m.presentation.CancelPending(); return m.refreshImage() }
func (m *Model) keepAnimationPicture() tea.Cmd {
	if m.still != nil {
		return m.still.pic.Keep()
	}
	if f := m.presentation.Front; f != nil {
		m.source = f.Image
	}
	cmd := m.presentation.Keep()
	m.picID = m.presentation.ID
	return cmd
}
func (m *Model) stopAnimation() {
	if a := m.animation; a != nil && a.seek != nil {
		a.seek.cancel()
	}
	m.presentation.Stop()
	m.animation = nil
}
func (m *Model) animationVisible() bool { return m.animation != nil && m.mediaVisible() }
func (m *Model) mediaVisible() bool {
	return m.openWithPicker == nil && !m.isPreview && !m.loading && !m.suspended && !m.quitting && !m.help && m.screen == screenDocument && m.markdown == nil && (m.typeset == nil || (m.typesetActive == 0 && m.typesetMatches()))
}
func (m *Model) scheduleAnimation() tea.Cmd {
	visible := m.mediaVisible()
	if m.animation != nil && m.animation.seek != nil {
		visible = false
	}
	return m.presentation.Schedule(visible)
}
func (m *Model) animationTick(t animationTick) tea.Cmd {
	m.presentation.SetVisible(m.mediaVisible())
	cmd, _ := m.presentation.Update(t)
	return cmd
}
func (m *Model) animationFrame(f animationFrame) tea.Cmd {
	cmd, _ := m.presentation.Update(f)
	if f.Owner == m.presentation && f.Epoch == m.presentation.Epoch && m.presentation.Source != nil {
		m.source = m.presentation.Source.Image()
		if s, ok := m.presentation.Source.(*animate.CompositeSource); ok {
			m.typesetPlayback = s
		}
	}
	m.picID = m.presentation.ID
	return cmd
}
func (m *Model) toggleAnimation() tea.Cmd { return m.presentation.Toggle() }
func (m *Model) animationTransmission(msg tea.Msg, cmd tea.Cmd) tea.Cmd {
	return m.presentation.Transmission(msg, cmd)
}
func (m *Model) clearPicture() tea.Cmd { return tea.Batch(m.clearStill(), m.presentation.Clear()) }
