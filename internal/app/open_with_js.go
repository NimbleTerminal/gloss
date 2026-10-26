//go:build js

package app

import (
	tea "charm.land/bubbletea/v2"
	"nimbleterminal.dev/gloss/internal/openwith"
)

func openWithTarget(string, string, bool) (openwith.Target, error) {
	return openwith.Target{}, errNoBrowserOpenWith
}
func (m *Model) openWithBrowse(tea.Msg) (tea.Cmd, bool) { return nil, false }

func (m *Model) terminalEditorMessage(tea.Msg) (tea.Cmd, bool) { return nil, false }
