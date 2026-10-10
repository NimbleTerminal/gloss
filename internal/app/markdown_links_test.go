package app

import (
	tea "charm.land/bubbletea/v2"
	"fmt"
	"image"
	"strings"
	"testing"

	"github.com/NimbleTerminal/gloss/internal/document"
	"github.com/charmbracelet/x/ansi"
)

func TestMarkdownHyperlinkSurvivesViewportClipping(t *testing.T) {
	const address = "https://example.org/"
	label := strings.Repeat("X", 24)
	for _, source := range []string{
		"[" + label + "](" + address + ")",
		"- [" + label + "](" + address + ")",
		"| Link |\n| --- |\n| [" + label + "](" + address + ") |",
	} {
		for _, width := range []int{8, 16, 60} {
			t.Run(fmt.Sprintf("%d/%s", width, source[:1]), func(t *testing.T) {
				v := newMarkdownView(&document.Markdown{Source: []byte(source)}, 100)
				defer v.close()
				v.layout(width, 2, 8, 16)
				count := 0
				// Every row can be the first row after scrolling; OSC state cannot depend
				// on a link opening on an earlier, now-hidden row.
				for _, line := range v.lines {
					text := line.text
					var state byte
					active := ""
					for len(text) > 0 {
						seq, w, n, next := ansi.DecodeSequence(text, state, nil)
						if n == 0 {
							t.Fatal("parser made no progress")
						}
						state = next
						text = text[n:]
						if strings.HasPrefix(seq, "\x1b]8;") {
							value := strings.TrimSuffix(strings.TrimSuffix(seq[4:], "\a"), "\x1b\\")
							if _, url, ok := strings.Cut(value, ";"); ok {
								active = url
							}
						}
						if w > 0 && strings.Contains(seq, "X") {
							count++
							if active != address {
								t.Fatalf("link lost on wrapped row %q", line.text)
							}
						}
					}
				}
				if count != len(label) {
					t.Fatalf("linked characters: %d", count)
				}
			})
		}
	}
}

func TestMarkdownLinkClicks(t *testing.T) {
	const address = "https://example.org/"
	m := searchDocument(t, "[Click here]("+address+")\n\n"+strings.Repeat("paragraph\n\n", 20))
	m.markdown.raw = false
	m.layoutMarkdown()
	opened := ""
	m.opts.OpenLink = func(s string) error { opened = s; return nil }
	x, y := -1, -1
	for row := 0; row < m.bodyHeight(); row++ {
		for col := 0; col < m.width; col++ {
			if m.linkAt(col, row) == address {
				x, y = col, row
				break
			}
		}
		if x >= 0 {
			break
		}
	}
	if x < 0 {
		t.Fatal("no link hit area")
	}
	m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	if opened != "" {
		t.Fatal("opened on press")
	}
	_, cmd := m.Update(tea.MouseReleaseMsg{X: x, Y: y, Button: tea.MouseLeft})
	drainAnimation(t, m, cmd)
	if opened != address {
		t.Fatal("click not opened", opened)
	}
	opened = ""
	m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	m.Update(tea.MouseMotionMsg{X: x + 1, Y: y, Button: tea.MouseLeft})
	_, cmd = m.Update(tea.MouseReleaseMsg{X: x, Y: y, Button: tea.MouseLeft})
	drainAnimation(t, m, cmd)
	if opened != "" {
		t.Fatal("drag activated link")
	}
	m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	if m.markdown.offset == 0 {
		t.Fatal("lost wheel scrolling")
	}
	m.markdown.offset = 0
	for _, hidden := range []string{"help", "source", "overlay", "loading"} {
		switch hidden {
		case "help":
			m.help = true
		case "source":
			m.markdown.raw = true
		case "overlay":
			m.layer = layerInfo
		case "loading":
			m.loading = true
		}
		if m.linkAt(x, y) != "" {
			t.Fatal("active hidden link", hidden)
		}
		m.help = false
		m.markdown.raw = false
		m.layer = layerNone
		m.loading = false
	}
}

func TestOSCLinkCellsAndSchemes(t *testing.T) {
	const address = "https://example.org/"
	text := "a" + ansi.SetHyperlink(address) + "界e\u0301" + ansi.ResetHyperlink() + "z"
	for x := 0; x < 5; x++ {
		got := oscLinkAt(text, x)
		want := ""
		if x >= 1 && x < 4 {
			want = address
		}
		if got != want {
			t.Fatalf("cell %d: %q", x, got)
		}
	}
	for _, address := range []string{"javascript:alert(1)", "file:///tmp/a", "https://", "../readme.md", "https://example.org/\n", "https://user:secret@example.org/"} {
		if webLink(address) != "" {
			t.Fatal("unsafe/non-web scheme accepted", address)
		}
	}
}

func TestLinkedMarkdownImageClick(t *testing.T) {
	const address = "https://example.org/"
	m := searchDocument(t, "")
	m.markdown = newMarkdownView(&document.Markdown{Source: []byte("[![picture](local.png)](" + address + ")"), Images: []document.MarkdownImage{{Image: image.NewRGBA(image.Rect(0, 0, 4, 2))}}}, 100)
	m.layoutMarkdown()
	found := false
	for y, line := range m.markdown.lines {
		if line.image >= 0 && y < m.bodyHeight() {
			found = true
			if got := m.linkAt(0, y); got != address {
				t.Fatal("linked figure", got)
			}
		}
	}
	if !found {
		t.Fatal("no figure")
	}
}

