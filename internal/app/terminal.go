package app

import (
	"math/rand/v2"

	tea "charm.land/bubbletea/v2"
	"github.com/NimbleMarkets/ntcharts/v2/picture"
	uv "github.com/charmbracelet/ultraviolet"
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

// Legacy Model.Toggle still gates Kitty mode on a package-level value. Feed
// that compatibility gate from this connection, without using legacy probes
// or reading their global capability as the source of truth.
func syncLegacyCapability(session *ntpicture.TerminalSession) {
	picture.ForceKittyCapability(picture.KittyCapability(session.Snapshot().Kitty))
}

// terminalPicture creates a legacy presentation model without starting its
// process-wide discovery. The connection's TerminalSession owns discovery.
func terminalPicture(render string, fresh bool, session *ntpicture.TerminalSession) (picture.Model, int) {
	syncLegacyCapability(session)
	if fresh {
		// Avoid image IDs left in scrollback by earlier Gloss processes.
		nextModelID.Store(rand.Int64N(8000))
	}
	id := nextKittyID()
	s := session.Snapshot()
	pic := picture.NewWithConfig(picture.Config{
		KittyID: id, KittyZ: -1, Background: pictureBackground, KittyFormat: picture.KittyFormatZlib,
		CellPixelWidth: s.CellPixelWidth, CellPixelHeight: s.CellPixelHeight,
	})
	if render == "kitty" || (render != "glyph" && s.Kitty == ntpicture.KittyCapabilitySupported) {
		pic.Toggle()
	}
	return pic, id
}

// Raw probe replies must not update the old process-wide capability state.
// Geometry remains a per-model setting, but invalid reports must be ignored
// just as they are by TerminalSession.
func legacyPictureMessage(msg tea.Msg) tea.Msg {
	switch v := msg.(type) {
	case uv.KittyGraphicsEvent:
		return nil
	case uv.CellSizeEvent:
		if v.Width <= 0 || v.Height <= 0 {
			return nil
		}
	}
	return msg
}

func updateTerminalPicture(pic *picture.Model, session *ntpicture.TerminalSession, autoKitty bool, msg tea.Msg) tea.Cmd {
	s := session.Snapshot()
	geometry := pic.SetCellPixelSize(s.CellPixelWidth, s.CellPixelHeight)
	cmd := pic.Update(legacyPictureMessage(msg))
	if autoKitty && s.Kitty == ntpicture.KittyCapabilitySupported && pic.Mode() != picture.PictureKitty {
		cmd = tea.Batch(cmd, pic.Toggle())
	}
	return tea.Batch(geometry, cmd)
}
