package app

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"nimbleterminal.dev/gloss/examples"
	"nimbleterminal.dev/gloss/internal/document"
	ntpicture "nimbleterminal.dev/ntpicture"
	ntanimation "nimbleterminal.dev/ntpicture/animation"
)

func inlineAnimated(t *testing.T, render string) *Model {
	t.Helper()
	r := image.Rect(0, 0, 4, 2)
	red, green := image.NewRGBA(r), image.NewRGBA(r)
	draw.Draw(red, r, image.NewUniform(color.RGBA{255, 0, 0, 255}), image.Point{}, draw.Src)
	draw.Draw(green, r, image.NewUniform(color.RGBA{0, 255, 0, 255}), image.Point{}, draw.Src)
	p, err := ntanimation.NewPlayer(&ntanimation.Clip{Bounds: r, Frames: []ntanimation.ClipFrame{{Image: red, Delay: time.Hour, Blend: draw.Src}, {Image: green, Delay: time.Hour, Blend: draw.Src}}})
	if err != nil {
		t.Fatal(err)
	}
	md := &document.Markdown{Source: []byte("![first](a.gif)\n\n" + strings.Repeat("paragraph\n\n", 20) + "![last](b.gif)\n\n![still](c.png)\n"), Images: []document.MarkdownImage{{Destination: "a.gif", Image: red, Animation: p}, {Destination: "b.gif", Image: red, Animation: p}, {Destination: "c.png", Image: green, Layout: document.ImageLayout{Width: 8}}}}
	m := New(Options{Files: []string{"doc.md"}, Render: render})
	t.Cleanup(func() { m.Close() })
	m.Update(tea.WindowSizeMsg{Width: 40, Height: 12})
	m.kind = "markdown"
	m.markdown = newMarkdownView(md, m.terminal)
	for _, a := range m.markdown.animators {
		if a != nil {
			a.Pause(true)
		}
	}
	drainAnimation(t, m, tea.Batch(m.markdown.setKitty(render == "kitty"), m.layoutMarkdown(), m.markdown.syncPictures(true)))
	return m
}

func TestInlineVisibilityReflowAndLateSeek(t *testing.T) {
	for _, render := range []string{"glyph", "kitty"} {
		t.Run(render, func(t *testing.T) {
			m := inlineAnimated(t, render)
			v := m.markdown
			a, b := v.animators[0], v.animators[1]
			if !a.Visible() || b.Visible() {
				t.Fatal("offscreen playback")
			}
			drainAnimation(t, m, a.Seek(2))
			if a.Status().PresentedPlayer.Frame() != 2 || b.Status().Player.Frame() != 1 {
				t.Fatal("frame reached wrong picture")
			}
			_, cmd := m.Update(tea.WindowSizeMsg{Width: 32, Height: 12})
			drainAnimation(t, m, cmd)
			if a.Status().PresentedPlayer.Frame() != 2 {
				t.Fatal("reflow restarted playback")
			}
			_, geometry := m.Update(uv.CellSizeEvent{Width: 10, Height: 20})
			drainAnimation(t, m, geometry)
			if cw, ch := a.CellPixelSize(); cw != 10 || ch != 20 {
				t.Fatal("cell geometry did not reach inline playback")
			}
			pending := a.Seek(1)
			v.key("end")
			drainAnimation(t, m, v.syncPictures(true))
			drainAnimation(t, m, pending)
			if a.Visible() || !b.Visible() || a.View() != "" {
				t.Fatal("scroll did not switch visible images")
			}
			if a.Status().Player.Frame() != 2 {
				t.Fatal("late seek changed hidden position")
			}
			v.key("home")
			drainAnimation(t, m, v.syncPictures(true))
			if a.Status().PresentedPlayer.Frame() != 2 {
				t.Fatal("scroll back restarted playback")
			}
			v.raw = true
			drainAnimation(t, m, tea.Batch(v.layout(32, 10, 8, 16), v.syncPictures(true)))
			if a.Visible() || b.Visible() || v.animatedFrame() != nil {
				t.Fatal("source still displays animations")
			}
			for _, p := range v.pictures {
				if p.View() != "" {
					t.Fatal("source retained an inline picture")
				}
			}
		})
	}
}

