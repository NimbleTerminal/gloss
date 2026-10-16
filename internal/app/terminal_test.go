package app

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/NimbleMarkets/ntcharts/v2/picture"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi/kitty"
	widget "nimbleterminal.dev/nt3d/widget"
	ntpicture "nimbleterminal.dev/ntpicture"
)

func TestTerminalSessionConnectionIsolation(t *testing.T) {
	a := New(Options{Render: "kitty", Files: []string{"example.png"}})
	b := New(Options{Render: "glyph", Files: []string{"example.png"}})
	t.Cleanup(func() { a.Close(); b.Close() })
	a.Update(uv.CellSizeEvent{Width: 11, Height: 23})
	if w, h := a.cellPixelSize(); w != 11 || h != 23 {
		t.Fatalf("session geometry: %dx%d", w, h)
	}
	if w, h := a.pic.CellPixelSize(); w != 11 || h != 23 {
		t.Fatalf("legacy renderer geometry: %dx%d", w, h)
	}
	if w, h := b.cellPixelSize(); w != 8 || h != 16 {
		t.Fatalf("another connection inherited geometry: %dx%d", w, h)
	}
	a.Update(uv.CellSizeEvent{Width: 0, Height: -1})
	if w, h := a.pic.CellPixelSize(); w != 11 || h != 23 {
		t.Fatalf("invalid report changed geometry: %dx%d", w, h)
	}
	// Alternating independent event loops must not borrow the compatibility
	// flag left by the other connection when interpreting a graphics toggle.
	a.Update(struct{}{})
	b.Update(press("g"))
	if b.pic.Mode() != picture.PictureGlyph || a.terminal.Snapshot().Kitty != ntpicture.KittyCapabilitySupported {
		t.Fatal("connections leaked capability or graphics mode")
	}
}

func TestTerminalSessionSharedWithPreviewAndMesh(t *testing.T) {
	m := New(Options{Render: "glyph", Files: []string{"example.png"}, Menu: true, Preview: true})
	t.Cleanup(func() { m.Close() })
	m.width, m.height = 100, 30
	raw := collectRaw(m.terminal.Init())
	if strings.Count(raw, "\x1b[16t") != 1 {
		t.Fatalf("missing connection geometry query: %q", raw)
	}
	m.updatePreview()
	if m.preview == nil || m.preview.terminal != m.terminal || m.preview.ownTerminal {
		t.Fatal("preview did not borrow the connection session")
	}
	if m.preview.terminal.Init() != nil {
		t.Fatal("preview started duplicate discovery")
	}
	m.Update(uv.CellSizeEvent{Width: 10, Height: 20})
	if w, h := m.preview.pic.CellPixelSize(); w != 10 || h != 20 {
		t.Fatalf("preview did not follow connection geometry: %dx%d", w, h)
	}
	m.disposePreview()
	m.chart = widget.New(nil, 10, 10, widget.WithTerminalSession(m.terminal))
	m.clearChart()
	m.Update(uv.CellSizeEvent{Width: 12, Height: 24})
	if w, h := m.cellPixelSize(); w != 12 || h != 24 {
		t.Fatal("closing a consumer closed the shared session")
	}
	m.Close()
	m.terminal.Update(uv.CellSizeEvent{Width: 14, Height: 28})
	if w, h := m.cellPixelSize(); w != 12 || h != 24 {
		t.Fatal("owner did not close the session")
	}
}

func TestTerminalSessionRoutesProbeReplies(t *testing.T) {
	t.Setenv("NTPICTURE_KITTY", "")
	t.Setenv("TERM", "xterm-kitty")
	makeModel := func() *Model {
		s := ntpicture.NewTerminalSession(ntpicture.TerminalOptions{SharedMemory: ntpicture.KittyCapabilityUnsupported})
		m := New(Options{terminal: s, Files: []string{"example.png"}})
		t.Cleanup(func() { m.Close(); s.Close() })
		return m
	}
	a, b := makeModel(), makeModel()
	raw := collectRaw(a.terminal.Init())
	b.terminal.Init()
	idText := regexp.MustCompile(`i=(\d+)`).FindStringSubmatch(raw)
	if len(idText) != 2 {
		t.Fatalf("missing Kitty probe: %q", raw)
	}
	id, _ := strconv.Atoi(idText[1])
	reply := uv.KittyGraphicsEvent{Options: kitty.Options{ID: id}, Payload: []byte("OK")}
	b.Update(reply)
	if b.terminal.Snapshot().Kitty != ntpicture.KittyCapabilityUnknown {
		t.Fatal("foreign probe changed session capability")
	}
	a.Update(reply)
	if a.terminal.Snapshot().Kitty != ntpicture.KittyCapabilitySupported || a.pic.Mode() != picture.PictureKitty {
		t.Fatal("session discovery did not enable automatic Kitty presentation")
	}
	a.Update(press("g"))
	a.Update(ntpicture.TerminalSessionMsg{Session: a.terminal, State: a.terminal.Snapshot()})
	if a.pic.Mode() != picture.PictureGlyph {
		t.Fatal("discovery notification overrode the user's graphics toggle")
	}
}
