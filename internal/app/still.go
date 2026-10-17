package app

import (
	"image"
	"image/color"

	tea "charm.land/bubbletea/v2"
	"github.com/NimbleMarkets/ntcharts/v2/picture"
	"github.com/charmbracelet/x/ansi"
	ntpicture "nimbleterminal.dev/ntpicture"
)

// stillPicture keeps only host policy and the full-resolution export source.
// Picture owns preparation, retained placements, cancellation and retirement.
type stillPicture struct {
	pic              *ntpicture.Picture
	background       color.RGBA64
	submitted, front image.Image
	presented        uint64
	width, height    int
}

func (s *stillPicture) view() string {
	if view := s.pic.View(); view != "" {
		return view
	}
	if err := s.pic.Status().Err; err != nil {
		return ansi.Truncate("Cannot display image: "+safe(err.Error()), s.width, "…")
	}
	if s.pic.Status().Busy {
		return ansi.Truncate("Preparing image…", s.width, "")
	}
	return ""
}

func (m *Model) showStill(img image.Image) tea.Cmd {
	background := color.RGBA64Model.Convert(m.pictureBackground()).(color.RGBA64)
	cleanup := m.clearStandalone()
	if m.still != nil && m.still.background != background {
		cleanup = tea.Batch(cleanup, m.clearStill())
	}
	if m.still == nil {
		pending := m.pic.SetImage(nil)
		m.retireAnimationPicture(pending)
		cleanup = tea.Batch(cleanup, pending, m.clearAnimationFront())
		m.still = &stillPicture{background: background}
		m.still.pic = ntpicture.New(ntpicture.Options{
			Session: m.terminal,
			Render:  ntpicture.Config{KittyZ: -1, Background: background, KittyFormat: ntpicture.KittyFormatZlib},
			// The existing components reserve ID blocks; using that same allocator
			// prevents collisions until they too adopt ntpicture's single-ID allocator.
			NextID: func() (int, error) { id := m.presentation.NextImageID(); m.picID = id; return id, nil },
		})
	}
	s := m.still
	s.submitted = m.source
	if s.submitted == nil {
		s.submitted = img
	}
	cw, ch := m.cellPixelSize()
	s.width, s.height = max(1, m.width), max(1, m.bodyHeight())
	return tea.Batch(cleanup, s.pic.SetTerminal(m.pic.Mode() == picture.PictureKitty, cw, ch), s.pic.SetSize(s.width, s.height), s.pic.SetImageImmutable(img))
}

func (m *Model) updateStill(msg tea.Msg) tea.Cmd {
	s := m.still
	if s == nil {
		return nil
	}
	cw, ch := m.cellPixelSize()
	cmd := tea.Batch(s.pic.SetTerminal(m.pic.Mode() == picture.PictureKitty, cw, ch), s.pic.Update(msg))
	status := s.pic.Status()
	if status.Presented != s.presented && status.Presented == status.Generation {
		s.presented, s.front = status.Presented, s.submitted
	}
	return cmd
}

func (m *Model) resizeStill() tea.Cmd {
	if m.still == nil {
		return nil
	}
	m.still.width, m.still.height = max(1, m.width), max(1, m.bodyHeight())
	return m.still.pic.SetSize(m.still.width, m.still.height)
}

func (m *Model) clearStill() tea.Cmd {
	if m.still == nil {
		return nil
	}
	s := m.still
	m.still = nil
	return s.pic.Close()
}

func (m *Model) hasPictureFront() bool {
	if m.playback != nil {
		return m.playback.Status().Presented != nil
	}
	if m.typesetLayers != nil {
		return m.typesetLayers.View() != ""
	}
	if m.still != nil {
		return m.still.front != nil
	}
	return m.presentation.Front != nil
}
func (m *Model) pictureBusy() bool {
	if m.playback != nil {
		return m.playback.Status().Picture.Busy
	}
	if m.typesetLayers != nil {
		return m.typesetLayers.Status().Picture.Busy
	}
	if m.still != nil {
		return m.still.pic.Status().Busy
	}
	return m.presentation.Awaiting
}
func (m *Model) pictureExact(w, h int) bool {
	if m.playback != nil {
		return m.playback.Mode() == ntpicture.PictureKitty && m.playback.Status().Presented != nil
	}
	if m.typesetLayers != nil {
		return m.typesetLayers.Mode() == ntpicture.PictureKitty && m.typesetLayers.View() != ""
	}
	if m.still != nil {
		return m.still.pic.Mode() == ntpicture.PictureKitty && m.still.front != nil && m.still.width == w && m.still.height == h
	}
	return m.presentation.Exact(w, h)
}
