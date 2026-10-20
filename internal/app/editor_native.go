//go:build !js

package app

import (
	"path/filepath"

	tea "charm.land/bubbletea/v2"
)

type editorFinished struct {
	picker    *openWithPicker
	top, left int
	err       error
}

func (m *Model) terminalEditorMessage(msg tea.Msg) (tea.Cmd, bool) {
	switch v := msg.(type) {
	case editorReady:
		if m.openWithPicker != v.picker {
			return nil, true
		}
		if m.generation != v.picker.generation {
			return m.closeOpenWith(), true
		}
		m.generation++ // Reject document loads already in flight before the handoff.
		v.picker.generation = m.generation
		top, left := m.typesetTop, m.typesetLeft
		if m.markdown != nil {
			m.savedMarkdown = m.markdown
		}
		if m.sheet != nil {
			m.savedSheet = m.sheet
		}
		// Remove Kitty placements and stop inline animations before releasing the TTY.
		// Keep the picker busy until the callback so no new animation is scheduled.
		cleanup := tea.Batch(m.clearGraphics(), m.disposePreview())
		return tea.Sequence(cleanup, tea.ExecProcess(v.command, func(err error) tea.Msg {
			return editorFinished{picker: v.picker, top: top, left: left, err: err}
		})), true
	case editorFinished:
		if m.openWithPicker != v.picker {
			return nil, true
		}
		if m.generation != v.picker.generation {
			return m.closeOpenWith(), true
		}
		m.closeOpenWith()
		m.typesetTop, m.typesetLeft = v.top, v.left
		m.note = "Editor closed"
		if v.err != nil {
			m.note = "Editor: " + safe(v.err.Error())
		}
		if v.picker.target.Temporary {
			m.note += " — temporary file; save a copy to keep it"
		}
		if m.browsing() {
			return m.reread(), true
		}
		if m.listing() {
			// Previews are recreated, and the grid's loader must discard its cached input.
			if m.grid != nil {
				for _, path := range m.opts.Files {
					absolute, _ := filepath.Abs(path)
					if absolute == v.picker.target.Path {
						delete(m.grid.tiles, path)
					}
				}
				m.grid.generation++
				m.grid.loading = ""
			}
			if m.screen == screenGrid {
				return m.layoutGrid(), true
			}
			return m.updatePreview(), true
		}
		if m.opts.typesetEnabled() {
			m.page = 1
		} // Restore the viewport, not a stale page number.
		return m.load(true), true
	}
	return nil, false
}
