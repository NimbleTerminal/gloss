package app

import (
	"image"
	"net/url"
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

type linkOpened struct {
	generation uint64
	err        error
}

// Only explicit web addresses are delegated to a host browser. In particular,
// Markdown cannot make the OS execute a local file or a custom URL scheme.
func webLink(address string) string {
	if strings.IndexFunc(address, unicode.IsControl) >= 0 {
		return ""
	}
	u, err := url.Parse(address)
	if err != nil || u.Hostname() == "" || u.User != nil || (!strings.EqualFold(u.Scheme, "http") && !strings.EqualFold(u.Scheme, "https")) {
		return ""
	}
	return address
}

// oscLinkAt measures terminal cells, including wide and combining characters.
func oscLinkAt(text string, x int) string {
	active := ""
	column := 0
	var state byte
	for len(text) > 0 {
		seq, w, n, next := ansi.DecodeSequence(text, state, nil)
		if n == 0 {
			break
		}
		state = next
		text = text[n:]
		if strings.HasPrefix(seq, "\x1b]8;") {
			value := strings.TrimSuffix(strings.TrimSuffix(seq[4:], "\a"), "\x1b\\")
			if _, address, ok := strings.Cut(value, ";"); ok {
				active = address
			}
		}
		if w > 0 && x >= column && x < column+w {
			return webLink(active)
		}
		column += w
	}
	return ""
}
func (m *Model) linkAt(x, y int) string {
	if m.loading || m.help || m.layer != layerNone || m.screen != screenDocument || m.isPreview || m.suspended || m.quitting || x < 0 || x >= m.width || y < 0 || y >= m.bodyHeight() {
		return ""
	}
	if md := m.markdown; md != nil {
		row := md.offset + y
		if md.raw || row >= len(md.lines) {
			return ""
		}
		if index := md.lines[row].image; index >= 0 {
			if md.renderedText != nil {
				return webLink(md.renderedText.imageLinks[index])
			}
			return ""
		}
		return oscLinkAt(md.lines[row].text, x)
	}
	if m.typeset == nil || m.source == nil || !m.typesetMatches() || m.typesetActive != 0 || m.presentation.Awaiting {
		return ""
	}
	// Match picture's FitContain geometry, including its integer letterboxing.
	cw, ch := m.pic.CellPixelSize()
	tw, th := m.width*cw, m.bodyHeight()*ch
	b := m.source.Bounds()
	sw, sh := b.Dx(), b.Dy()
	if tw <= 0 || th <= 0 || sw <= 0 || sh <= 0 {
		return ""
	}
	iw, ih := tw, sh*tw/sw
	if sw*th < sh*tw {
		ih = th
		iw = sw * th / sh
	}
	iw, ih = max(1, iw), max(1, ih)
	px, py := x*cw+cw/2-(tw-iw)/2, y*ch+ch/2-(th-ih)/2
	if px < 0 || py < 0 || px >= iw || py >= ih {
		return ""
	}
	at := image.Pt(b.Min.X+px*sw/iw, b.Min.Y+py*sh/ih)
	for _, link := range m.typesetLinks {
		if at.In(link.Rect) {
			return webLink(link.URL)
		}
	}
	return ""
}
func (m *Model) linkMouse(msg tea.Msg) (tea.Cmd, bool) {
	// Keep hover independent of the host's browser opener. Resolve the URL
	// against the displayed viewport in View, never retain stale link targets.
	switch v := msg.(type) {
	case tea.MouseMotionMsg:
		m.hoverLinkPoint = image.Pt(v.X, v.Y)
		m.hoverLinkActive = m.typeset != nil && v.Button == tea.MouseNone
	case tea.MouseClickMsg, tea.MouseReleaseMsg, tea.MouseWheelMsg,
		tea.KeyPressMsg, tea.WindowSizeMsg, tea.BlurMsg:
		m.hoverLinkActive = false
	}
	if m.opts.OpenLink == nil {
		return nil, false
	}
	switch v := msg.(type) {
	case tea.MouseClickMsg:
		m.pressedLink = ""
		if v.Button != tea.MouseLeft || v.Mod&(tea.ModShift|tea.ModAlt) != 0 {
			return nil, false
		}
		m.pressedLink = m.linkAt(v.X, v.Y)
		m.linkPoint = image.Pt(v.X, v.Y)
		return nil, m.pressedLink != ""
	case tea.MouseMotionMsg:
		if image.Pt(v.X, v.Y) != m.linkPoint {
			m.pressedLink = ""
		}
	case tea.MouseWheelMsg, tea.KeyPressMsg, tea.WindowSizeMsg:
		m.pressedLink = ""
	case tea.MouseReleaseMsg:
		address := m.pressedLink
		m.pressedLink = ""
		if v.Button != tea.MouseLeft || address == "" {
			return nil, false
		}
		if image.Pt(v.X, v.Y) != m.linkPoint || m.linkAt(v.X, v.Y) != address {
			return nil, true
		}
		open, generation := m.opts.OpenLink, m.generation
		return func() tea.Msg { return linkOpened{generation, open(address)} }, true
	}
	return nil, false
}
