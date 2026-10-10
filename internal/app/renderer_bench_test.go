package app

import (
	"io"
	"strings"
	"testing"
	"testing/fstest"

	tea "charm.land/bubbletea/v2"
	"github.com/NimbleMarkets/ntcharts/v2/picture"
	"github.com/charmbracelet/colorprofile"
	uv "github.com/charmbracelet/ultraviolet"
)

// twoFrames is the view before and after one scroll step of a page shown
// under render, at cols×rows: the two frames a keystroke makes the terminal
// renderer diff.
func twoFrames(tb testing.TB, render, mode string, cols, rows int) [2]string {
	tb.Helper()
	tb.Cleanup(func() { picture.ForceKittyCapability(picture.KittyCapabilityUnknown) })
	files := fstest.MapFS{"page.md": &fstest.MapFile{Data: []byte("# A typeset page\n\n" + strings.Repeat("Café and a paragraph of words that wraps across the measure.\n\n", 100))}}
	m := New(Options{Files: []string{"page.md"}, FilesFS: files, MarkdownMode: mode, Render: render, DPI: 150, Page: 1})
	tb.Cleanup(func() { _ = m.Close() })
	m.Update(tea.WindowSizeMsg{Width: cols, Height: rows})
	drainAnimation(tb, m, m.load(false))
	var frames [2]string
	frames[0] = m.View().Content
	cmd, _ := m.documentKey("down")
	drainAnimation(tb, m, cmd)
	frames[1] = m.View().Content
	if frames[0] == frames[1] {
		tb.Fatal("scrolling did not change the frame")
	}
	return frames
}

// BenchmarkTerminalFrame is what Bubble Tea does with a changed view between
// two keystrokes, without the TTY: draw it into the screen buffer, have
// ultraviolet's renderer diff it against the frame before and write the
// escape sequences. Each case alternates two frames so every iteration has
// a real change: the Kitty page is a grid of placeholder cells whose colors
// carry the picture ID, the glyph page is colored half-block cells, text is
// styled Markdown.
func BenchmarkTerminalFrame(b *testing.B) {
	const cols, rows = 200, 52
	cases := []struct{ name, render, mode string }{
		{"kitty", "kitty", "typeset"},
		{"glyph", "glyph", "typeset"},
		{"text", "glyph", "text"},
	}
	for _, profile := range []colorprofile.Profile{colorprofile.TrueColor, colorprofile.ANSI256} {
		for _, c := range cases {
			b.Run(c.name+"/"+profile.String(), func(b *testing.B) {
				frames := twoFrames(b, c.render, c.mode, cols, rows)
				cellbuf := uv.NewScreenBuffer(cols, rows)
				r := uv.NewTerminalRenderer(io.Discard, nil)
				r.SetColorProfile(profile)
				r.SetFullscreen(true)
				r.Resize(cols, rows)
				draw := func(content string) {
					cellbuf.Clear()
					uv.NewStyledString(content).Draw(cellbuf, cellbuf.Bounds())
					r.Render(cellbuf.RenderBuffer)
					if err := r.Flush(); err != nil {
						b.Fatal(err)
					}
				}
				draw(frames[0])
				b.ReportAllocs()
				i := 0
				for b.Loop() {
					i++
					draw(frames[i&1])
				}
			})
		}
	}
}
