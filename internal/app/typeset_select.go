package app

import (
	"image"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"nimbleterminal.dev/gloss/internal/document"
	"nimbleterminal.dev/gloss/internal/i18n"
	"nimbleterminal.dev/nttypeset"
)

// Selecting text on a typeset page. The page is a picture, so the terminal
// cannot select from it: a left drag over it asks the loader to highlight
// the text between two page points, and y copies that text through the
// terminal's clipboard. Each end of the selection comes back placed in the
// text, so it keeps through scrolling, zooming and reflowing. Shift and Alt
// drags are left to the terminal, as clicks on links leave them.

func selectionsEqual(a, b *document.TypesetSelection) bool {
	return a == b || (a != nil && b != nil && *a == *b)
}

func (m *Model) clearTypesetSelection() {
	m.typesetSelect, m.typesetSelected, m.typesetDragging = nil, "", false
}

// typesetPixel maps a body cell to a pixel of the shown page picture, which
// is fitted to the body as the picture library fits it, in the cell shape
// it draws (typesetCell), letterboxing in whole cells. With clamp, a cell beside the picture maps to its nearest
// edge pixel, for a drag that leaves it.
func (m *Model) typesetPixel(x, y int, clamp bool) (image.Point, bool) {
	cw, ch := m.typesetCell()
	tw, th := m.width*cw, m.bodyHeight()*ch
	iw, ih, ok := m.typesetFitted()
	if !ok {
		return image.Point{}, false
	}
	b := m.source.Bounds()
	sw, sh := b.Dx(), b.Dy()
	px, py := x*cw+cw/2-(tw-iw)/2, y*ch+ch/2-(th-ih)/2
	if clamp {
		px, py = max(0, min(iw-1, px)), max(0, min(ih-1, py))
	}
	if px < 0 || py < 0 || px >= iw || py >= ih {
		return image.Point{}, false
	}
	return image.Pt(b.Min.X+px*sw/iw, b.Min.Y+py*sh/ih), true
}

// typesetFitted is the size the page picture is drawn at in the body, in
// the pixels of typesetCell.
func (m *Model) typesetFitted() (iw, ih int, ok bool) {
	cw, ch := m.typesetCell()
	tw, th := m.width*cw, m.bodyHeight()*ch
	b := m.source.Bounds()
	sw, sh := b.Dx(), b.Dy()
	if tw <= 0 || th <= 0 || sw <= 0 || sh <= 0 {
		return 0, 0, false
	}
	iw, ih = tw, sh*tw/sw
	if sw*th < sh*tw {
		ih = th
		iw = sw * th / sh
	}
	return max(1, iw), max(1, ih), true
}

// typesetCellAt is the body cell a pixel of the shown page picture is drawn
// in, the inverse of typesetPixel, kept within the body.
func (m *Model) typesetCellAt(p image.Point) image.Point {
	cw, ch := m.typesetCell()
	tw, th := m.width*cw, m.bodyHeight()*ch
	iw, ih, ok := m.typesetFitted()
	if !ok {
		return image.Point{}
	}
	b := m.source.Bounds()
	px := (p.X-b.Min.X)*iw/b.Dx() + (tw-iw)/2
	py := (p.Y-b.Min.Y)*ih/b.Dy() + (th-ih)/2
	return image.Pt(max(0, min(m.width-1, px/cw)), max(0, min(m.bodyHeight()-1, py/ch)))
}

// typesetPagePoint is the page point under a body cell, while the picture
// shows the part of the page the viewer is at.
func (m *Model) typesetPagePoint(x, y int, clamp bool) (image.Point, bool) {
	shown := m.typesetShown
	if m.typeset == nil || m.source == nil || shown == nil || shown.Width <= 0 || !m.typesetViewMatches() {
		return image.Point{}, false
	}
	px, ok := m.typesetPixel(x, y, clamp)
	if !ok {
		return image.Point{}, false
	}
	b := m.source.Bounds()
	view := nttypeset.View{Top: shown.Top, Left: shown.Left, Scale: float64(b.Dx()) / float64(shown.Width)}
	return view.Unproject(px.Sub(b.Min)), true
}

// typesetSelectMouse follows a left drag over the typeset page. It says
// whether the message was the page's, so that nothing else answers it.
func (m *Model) typesetSelectMouse(msg tea.Msg) (tea.Cmd, bool) {
	if m.typeset == nil || m.markdown != nil || m.help || m.screen != screenDocument || m.layer != layerNone || m.isPreview {
		return nil, false
	}
	switch v := msg.(type) {
	case tea.MouseClickMsg:
		m.typesetDragging = false
		if v.Button != tea.MouseLeft || v.Mod&(tea.ModShift|tea.ModAlt) != 0 {
			return nil, false
		}
		var cmd tea.Cmd
		if m.typesetSelect != nil {
			m.clearTypesetSelection()
			cmd = m.queueTypesetReflow()
		}
		if p, ok := m.typesetPagePoint(v.X, v.Y, false); ok {
			m.typesetDragging, m.typesetDragFrom = true, p
		}
		return cmd, true
	case tea.MouseMotionMsg:
		if !m.typesetDragging {
			return nil, false
		}
		p, ok := m.typesetPagePoint(v.X, v.Y, true)
		if !ok {
			return nil, true
		}
		// A new value, not a change in place: the shown selection and a
		// request in flight may hold the one this came from.
		sel := document.TypesetSelection{Anchor: document.TypesetPlace{Point: m.typesetDragFrom}}
		if m.typesetSelect != nil {
			sel.Anchor = m.typesetSelect.Anchor
		}
		sel.Focus = document.TypesetPlace{Point: p}
		m.typesetSelect = &sel
		return m.queueTypesetSelection(), true
	case tea.MouseReleaseMsg:
		if !m.typesetDragging {
			return nil, false
		}
		m.typesetDragging = false
		if m.typesetSelected == "" && selectionsEqual(m.typesetSelect, m.typesetSelectShown) {
			m.typesetSelect, m.typesetSelectShown = nil, nil
		}
		return nil, true
	}
	return nil, false
}

// typesetSelectKey copies the selection on y and clears it on Esc.
func (m *Model) typesetSelectKey(k string) (tea.Cmd, bool) {
	switch k {
	case "y":
		if m.typesetSelected == "" {
			return nil, true
		}
		m.note = i18n.Tn("app.note.copiedChars", "copied {{.Count}} character", "copied {{.Count}} characters", utf8.RuneCountInString(m.typesetSelected))
		return tea.SetClipboard(m.typesetSelected), true
	case "esc":
		if m.typesetSelect == nil && m.typesetSelectShown == nil {
			return nil, false
		}
		m.clearTypesetSelection()
		return m.queueTypesetReflow(), true
	}
	return nil, false
}
