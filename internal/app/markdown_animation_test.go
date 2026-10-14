package app

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	ntanimation "nimbleterminal.dev/ntpicture/animation"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"nimbleterminal.dev/gloss/internal/animate"
	"nimbleterminal.dev/gloss/internal/document"
)

func inlineAnimated(t *testing.T) *markdownView {
	t.Helper()
	r := image.Rect(0, 0, 4, 2)
	red, green := image.NewRGBA(r), image.NewRGBA(r)
	draw.Draw(red, r, image.NewUniform(color.RGBA{255, 0, 0, 255}), image.Point{}, draw.Src)
	draw.Draw(green, r, image.NewUniform(color.RGBA{0, 255, 0, 255}), image.Point{}, draw.Src)
	p, err := ntanimation.NewPlayer(&ntanimation.Clip{Bounds: r, Frames: []ntanimation.ClipFrame{{Image: red, Delay: time.Second, Blend: draw.Src}, {Image: green, Delay: time.Second, Blend: draw.Src}}})
	if err != nil {
		t.Fatal(err)
	}
	md := &document.Markdown{Source: []byte("![first](a.gif)\n\n" + strings.Repeat("paragraph\n\n", 20) + "![last](b.gif)\n"), Images: []document.MarkdownImage{{Destination: "a.gif", Image: red, Animation: p}, {Destination: "b.gif", Image: red, Animation: p}}}
	v := newMarkdownView(md, 1000)
	v.layout(40, 10, 8, 16)
	t.Cleanup(func() { v.close() })
	return v
}
func TestInlineAnimationVisibilityAndSource(t *testing.T) {
	v := inlineAnimated(t)
	v.scheduleAnimations(true)
	a, b := v.animators[0], v.animators[1]
	if !a.Pending || b.Pending {
		t.Fatal("offscreen playback")
	}
	tick := animate.Tick{Owner: a, Epoch: a.Epoch}
	compose, _ := a.Update(tick)
	frame := compose()
	a.Update(frame)
	if a.Player.Frame() != 2 {
		t.Fatal("frame did not reach first picture")
	}
	if b.Player.Frame() != 1 {
		t.Fatal("frame reached wrong picture")
	}
	v.key("end")
	v.scheduleAnimations(true)
	if a.Pending || a.HasImage() || !b.Pending {
		t.Fatal("scroll did not switch active image")
	}
	v.key("home")
	v.scheduleAnimations(true)
	if a.Player.Frame() != 2 || !a.Pending {
		t.Fatal("scroll back restarted")
	}
	v.raw = true
	v.layout(40, 10, 8, 16)
	v.scheduleAnimations(true)
	if a.Pending || b.Pending || v.animatedFrame() != nil {
		t.Fatal("source still animates")
	}
	a.Update(frame)
	if a.HasImage() {
		t.Fatal("stale composition restored hidden picture")
	}
}
func TestInlineExportFirstVisibleCurrentFrame(t *testing.T) {
	v := inlineAnimated(t)
	a := v.animators[0]
	p, _ := a.Player.Next()
	a.SetPlayer(p)
	a.Show(p.Image())
	var data []byte
	m := New(Options{Files: []string{"doc.md"}, Save: func(_ string, b []byte) (string, error) { data = b; return "doc.png", nil }})
	defer m.Close()
	m.kind = "markdown"
	m.markdown = v
	m.loading = false
	cmd := m.export()
	if cmd == nil {
		t.Fatal("no inline export")
	}
	result := cmd().(exportResult)
	if result.err != nil {
		t.Fatal(result.err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	red, green, _, _ := img.At(0, 0).RGBA()
	if green <= red {
		t.Fatal("exported first frame instead of current")
	}
	v.key("end")
	if v.animatedFrame() != v.animators[1].Source.Image() {
		t.Fatal("export target did not follow visibility")
	}
}

func TestInlineReloadAndLeavingInvalidatePlayback(t *testing.T) {
	m := animated(t, "glyph", 0)
	files := m.loader.Files.(fstest.MapFS)
	files["doc.md"] = &fstest.MapFile{Data: []byte("![motion](motion.gif)\n")}
	m.opts.Files = append(m.opts.Files, "doc.md")
	m.index = len(m.opts.Files) - 1
	m.loaded(m.loader.Load(m.request(false)))
	if m.markdown == nil || m.markdown.animators[0] == nil {
		t.Fatal("missing inline player")
	}
	old := m.markdown.animators[0]
	next, _ := old.Player.Next()
	old.SetPlayer(next)
	old.Schedule(true)
	stale := animate.Tick{Owner: old, Epoch: old.Epoch}
	m.loaded(m.loader.Load(m.request(true)))
	if m.markdown.animators[0].Player.Frame() != 1 || old.Source != nil || old.Pending {
		t.Fatal("reload did not restart and retire old playback")
	}
	if cmd, _ := old.Update(stale); cmd != nil {
		t.Fatal("old timer survived reload")
	}
	current := m.markdown.animators[0]
	current.Schedule(true)
	m.clearGraphics()
	if current.Source != nil || current.Pending {
		t.Fatal("leaving retained playback")
	}
}

func TestInlineKeepScreenFreezesFrame(t *testing.T) {
	v := inlineAnimated(t)
	a := v.animators[0]
	v.scheduleAnimations(true)
	before := v.view()
	v.keep()
	v.scheduleAnimations(false)
	if a.Pending || a.Source != nil || v.animators[0] != nil {
		t.Fatal("keep retained playback")
	}
	if v.view() != before {
		t.Fatal("keep changed the visible glyph frame")
	}
}

func TestHTMLInlineAnimation(t *testing.T) {
	v := inlineAnimated(t)
	v.doc.Source = []byte(`<p align="center"><a href="https://example.invalid"><img src="a.gif" width="16"></a></p>`)
	v.doc.Images = v.doc.Images[:1]
	v.doc.Images[0].Layout = document.ImageLayout{Width: 16, Align: "center"}
	v.renderedText = nil
	v.layout(40, 10, 8, 16)
	v.scheduleAnimations(true)
	a := v.animators[0]
	if !a.Pending || !v.visibleImages()[0] {
		t.Fatal("HTML GIF not visible/playing")
	}
	if v.renderedText.imageLinks[0] != "https://example.invalid" {
		t.Fatal("HTML image link missing")
	}
	compose, _ := a.Update(animate.Tick{Owner: a, Epoch: a.Epoch})
	a.Update(compose())
	if a.Player.Frame() != 2 {
		t.Fatal("HTML GIF did not animate")
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
