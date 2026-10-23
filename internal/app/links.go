package app

import (
	"image"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"nimbleterminal.dev/gloss/internal/document"
)

type linkOpened struct {
	generation uint64
	err        error
}

func documentLink(address string) string {
	if _, ok := document.FootnotePeer(address); ok {
		return address
	}
	return webLink(address)
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
			return documentLink(active)
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
	if m.typeset == nil || m.source == nil || !m.typesetViewMatches() || m.typesetActive != 0 || m.pictureBusy() {
		return ""
	}
	at, ok := m.typesetPixel(x, y, false)
	if !ok {
		return ""
	}
	for _, link := range m.typesetLinks {
		if at.In(link.Rect) {
			return documentLink(link.URL)
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
	switch v := msg.(type) {
	case tea.MouseClickMsg:
		m.pressedLink = ""
		if v.Button != tea.MouseLeft || v.Mod&(tea.ModShift|tea.ModAlt) != 0 {
			return nil, false
		}
		m.pressedLink = m.linkAt(v.X, v.Y)
		if _, internal := document.FootnotePeer(m.pressedLink); !internal && m.opts.OpenLink == nil {
			m.pressedLink = ""
		}
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
		if target, internal := document.FootnotePeer(address); internal {
			return m.jumpFootnote(target), true
		}
		open, generation := m.opts.OpenLink, m.generation
		return func() tea.Msg { return linkOpened{generation, open(address)} }, true
	}
	return nil, false
}

// linkEntries lists what can be done with a link: open it in the browser,
// called label, when it is a web address and the host has one, and copy
// its address.
// fetchEntries are what --fetch allows with a web address: downloading it
// to show in gloss, or to keep.
func fetchEntries(m *Model, address string) []menuEntry {
	if m.opts.Fetch == nil || webLink(address) == "" {
		return nil
	}
	return []menuEntry{
		{"Open in gloss", func(m *Model) tea.Cmd { return m.open(address) }},
		{"Save linked file", func(m *Model) tea.Cmd { return m.saveLink(address) }},
	}
}

// bareURL is a web address written out in text, not made a link.
var bareURL = regexp.MustCompile(`https?://[^\s<>"'` + "`" + `]+`)

// bareURLAt is the web address written in text that covers byte at, without
// the punctuation a sentence puts after it; empty when there is none.
func bareURLAt(text string, at int) string {
	for _, span := range bareURL.FindAllStringIndex(text, -1) {
		if at < span[0] || at >= span[1] {
			continue
		}
		address := trimAddress(text[span[0]:span[1]])
		if at >= span[0]+len(address) {
			return "" // On the punctuation after it.
		}
		return webLink(address)
	}
	return ""
}

// trimAddress drops what a sentence puts after an address: its
// punctuation, and a closing bracket the address did not open.
func trimAddress(address string) string {
	for {
		before := address
		address = strings.TrimRight(address, ".,;:!?")
		if i := strings.IndexAny(")]}", address[len(address)-1:]); len(address) > 0 && i >= 0 {
			if strings.Count(address, "([{"[i:i+1]) < strings.Count(address, ")]}"[i:i+1]) {
				address = address[:len(address)-1]
			}
		}
		if address == before {
			return address
		}
	}
}

// bareLinkAt is the web address written out in the text at cell x, y of
// the view, found in the logical line, so that one wrapped across rows is
// whole.
func (v *markdownView) bareLinkAt(x, y int) string {
	row := v.offset + y
	if x < 0 || y < 0 || y >= v.height || row >= len(v.lines) || v.lines[row].image >= 0 {
		return ""
	}
	line := v.lines[row]
	plain := ansi.Strip(line.text)
	col := x - v.gutter + v.left
	if col < 0 || col >= ansi.StringWidth(plain) {
		return ""
	}
	at := len(ansi.Cut(plain, 0, col))
	for _, span := range line.spans {
		if at >= span.offset && at < span.offset+span.end-span.start {
			return bareURLAt(v.content.units[span.unit].plain, span.start+at-span.offset)
		}
	}
	return ""
}

// bareLinkActions opens the menu of a web address written out in text at
// the pointer: Open and Copy address, as a link's.
func (m *Model) bareLinkActions(at image.Point) tea.Cmd {
	address := m.markdown.bareLinkAt(at.X, at.Y)
	if address == "" {
		return nil
	}
	return m.openActions(address, linkEntries(m, address, "Open"), at)
}

func linkEntries(m *Model, url string, label string) []menuEntry {
	var entries []menuEntry
	if webLink(url) != "" && m.opts.OpenLink != nil {
		entries = append(entries, menuEntry{label, func(m *Model) tea.Cmd {
			open, generation := m.opts.OpenLink, m.generation
			return func() tea.Msg { return linkOpened{generation, open(url)} }
		}})
	}
	entries = append(entries, fetchEntries(m, url)...)
	return append(entries, menuEntry{"Copy address", func(m *Model) tea.Cmd {
		m.note = "Address copied"
		return tea.SetClipboard(url)
	}})
}

// typesetHoverMouse tints the link under the pointer on a typeset page,
// every line of it, and untints it when the pointer leaves it, a button is
// pressed or the page moves.
func (m *Model) typesetHoverMouse(msg tea.Msg) tea.Cmd {
	var marks []document.TypesetMark
	switch v := msg.(type) {
	case tea.MouseMotionMsg:
		if v.Button == tea.MouseNone && m.layer == layerNone && !m.help && !m.isPreview && m.markdown == nil {
			if i := m.typesetItemIndex(v.X, v.Y); i >= 0 && m.typesetItems[i].Kind == document.TypesetLinkItem {
				marks = m.typesetItems[i].Marks
			}
		}
	case tea.MouseClickMsg, tea.MouseWheelMsg:
	default:
		return nil
	}
	if slices.Equal(marks, m.typesetHover) {
		return nil
	}
	m.typesetHover = marks
	return m.queueTypesetReflow()
}

func (m *Model) jumpFootnote(target string) tea.Cmd {
	if md := m.markdown; md != nil {
		for row, line := range md.lines {
			text := line.text
			var state byte
			for len(text) > 0 {
				seq, _, n, next := ansi.DecodeSequence(text, state, nil)
				if n == 0 {
					break
				}
				text, state = text[n:], next
				if strings.HasPrefix(seq, "\x1b]8;") {
					value := strings.TrimSuffix(strings.TrimSuffix(seq[4:], "\a"), "\x1b\\")
					if _, address, ok := strings.Cut(value, ";"); ok && address == target {
						md.offset = max(0, min(row, len(md.lines)-m.bodyHeight()))
						return nil
					}
				}
			}
		}
		return nil
	}
	if m.typeset != nil {
		m.clearTypesetSelection()
		m.typesetGoLink, m.typesetEnd = target, false
		return m.queueTypesetReflow()
	}
	return nil
}
