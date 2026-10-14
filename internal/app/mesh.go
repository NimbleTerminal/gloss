package app

import (
	tea "charm.land/bubbletea/v2"
	"nimbleterminal.dev/gloss/internal/document"
	ntpicture "nimbleterminal.dev/ntpicture"
)

func (m *Model) meshHeight() int { return max(1, m.bodyHeight()-2) }
func (m *Model) meshKitty() bool {
	return m.chart != nil && m.chart.PictureMode() == ntpicture.PictureKitty
}

func (m *Model) setMesh(mesh *document.Mesh) tea.Cmd {
	if err := mesh.SetScene(m.chart.Scene()); err != nil {
		m.err = err
		return nil
	}
	return m.chart.Refresh()
}

// The widget owns only the picture; Gloss adds the projection row above it.
// Its mouse zones are local to that picture, including in menu previews.
func meshMessage(msg tea.Msg) tea.Msg {
	mouse, ok := msg.(tea.MouseMsg)
	if !ok {
		return msg
	}
	at := mouse.Mouse()
	at.Y--
	switch msg.(type) {
	case tea.MouseClickMsg:
		return tea.MouseClickMsg(at)
	case tea.MouseReleaseMsg:
		return tea.MouseReleaseMsg(at)
	case tea.MouseMotionMsg:
		return tea.MouseMotionMsg(at)
	case tea.MouseWheelMsg:
		return tea.MouseWheelMsg(at)
	}
	return msg
}
