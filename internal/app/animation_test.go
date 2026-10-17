package app

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"io"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"nimbleterminal.dev/gloss/examples"
	ntpicture "nimbleterminal.dev/ntpicture"
)

// drainAnimation runs finite render/cleanup commands to completion, including
// the ordered APC, grid, and presentation messages. These tests pause playback
// (or leave the document) first and drive timers explicitly. Unlike pump, this
// helper never guesses that a slow encoder is a timer and drops its result.
func drainAnimation(t testing.TB, m *Model, cmd tea.Cmd, observe ...func(tea.Msg)) {
	t.Helper()
	remaining := 1000
	var run func(tea.Cmd)
	run = func(cmd tea.Cmd) {
		if cmd == nil {
			return
		}
		if a := m.animation; a != nil && m.animationVisible() && !a.Status().Paused && !a.Status().Finished {
			t.Fatal("pause playback before draining animation commands")
		}
		remaining--
		if remaining < 0 {
			t.Fatal("animation command loop did not settle")
		}
		msg := cmd()
		for _, f := range observe {
			f(msg)
		}
		if msg == nil {
			return
		}
		if batch, ok := msg.(tea.BatchMsg); ok {
			for _, next := range batch {
				run(next)
			}
			return
		}
		// Bubble Tea's sequence message is an unexported []tea.Cmd type.
		v := reflect.ValueOf(msg)
		if v.Kind() == reflect.Slice && v.Type().Elem() == reflect.TypeOf(tea.Cmd(nil)) {
			for i := 0; i < v.Len(); i++ {
				run(v.Index(i).Interface().(tea.Cmd))
			}
			return
		}
		_, next := m.Update(msg)
		run(next)
	}
	run(cmd)
}

func animated(t *testing.T, render string, loop int) *Model {
	t.Helper()
	palette := color.Palette{color.RGBA{255, 0, 0, 255}, color.RGBA{0, 255, 0, 255}}
	first, second := image.NewPaletted(image.Rect(0, 0, 4, 2), palette), image.NewPaletted(image.Rect(0, 0, 4, 2), palette)
	for i := range second.Pix {
		second.Pix[i] = 1
	}
	var b bytes.Buffer
	if err := gif.EncodeAll(&b, &gif.GIF{Image: []*image.Paletted{first, second}, Delay: []int{65535, 2}, LoopCount: loop}); err != nil {
		t.Fatal(err)
	}
	m := New(Options{Files: []string{"motion.gif", "still.gif"}, FilesFS: fstest.MapFS{"motion.gif": {Data: b.Bytes()}, "still.gif": {Data: b.Bytes()}}, Render: render, Page: 1})
	t.Cleanup(func() { m.Close() })
	m.Update(tea.WindowSizeMsg{Width: 40, Height: 12})
	m.generation = 1
	cmd := m.loaded(m.loader.Load(m.request(false)))
	if m.animation == nil {
		t.Fatalf("animation missing: %v", m.err)
	}
	// Drain only rendering commands: tests drive time explicitly below.
	m.animation.Pause(true)
	drainAnimation(t, m, cmd)
	// Tests leave playback paused; the event-loop tests exercise scheduling.
	return m
}

// Exercise scheduling through Bubble Tea itself, including nested transmission
// sequences and the presentation acknowledgement, instead of injecting ticks.
type animationObserver struct {
	*Model
	reached   bool
	flickered bool
	control   func(*Model) (tea.Cmd, bool)
}

func (m *animationObserver) Init() tea.Cmd {
	return tea.Batch(m.Model.Init(), func() tea.Msg { return tea.WindowSizeMsg{Width: 30, Height: 12} })
}
func (m *animationObserver) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	_, cmd := m.Model.Update(msg)
	if m.control != nil {
		next, done := m.control(m.Model)
		m.reached = m.reached || done
		return m, tea.Batch(cmd, next)
	}
	if m.animation != nil && m.kitty && glyphs(m.pictureView()) {
		m.flickered = true
	}
	if m.animation != nil && m.animation.player().Frame() >= 3 {
		m.reached = true
		return m, m.quit()
	}
	return m, cmd
}

