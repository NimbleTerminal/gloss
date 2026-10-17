package app

import (
	"math/rand/v2"

	ntpicture "nimbleterminal.dev/ntpicture"
)

func newTerminalSession(render string) *ntpicture.TerminalSession {
	capability := ntpicture.KittyCapabilityUnknown
	switch render {
	case "kitty":
		capability = ntpicture.KittyCapabilitySupported
	case "glyph":
		capability = ntpicture.KittyCapabilityUnsupported
	}
	return ntpicture.NewTerminalSession(ntpicture.TerminalOptions{Kitty: capability})
}

func (m *Model) cellPixelSize() (int, int) {
	s := m.terminal.Snapshot()
	return s.CellPixelWidth, s.CellPixelHeight
}

// toggleGraphics records an explicit choice without changing discovered capability.
func (m *Model) toggleGraphics() {
	m.autoKitty = false
	if m.kitty {
		m.kitty = false
	} else if m.terminal.Snapshot().Kitty == ntpicture.KittyCapabilitySupported {
		m.kitty = true
	}
}

// resetKittyIDs avoids image IDs left in scrollback by earlier Gloss processes.
func resetKittyIDs() { nextModelID.Store(rand.Int64N(8000)) }
