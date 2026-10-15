//go:build !js

package app

import (
	"fmt"
	"image"
	"image/color"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/ansi/kitty"
	"github.com/makiuchi-d/gozxing"
	decoder "github.com/makiuchi-d/gozxing/qrcode"
	ntpicture "nimbleterminal.dev/ntpicture"
)

const handoffURL = "http://192.168.9.216:54321/0123456789abcdef0123456789abcdef/"

func TestHandoffSessionGraphicsChoice(t *testing.T) {
	t.Setenv("NTCHARTS_KITTY", "")
	t.Setenv("TERM", "xterm-kitty")
	t.Setenv("NTCHARTS_TMUX_PASSTHROUGH", "false")
	for _, manual := range []bool{false, true} {
		m := handoff(t, "")
		if m.kitty {
			t.Fatal("enabled graphics before discovery")
		}
		raw := collectRaw(m.terminal.Init())
		match := regexp.MustCompile(`a=q,t=d,f=24,s=1,v=1,i=(\d+)`).FindStringSubmatch(raw)
		if len(match) != 2 {
			t.Fatalf("missing session probe: %q", raw)
		}
		if manual {
			m.Update(press("g")) // Stay in glyph mode while support is unknown.
		}
		id, _ := strconv.Atoi(match[1])
		_, cmd := m.Update(uv.KittyGraphicsEvent{Options: kitty.Options{ID: id}, Payload: []byte("OK")})
		pump(m, cmd, 0)
		if m.kitty == manual || strings.ContainsRune(m.qr.View(), kitty.Placeholder) == manual {
			t.Fatalf("discovery ignored graphics policy: manual=%v kitty=%v", manual, m.kitty)
		}
		if !manual {
			_, cmd = m.Update(press("g"))
			pump(m, cmd, 0)
		}
		m.Update(ntpicture.TerminalSessionMsg{Session: m.terminal, State: m.terminal.Snapshot()})
		if m.kitty {
			t.Fatal("session notification overrode the user's glyph choice")
		}
		_, cmd = m.Update(press("g"))
		pump(m, cmd, 0)
		if !strings.ContainsRune(m.qr.View(), kitty.Placeholder) {
			t.Fatal("could not reenable discovered graphics")
		}
	}
	m := handoff(t, "glyph")
	m.Update(press("g"))
	if m.kitty {
		t.Fatal("forced unsupported capability enabled graphics")
	}
}

func handoff(t *testing.T, render string) *Handoff {
	t.Helper()
	m, err := NewHandoff(handoffURL, "Send a photo", render, make(chan struct{}), func() {})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Close() })
	_, cmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 45})
	pump(m, cmd, 0)
	return m
}

func TestHandoffGlyphContainsExactURLAndFits(t *testing.T) {
	m := handoff(t, "glyph")
	view := m.View()
	if !view.AltScreen || !strings.Contains(view.Content, handoffURL) || !strings.Contains(view.Content, "Send a photo") || !strings.Contains(view.Content, m.qr.View()) {
		t.Fatal("missing request, address, or complete QR")
	}
	// Decode from the actual half-block output, including explicit colors,
	// rather than from the encoder's image or module matrix.
	rows := strings.Split(m.qr.View(), "\n")
	width := ansi.StringWidth(rows[0])
	img := image.NewGray(image.Rect(0, 0, width*4, len(rows)*8))
	tokens := regexp.MustCompile("\x1b\\[[0-9;]+m|▀")
	for y, row := range rows {
		x, fg, bg := 0, -1, -1
		for _, token := range tokens.FindAllString(row, -1) {
			if token == "\x1b[0m" {
				fg, bg = -1, -1
			} else if token != "▀" {
				var g, b, bgG, bgB int
				if n, err := fmt.Sscanf(token, "\x1b[38;2;%d;%d;%d;48;2;%d;%d;%dm", &fg, &g, &b, &bg, &bgG, &bgB); err != nil || n != 6 || fg != g || g != b || bg != bgG || bgG != bgB {
					t.Fatalf("unexpected QR colors: %q", token)
				}
			} else {
				if (fg != 0 && fg != 255) || (bg != 0 && bg != 255) {
					t.Fatal("QR depends on terminal theme colors")
				}
				for dy := range 8 {
					value := fg
					if dy >= 4 {
						value = bg
					}
					for dx := range 4 {
						img.SetGray(x*4+dx, y*8+dy, color.Gray{Y: uint8(value)})
					}
				}
				x++
			}
		}
		if x != width {
			t.Fatal("incomplete QR row")
		}
	}
	bitmap, _ := gozxing.NewBinaryBitmapFromImage(img)
	result, err := decoder.NewQRCodeReader().Decode(bitmap, nil)
	if err != nil || result.GetText() != handoffURL {
		t.Fatalf("wrong handoff encoded: %v %v", result, err)
	}
	m.Update(tea.WindowSizeMsg{Width: 20, Height: 10})
	if strings.Contains(m.View().Content, "▀") || m.qr.Err() == nil {
		t.Fatal("undersized screen cropped a QR")
	}
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 45})
	m.Update(uv.CellSizeEvent{Width: 8, Height: 17})
	if m.qr.Err() != nil || !strings.Contains(m.View().Content, "▀") {
		t.Fatal("resize did not recover the QR")
	}
}

func TestHandoffKittyCleanupAndCancellation(t *testing.T) {
	for _, key := range []tea.KeyPressMsg{press("q"), {Code: tea.KeyEscape}, {Code: 'c', Mod: tea.ModCtrl}} {
		m := handoff(t, "kitty")
		if !strings.ContainsRune(m.View().Content, kitty.Placeholder) {
			t.Fatal("Kitty frames did not reach the handoff view")
		}
		cancelled := 0
		m.cancel = func() { cancelled++ }
		for range 2 {
			_, cmd := m.Update(key)
			pump(m, cmd, 0)
		}
		if cancelled != 1 || !m.closing {
			t.Fatal("cancel should reach the server exactly once")
		}
		_, cmd := m.Update(handoffFinished{})
		if raw := collectRaw(cmd); !strings.Contains(raw, "a=d,d=I,i=") || m.qr.View() != "" {
			t.Fatal("settlement did not clean up the QR")
		}
	}
	m := handoff(t, "kitty")
	_, cmd := m.Update(press("g"))
	if raw := collectRaw(cmd); !strings.Contains(raw, "a=d,d=I,i=") || !strings.Contains(m.View().Content, "▀") {
		t.Fatal("glyph toggle failed to retire Kitty image")
	}
}

func TestHandoffPumpWaitsForSlowKittyFrame(t *testing.T) {
	m, err := NewHandoff(handoffURL, "Send a photo", "kitty", make(chan struct{}), func() {})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Close() })
	_, cmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 45})
	// Force a render command past the helper's timer cutoff, as can happen
	// under the race detector on a busy CI runner.
	pump(m, func() tea.Msg {
		time.Sleep(2 * pumpPatience)
		return cmd()
	}, 0)
	if !strings.ContainsRune(m.View().Content, kitty.Placeholder) {
		t.Fatal("slow Kitty frame was discarded as a timer")
	}
	collectRaw(m.qr.Close())
}