func TestGIFFinitePlaybackRestartAndSeek(t *testing.T) {
	for _, render := range []string{"glyph", "kitty"} {
		t.Run(render, func(t *testing.T) {
			data, err := examples.Files.ReadFile("motion.gif")
			if err != nil {
				t.Fatal(err)
			}
			clip, err := gif.DecodeAll(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			clip.LoopCount = -1
			for i := range clip.Delay {
				clip.Delay[i] = 2
			}
			var b bytes.Buffer
			if err := gif.EncodeAll(&b, clip); err != nil {
				t.Fatal(err)
			}
			stage := 0
			model := &animationObserver{Model: New(Options{Files: []string{"once.gif"}, FilesFS: fstest.MapFS{"once.gif": {Data: b.Bytes()}}, Render: render, Page: 1})}
			defer model.Close()

			model.control = func(m *Model) (tea.Cmd, bool) {
				a := m.animation
				if a == nil {
					return nil, false
				}
				s := a.Status()
				switch {
				case stage == 0 && s.Finished:
					stage++
					return tea.Batch(a.setSpeed(1), m.toggleAnimation()), false
				case stage == 1 && a.player().Frame() == 1 && !s.Finished:
					stage++
					return m.toggleAnimation(), false
				case stage == 2 && s.Paused && !s.Busy && !s.Picture.Busy:
					stage++
					return m.seekAnimation(a.player().Frames()), false
				case stage == 3 && s.Paused && !s.Busy && !s.Picture.Busy && a.player().Frame() == a.player().Frames():
					stage++
					return m.toggleAnimation(), false
				case stage == 4 && s.Finished:
					if s.Rate != 2 {
						t.Error("restart or seek lost playback speed")
					}
					return m.quit(), true
				}
				return nil, false
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_, err = tea.NewProgram(model, tea.WithContext(ctx), tea.WithInput(nil), tea.WithOutput(io.Discard), tea.WithoutRenderer(), tea.WithoutSignalHandler()).Run()
			if err != nil || !model.reached {
				t.Fatalf("playback controls stalled at stage %d: %v", stage, err)
			}
		})
	}
}

func TestGIFLongTimerCancellationIsPrompt(t *testing.T) {
	for _, key := range []string{"space", ">", ".", "?"} {
		t.Run(key, func(t *testing.T) {
			m := animated(t, "glyph", 0)
			timer := m.toggleAnimation()
			done := make(chan tea.Msg, 1)
			go func() { done <- timer() }()
			_, cmd := m.Update(press(key))
			// The first frame lasts over ten minutes. Each control must cancel
			// that wait without waiting for its deadline or admitting its result.
			select {
			case msg := <-done:
				m.animation.Pause(true)
				_, next := m.Update(msg)
				drainAnimation(t, m, tea.Batch(cmd, next))
			case <-time.After(time.Second):
				t.Fatal("control left the old frame timer running")
			}
		})
	}
}
func TestGIFActualEventLoop(t *testing.T) {
	for _, render := range []string{"glyph", "kitty"} {
		for _, file := range []string{"motion.gif", "motion.webp"} {
			t.Run(render+"/"+file, func(t *testing.T) {

				model := &animationObserver{Model: New(Options{Files: []string{file}, FilesFS: examples.Files, Render: render, Page: 1})}
				defer model.Close()
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_, err := tea.NewProgram(model, tea.WithContext(ctx), tea.WithInput(nil), tea.WithOutput(io.Discard), tea.WithoutRenderer(), tea.WithoutSignalHandler()).Run()
				if err != nil || !model.reached || model.flickered {
					t.Fatalf("%s playback stalled: %v, note %s", render, err, model.note+fmt.Sprintf(" flickered=%v", model.flickered))
				}
			})
		}
	}
}

func TestGIFExportCapturesPresentedFullFrame(t *testing.T) {
	for _, render := range []string{"glyph", "kitty"} {
		t.Run(render, func(t *testing.T) {
			m := animated(t, render, 0)
			a := m.animation
			drainAnimation(t, m, m.seekAnimation(2))
			m.zoom = 1
			drainAnimation(t, m, m.refreshImage())
			var out saved
			m.opts.Save = out.save(t)
			export := m.export()
			drainAnimation(t, m, m.seekAnimation(1))
			export()
			if len(out.images) != 1 {
				t.Fatal("missing export")
			}
			if got := out.images[0]; got.Bounds().Size() != image.Pt(4, 2) {
				t.Fatal("export used cropped image")
			} else if r, g, _, _ := got.At(0, 0).RGBA(); r != 0 || g != 65535 {
				t.Fatal("export did not capture displayed green frame")
			}
			if a.player().Frame() != 1 {
				t.Fatal("export disturbed playback")
			}
		})
	}
}

func TestGIFExportRetainsFrontDuringSeek(t *testing.T) {
	m := animated(t, "kitty", 0)
	pending := m.seekAnimation(2)
	var out saved
	m.opts.Save = out.save(t)
	m.export()()
	if len(out.images) != 1 {
		t.Fatal("missing export")
	}
	if r, g, _, _ := out.images[0].At(0, 0).RGBA(); r != 65535 || g != 0 {
		t.Fatal("export used unpresented frame")
	}
	drainAnimation(t, m, pending)
}

func TestGIFControlsCropResizeAndRate(t *testing.T) {
	for _, render := range []string{"glyph", "kitty"} {
		t.Run(render, func(t *testing.T) {
			m := animated(t, render, 0)
			a := m.animation
			for _, tc := range []struct {
				key  string
				rate float64
			}{
				{"<", .5}, {"<", .25}, {"<", .25}, {">", .5}, {">", 1}, {">", 2}, {">", 4}, {">", 4}, {"backspace", 1},
			} {
				_, cmd := m.Update(press(tc.key))
				drainAnimation(t, m, cmd)
				if a.rate() != tc.rate || !a.Status().Paused {
					t.Fatalf("%s: %+v", tc.key, a.Status())
				}
				if !strings.Contains(m.detail(120, 40), fmt.Sprintf("%g× speed", tc.rate)) {
					t.Fatal("missing rate label")
				}
			}
			_, cmd := m.Update(press("+"))
			drainAnimation(t, m, cmd)
			if a.Crop().Size() != image.Pt(2, 1) {
				t.Fatalf("crop %v", a.Crop())
			}
			_, cmd = m.Update(tea.WindowSizeMsg{Width: 30, Height: 10})
			drainAnimation(t, m, cmd)
			_, cmd = m.Update(uv.CellSizeEvent{Width: 11, Height: 23})
			drainAnimation(t, m, cmd)
			if w, h := a.CellPixelSize(); w != 11 || h != 23 {
				t.Fatal("lost cell geometry")
			}
			if !a.Status().Paused || a.rate() != 1 || a.player().Frame() != 1 {
				t.Fatal("resize changed playback")
			}
			if render == "kitty" {
				assertAnimationFits(t, m)
			}
			load := m.load(true)
			_, cmd = m.Update(load())
			m.animation.Pause(true)
			drainAnimation(t, m, cmd)
			if m.animation.rate() != 1 {
				t.Fatal("reload retained old rate")
			}
		})
	}
}

func TestGIFHiddenPlaybackAndLateSeek(t *testing.T) {
	for _, action := range []string{"help", "browser", "menu", "reload", "file", "quit", "close"} {
		t.Run(action, func(t *testing.T) {
			m := animated(t, "kitty", 0)
			a := m.animation
			pending := m.seekAnimation(2)
			var cmd tea.Cmd
			switch action {
			case "help":
				_, cmd = m.Update(press("?"))
			case "browser":
				m.opts.FilesFS = nil
				cmd = tea.Batch(m.browseFrom(t.TempDir()), m.updateStandalone(nil))
			case "menu":
				cmd = m.openMenu(0)
			case "reload":
				m.load(true)
			case "file":
				m.switchFile(1)
			case "quit":
				cmd = m.quit()
			case "close":
				m.Close()
			}
			before := m.source
			drainAnimation(t, m, cmd)
			drainAnimation(t, m, pending)
			if m.source != before {
				t.Fatal("late seek replaced hidden document")
			}
			if action == "help" || action == "browser" {
				m.help = false
				m.screen = screenDocument
				drainAnimation(t, m, m.updateStandalone(nil))
				if m.animation != a || a.player().Frame() != 1 || !a.Visible() {
					t.Fatal("hidden playback did not restore")
				}
			} else if m.animation != nil {
				t.Fatal("controls survived leaving document")
			}
		})
	}
}

func TestGIFKeepPreservesPresentedFrame(t *testing.T) {
	m := animated(t, "kitty", 0)
	shown := m.playback.Status().Presented
	view := m.pictureView()
	pending := m.seekAnimation(2)
	m.opts.KeepScreen = true
	drainAnimation(t, m, m.quit())
	drainAnimation(t, m, pending)
	if m.pictureView() != view || m.playback.Status().Presented != shown {
		t.Fatal("Keep lost displayed frame")
	}
}

func TestGIFStopRetainsCommittedFrameDuringLoad(t *testing.T) {
	for _, render := range []string{"glyph", "kitty"} {
		t.Run(render, func(t *testing.T) {
			m := animated(t, render, 0)
			c := m.playback
			shown, view := c.Status().Presented, c.View()
			pending := m.seekAnimation(2)
			stopped := m.stopAnimation()
			m.loading = true
			drainAnimation(t, m, tea.Batch(stopped, pending))
			if c.Status().Presented != shown || c.View() != view || c.Status().Busy || c.Status().Picture.Busy {
				t.Fatal("late seek changed the retained frame while loading")
			}
			drainAnimation(t, m, m.clearStandalone())
			if !c.Closed() || c.View() != "" {
				t.Fatal("stopped playback could not be closed")
			}
		})
	}
}

func TestGIFKeepScreenSwitchRetransmits(t *testing.T) {
	for _, mainScreen := range []bool{false, true} {
		t.Run(fmt.Sprintf("main=%v", mainScreen), func(t *testing.T) {
			m := animated(t, "kitty", 0)
			m.opts.KeepScreen = mainScreen
			c, view := m.playback, m.playback.View()
			_, cmd := m.Update(press("Q"))
			var raw strings.Builder
			drainAnimation(t, m, cmd, func(msg tea.Msg) {
				if r, ok := msg.(tea.RawMsg); ok {
					fmt.Fprint(&raw, r.Msg)
				}
			})
			if got := strings.Contains(raw.String(), "a=T"); got == mainScreen {
				t.Fatalf("retransmission=%v, started on main screen=%v", got, mainScreen)
			}
			if m.View().AltScreen || c.View() != view {
				t.Fatal("quit did not retain the view on the main screen")
			}
			m.Close()
			if c.View() != view {
				t.Fatal("Close removed the kept view")
			}
		})
	}
}

func TestGIFGraphicsToggleAndResizeBurst(t *testing.T) {
	m := animated(t, "kitty", 0)
	a := m.animation
	var cmds []tea.Cmd
	for _, size := range []tea.WindowSizeMsg{{Width: 25, Height: 8}, {Width: 8, Height: 4}, {Width: 1, Height: 1}, {Width: 60, Height: 18}} {
		_, cmd := m.Update(size)
		cmds = append(cmds, cmd)
		generation := a.Status().Picture.Generation
		m.Update(size)
		if a.Status().Picture.Generation != generation {
			t.Fatal("unchanged size restarted preparation")
		}
		assertAnimationFits(t, m)
	}
	for i := len(cmds) - 1; i >= 0; i-- {
		drainAnimation(t, m, cmds[i])
	}
	assertAnimationFits(t, m)
	for _, key := range []string{"g", "g"} {
		_, cmd := m.Update(press(key))
		drainAnimation(t, m, cmd)
		if (a.Mode() == ntpicture.PictureKitty) != m.kitty {
			t.Fatal("graphics choice not forwarded")
		}
	}
}

func assertAnimationFits(t *testing.T, m *Model) {
	t.Helper()
	view := m.View().Content
	if strings.Count(view, "\n")+1 > m.height {
		t.Fatal("view exceeds window height")
	}
	for _, line := range strings.Split(view, "\n") {
		if ansi.StringWidth(line) > m.width {
			t.Fatal("view exceeds window width")
		}
	}
	if glyphs(m.pictureView()) {
		t.Fatal("Kitty resize exposed glyphs")
	}
}
