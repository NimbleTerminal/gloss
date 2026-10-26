package app

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"nimbleterminal.dev/gloss/internal/i18n"
)

func (m *Model) seekAnimation(frame int) tea.Cmd {
	a := m.animation
	frame = max(1, min(frame, a.player().Frames()))
	a.target = frame
	return a.Seek(frame)
}

func (a *animation) position() int {
	if a.target != 0 {
		return a.target
	}
	return a.player().Frame()
}

func (m *Model) scrubberVisible() bool {
	return m.animationVisible() && m.layer == layerNone && m.height >= 3 && m.width >= 8
}

// Use the existing hint row, keeping picture placement and prompt layout stable.
func (m *Model) scrubberGeometry() (start, cells int, suffix string) {
	suffix = " " + i18n.T("app.keys.scrubber", ",/. step · Space play · ? help")
	if m.width < 50 {
		suffix = ""
	}
	return 2, max(2, m.width-4-ansi.StringWidth(suffix)), suffix
}

func (m *Model) scrubberView() string {
	start, cells, suffix := m.scrubberGeometry()
	a := m.animation
	at := (a.position() - 1) * (cells - 1) / max(1, a.player().Frames()-1)
	return strings.Repeat(" ", start-1) + "[" + strings.Repeat("─", at) + "●" + strings.Repeat("─", cells-at-1) + "]" + suffix
}

// Called with screen coordinates, before prompt-relative mouse translation.
func (m *Model) scrubberMouse(msg tea.MouseMsg) (tea.Cmd, bool) {
	a := m.animation
	if !m.scrubberVisible() {
		if a != nil {
			a.dragging = false
		}
		return nil, false
	}
	start, cells, _ := m.scrubberGeometry()
	mouse := msg.Mouse()
	switch msg.(type) {
	case tea.MouseClickMsg:
		if mouse.Button != tea.MouseLeft || mouse.Y != m.height-1 || mouse.X < start || mouse.X >= start+cells {
			return nil, false
		}
		a.dragging = true
	case tea.MouseMotionMsg:
		if !a.dragging {
			return nil, false
		}
	case tea.MouseReleaseMsg:
		if !a.dragging {
			return nil, false
		}
		a.dragging = false
	default:
		return nil, false
	}
	x := max(0, min(cells-1, mouse.X-start))
	frame := 1 + (x*(a.player().Frames()-1)+(cells-1)/2)/(cells-1)
	return m.seekAnimation(frame), true
}