func TestInlineExportFirstVisibleCommittedFrame(t *testing.T) {
	for _, render := range []string{"glyph", "kitty"} {
		t.Run(render, func(t *testing.T) {
			m := inlineAnimated(t, render)
			v := m.markdown
			a := v.animators[0]
			drainAnimation(t, m, a.Seek(2))
			pending := a.Seek(1)
			var data []byte
			m.opts.Save = func(_ string, b []byte) (string, error) { data = b; return "doc.png", nil }
			result := m.export()().(exportResult)
			if result.err != nil {
				t.Fatal(result.err)
			}
			img, err := png.Decode(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			r, g, _, _ := img.At(0, 0).RGBA()
			if g <= r {
				t.Fatal("export did not capture committed frame")
			}
			drainAnimation(t, m, pending)
			v.key("end")
			drainAnimation(t, m, v.syncPictures(true))
			if v.animatedFrame() != v.animators[1].Status().Presented.Image() {
				t.Fatal("export target did not follow visibility")
			}
		})
	}
}

func TestInlineReloadAndLeavingInvalidatePlayback(t *testing.T) {
	m := animated(t, "glyph", 0)
	files := m.loader.Files.(fstest.MapFS)
	files["doc.md"] = &fstest.MapFile{Data: []byte("![motion](motion.gif)\n")}
	m.opts.Files = append(m.opts.Files, "doc.md")
	m.index = len(m.opts.Files) - 1
	cmd := m.loaded(m.loader.Load(m.request(false)))
	m.markdown.animators[0].Pause(true)
	drainAnimation(t, m, tea.Batch(cmd, m.markdown.syncPictures(true)))
	old := m.markdown.animators[0]
	pending := old.Seek(2)
	cmd = m.loaded(m.loader.Load(m.request(true)))
	m.markdown.animators[0].Pause(true)
	drainAnimation(t, m, tea.Batch(cmd, m.markdown.syncPictures(true)))
	drainAnimation(t, m, pending)
	if !old.Closed() || m.markdown.animators[0].Status().PresentedPlayer.Frame() != 1 {
		t.Fatal("reload did not retire and restart")
	}
	current := m.markdown.animators[0]
	drainAnimation(t, m, m.clearGraphics())
	if !current.Closed() || current.View() != "" {
		t.Fatal("leaving retained playback")
	}
}

func TestInlineKeepScreenFreezesPictures(t *testing.T) {
	for _, render := range []string{"glyph", "kitty"} {
		t.Run(render, func(t *testing.T) {
			m := inlineAnimated(t, render)
			v := m.markdown
			v.key("end")
			drainAnimation(t, m, v.syncPictures(true))
			before := v.view()
			pending := v.animators[1].Seek(2)
			_, cmd := m.Update(press("Q"))
			drainAnimation(t, m, cmd)
			drainAnimation(t, m, pending)
			if !v.animators[1].Closed() || v.view() != before {
				t.Fatal("keep changed the presented frame")
			}
			m.Close()
			if v.view() != before {
				t.Fatal("close deleted kept view")
			}
		})
	}
}

func TestHTMLInlineAnimation(t *testing.T) {
	m := inlineAnimated(t, "glyph")
	v := m.markdown
	v.doc.Source = []byte(`<p align="center"><a href="https://example.invalid"><img src="a.gif" width="16"></a></p>`)
	v.doc.Images[0].Layout = document.ImageLayout{Width: 16, Align: "center"}
	v.renderedText = nil
	drainAnimation(t, m, v.layout(40, 10, 8, 16))
	drainAnimation(t, m, v.animators[0].Seek(2))
	if !v.animators[0].Visible() || v.animators[0].Status().PresentedPlayer.Frame() != 2 {
		t.Fatal("HTML GIF not playing")
	}
	if v.renderedText.imageLinks[0] != "https://example.invalid" {
		t.Fatal("HTML image link missing")
	}
	found := false
	for _, line := range v.lines {
		if line.image == 0 {
			found = true
			if line.inset != 19 {
				t.Fatalf("inset %d", line.inset)
			}
		}
	}
	if !found {
		t.Fatal("no image rows")
	}
}

func TestInlineClippingAndSharedSession(t *testing.T) {
	m := inlineAnimated(t, "kitty")
	v := m.markdown
	a := v.animators[0]
	if a.Session() != m.terminal || v.pictures[2].(*ntpicture.Picture).Session() != m.terminal {
		t.Fatal("inline pictures do not share terminal session")
	}
	first := -1
	for i, line := range v.lines {
		if line.image == 0 {
			first = i
			break
		}
	}
	v.offset = first + 1
	drainAnimation(t, m, v.syncPictures(true))
	expected := strings.Split(a.View(), "\n")[1]
	if got := strings.Split(v.view(), "\n")[0]; got != expected {
		t.Fatal("scroll did not clip first image row")
	}
	generation := a.Status().Picture.Generation
	v.offset++
	drainAnimation(t, m, v.syncPictures(true))
	if a.Status().Picture.Generation != generation {
		t.Fatal("scroll within a picture retransmitted it")
	}
}

func TestInlineActualEventLoop(t *testing.T) {
	for _, render := range []string{"glyph", "kitty"} {
		for _, file := range []string{"motion.gif", "motion.webp"} {
			t.Run(render+"/"+file, func(t *testing.T) {
				data, err := examples.Files.ReadFile(file)
				if err != nil {
					t.Fatal(err)
				}
				files := fstest.MapFS{"doc.md": {Data: []byte(fmt.Sprintf("![animated](%s)\n", file))}, file: {Data: data}}
				observer := &animationObserver{Model: New(Options{Files: []string{"doc.md"}, FilesFS: files, Render: render})}
				defer observer.Close()
				observer.control = func(m *Model) (tea.Cmd, bool) {
					if m.markdown != nil {
						a := m.markdown.animators[0]
						if a != nil && a.Status().PresentedPlayer != nil && a.Status().PresentedPlayer.Frame() >= 3 {
							return m.quit(), true
						}
					}
					return nil, false
				}
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_, err = tea.NewProgram(observer, tea.WithContext(ctx), tea.WithInput(nil), tea.WithOutput(io.Discard), tea.WithoutRenderer(), tea.WithoutSignalHandler()).Run()
				if err != nil || !observer.reached {
					t.Fatalf("inline playback stalled: %v", err)
				}
			})
		}
	}
}

func TestInlinePreviewKeepsFirstFrame(t *testing.T) {
	data, err := examples.Files.ReadFile("motion.gif")
	if err != nil {
		t.Fatal(err)
	}
	m := New(Options{Files: []string{"doc.md"}, FilesFS: fstest.MapFS{
		"doc.md": {Data: []byte("![animation](motion.gif)\n")}, "motion.gif": {Data: data},
	}, Render: "kitty"})
	defer m.Close()
	m.isPreview = true
	m.Update(tea.WindowSizeMsg{Width: 40, Height: 12})
	_, cmd := m.Update(m.loader.Load(m.request(false)))
	drainAnimation(t, m, cmd)
	if m.markdown == nil || m.markdown.animators[0] != nil {
		t.Fatal("preview created a playback controller")
	}
	p := m.markdown.pictures[0].(*ntpicture.Picture)
	if !p.Visible() || p.Status().Presented == 0 || p.View() == "" {
		t.Fatal("preview did not present its first frame")
	}
}