func TestTypesetLinkHitGeometry(t *testing.T) {
	const address = "https://example.org/"
	m := New(Options{Files: []string{"doc.md"}, Typeset: true, Render: "glyph"})
	defer m.Close()
	m.width, m.height = 40, 12
	m.kind = "markdown"
	m.typeset = &document.Markdown{}
	m.typesetWidth = m.typesetMeasure()
	v := m.typesetViewport()
	v.Height = max(1, v.Height>>v.Zoom)
	m.typesetShown = &v
	m.typesetTheme = m.theme
	m.source = image.NewRGBA(image.Rect(0, 0, 320, 160))
	m.typesetLinks = []document.TypesetLink{{Rect: image.Rect(40, 16, 80, 48), URL: address}}
	if got := m.linkAt(6, 1); got != address {
		t.Fatal("link pixel mapping", got)
	}
	if m.linkAt(0, 0) != "" {
		t.Fatal("outside link")
	}
	m.presentation.Awaiting = true
	if m.linkAt(6, 1) != "" {
		t.Fatal("pending image used new hit regions")
	}
	m.presentation.Awaiting = false
	m.typesetTop++
	if m.linkAt(6, 1) != "" {
		t.Fatal("stale viewport hit")
	}
}

func TestTypesetLinkHover(t *testing.T) {
	const address = "https://example.org/"
	m := New(Options{Files: []string{"doc.md"}, Typeset: true, Render: "glyph"})
	defer m.Close()
	m.width, m.height = 40, 12
	m.kind = "markdown"
	m.typeset = &document.Markdown{}
	m.typesetWidth = m.typesetMeasure()
	v := m.typesetViewport()
	v.Height = max(1, v.Height>>v.Zoom)
	m.typesetShown, m.typesetTheme = &v, m.theme
	m.source = image.NewRGBA(image.Rect(0, 0, 320, 160))
	m.typesetLinks = []document.TypesetLink{{Rect: image.Rect(40, 16, 80, 48), URL: address}}
	if m.View().MouseMode != tea.MouseModeAllMotion {
		t.Fatal("typeset links need hover events")
	}
	hover := tea.MouseMotionMsg{X: 6, Y: 1, Button: tea.MouseNone}
	preview := func() bool {
		bar, _ := m.documentStatus(m.width, m.bodyHeight())
		return strings.Contains(ansi.Strip(bar), "Link: "+address)
	}
	m.Update(hover)
	if !preview() {
		t.Fatal("missing preview without browser opener")
	}
	m.Update(tea.MouseMotionMsg{X: 0, Y: 0})
	if preview() {
		t.Fatal("preview remained outside link")
	}
	for _, msg := range []tea.Msg{
		tea.MouseWheelMsg{Button: tea.MouseWheelDown}, tea.KeyPressMsg{Code: 'j'},
		tea.WindowSizeMsg{Width: 40, Height: 12}, tea.BlurMsg{},
		tea.MouseMotionMsg{X: 6, Y: 1, Button: tea.MouseLeft},
	} {
		m.linkMouse(hover)
		m.linkMouse(msg)
		if preview() {
			t.Fatalf("preview survived %T", msg)
		}
	}
	m.linkMouse(hover)
	m.presentation.Awaiting = true
	if preview() {
		t.Fatal("preview used pending image regions")
	}
	m.presentation.Awaiting = false
	m.help = true
	if preview() || m.View().MouseMode == tea.MouseModeAllMotion {
		t.Fatal("hover active under help")
	}
	m.help = false
	m.typeset = nil
	if preview() || m.View().MouseMode == tea.MouseModeAllMotion {
		t.Fatal("hover active outside typeset")
	}
}

func TestMarkdownLinkPromptOffsetAndHostBoundary(t *testing.T) {
	const address = "https://example.org/"
	m := searchDocument(t, "[link]("+address+")")
	m.opts.Prompt = "Choose a link"
	m.opts.PromptTop = true
	m.markdown.raw = false
	m.layoutMarkdown()
	x, y := -1, -1
	for row := 0; row < m.bodyHeight(); row++ {
		for col := 0; col < m.width; col++ {
			if m.linkAt(col, row) == address {
				x, y = col, row
				break
			}
		}
		if x >= 0 {
			break
		}
	}
	if x < 0 {
		t.Fatal("link missing")
	}
	if _, handled := m.linkMouse(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft}); handled {
		t.Fatal("host without opener captured click")
	}
	count := 0
	m.opts.OpenLink = func(string) error { count++; return fmt.Errorf("browser unavailable") }
	top := m.promptHeight()
	m.Update(tea.MouseClickMsg{X: x, Y: y + top, Button: tea.MouseLeft})
	_, cmd := m.Update(tea.MouseReleaseMsg{X: x, Y: y + top, Button: tea.MouseLeft})
	drainAnimation(t, m, cmd)
	if count != 1 || !strings.Contains(m.note, "browser unavailable") {
		t.Fatalf("prompt/open failure: %d %q", count, m.note)
	}
}
