//go:build js

package app

import (
	tea "charm.land/bubbletea/v2"
	"fmt"
	"nimbleterminal.dev/gloss/internal/openwith"
)

func openWithTarget(string, string, bool) (openwith.Target, error) {
	return openwith.Target{}, fmt.Errorf("not available in the browser")
}
func (m *Model) openWithBrowse(tea.Msg) (tea.Cmd, bool) { return nil, false }
