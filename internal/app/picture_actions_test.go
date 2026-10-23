//go:build !js

package app

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"

	tea "charm.land/bubbletea/v2"
	"nimbleterminal.dev/gloss/internal/document"
	"nimbleterminal.dev/ntvideo"
)

// labels are the names of the entries, in order.
func labels(entries []menuEntry) []string {
	var out []string
	for _, e := range entries {
		out = append(out, e.label)
	}
	return out
}

// pictureHosts gives m a host that opens files, a clipboard that keeps the
// last PNG, and a scratch folder in dir.
func pictureHosts(m *Model, dir string) (host *fakeOpenHost, copied *[]byte) {
	host, copied = &fakeOpenHost{}, new([]byte)
	m.opts.OpenWith = host
	m.opts.CopyImage = func(data []byte) error { *copied = data; return nil }
	m.opts.Scratch = func(name string, data []byte) (string, error) {
		path := filepath.Join(dir, filepath.Base(name))
		return path, os.WriteFile(path, data, 0o600)
	}
	m.opts.IsTemporary = func(path string) bool { return strings.HasPrefix(path, dir) }
	return host, copied
}

func TestPictureEntriesByHost(t *testing.T) {
	shown := func() image.Image { return image.NewRGBA(image.Rect(0, 0, 2, 2)) }
	scratch := func(string, []byte) (string, error) { return "", nil }
	tests := []struct {
		name  string
		setup func(*Options)
		item  pictureItem
		want  []string
	}{
		{"everything", func(o *Options) {
			o.OpenWith, o.CopyImage = &fakeOpenHost{}, func([]byte) error { return nil }
		}, pictureItem{name: "a.png", path: "a.png", shown: shown}, []string{"Open with…", "Edit with…", "Copy image", "Copy path"}},
		{"no open with", func(o *Options) {
			o.CopyImage = func([]byte) error { return nil }
		}, pictureItem{name: "a.png", path: "a.png", shown: shown}, []string{"Copy image", "Copy path"}},
		{"a temporary file is not edited", func(o *Options) {
			o.OpenWith = &fakeOpenHost{}
			o.IsTemporary = func(string) bool { return true }
		}, pictureItem{name: "a.png", path: "/tmp/a.png", shown: shown}, []string{"Open with…", "Copy path"}},
		{"standard input is not edited", func(o *Options) {
			o.OpenWith, o.Stdin = &fakeOpenHost{}, "/tmp/in"
		}, pictureItem{name: "stdin", path: "/tmp/in", shown: shown}, []string{"Open with…", "Copy path"}},
		{"no file, no way to write one", func(o *Options) {
			o.OpenWith = &fakeOpenHost{}
		}, pictureItem{name: "page", shown: shown}, nil},
		{"no file, scratch", func(o *Options) {
			o.OpenWith, o.Scratch = &fakeOpenHost{}, scratch
		}, pictureItem{name: "page", shown: shown}, []string{"Open with…"}},
		{"a video", func(o *Options) {
			o.OpenWith = &fakeOpenHost{}
		}, pictureItem{name: "v.mp4", path: "v.mp4", shown: shown, video: true}, []string{"Copy path", "Save frame as PNG"}},
		{"a video's frame", func(o *Options) {
			o.OpenWith, o.Scratch = &fakeOpenHost{}, scratch
		}, pictureItem{name: "v.mp4", path: "v.mp4", shown: shown, video: true}, []string{"Open with…", "Copy path", "Save frame as PNG"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := loaded(t, "photos/landscape.png")
			m.opts.OpenWith, m.opts.CopyImage, m.opts.Scratch, m.opts.IsTemporary, m.opts.Stdin = nil, nil, nil, nil, ""
			tt.setup(&m.opts)
			got := labels(m.pictureEntries(tt.item))
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("entries = %q, want %q", got, tt.want)
			}
			m.openActions("x", m.pictureEntries(tt.item), image.Pt(1, 1))
			if opened := m.layer == layerActions; opened != (len(tt.want) > 0) {
				t.Fatalf("menu open = %v with %d entries", opened, len(tt.want))
			}
		})
	}
}

func TestPictureMenuFromKey(t *testing.T) {
	m := loaded(t, "photos/landscape.png")
	_, copied := pictureHosts(m, t.TempDir())
	send(m, press("a"))
	if m.layer != layerActions || m.actions == nil || m.actions.title != "landscape.png" {
		t.Fatalf("a: layer=%v actions=%+v", m.layer, m.actions)
	}
	if got := labels(m.actions.entries); !reflect.DeepEqual(got, []string{"Open with…", "Edit with…", "Copy image", "Copy path"}) {
		t.Fatalf("entries = %q", got)
	}
	send(m, press("j"), press("j"), tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.layer != layerNone {
		t.Fatalf("the menu stayed open: layer=%v", m.layer)
	}
	img, err := png.Decode(bytes.NewReader(*copied))
	if err != nil {
		t.Fatalf("Copy image gave no PNG: %v", err)
	}
	if got := img.Bounds().Size(); got != image.Pt(64, 40) {
		t.Fatalf("copied %v, want the 64×40 picture", got)
	}
	if m.note != "Image copied" {
		t.Fatalf("note = %q", m.note)
	}
}

func TestCopyImageSaysWhyItFailed(t *testing.T) {
	m := loaded(t, "photos/landscape.png")
	pictureHosts(m, t.TempDir())
	m.opts.CopyImage = func([]byte) error { return os.ErrPermission }
	send(m, press("a"), press("j"), press("j"), tea.KeyPressMsg{Code: tea.KeyEnter})
	if !strings.Contains(m.note, "permission denied") {
		t.Fatalf("note = %q", m.note)
	}
}

func TestCopyPathNotesIt(t *testing.T) {
	m := loaded(t, "photos/landscape.png")
	pictureHosts(m, t.TempDir())
	send(m, press("a"), press("j"), press("j"), press("j"))
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil || m.note != "Path copied" {
		t.Fatalf("cmd=%v note=%q", cmd != nil, m.note)
	}
}

func TestShiftF10AndRightClick(t *testing.T) {
	shiftF10 := tea.KeyPressMsg{Code: tea.KeyF10, Mod: tea.ModShift}
	if shiftF10.String() != "shift+f10" {
		t.Fatalf("Shift-F10 is %q", shiftF10.String())
	}
	m := loaded(t, "photos/landscape.png")
	pictureHosts(m, t.TempDir())
	send(m, shiftF10)
	if m.layer != layerActions || m.actions.title != "landscape.png" {
		t.Fatalf("Shift-F10: layer=%v", m.layer)
	}

	m = loaded(t, "photos/landscape.png")
	pictureHosts(m, t.TempDir())
	send(m, tea.MouseClickMsg{X: 30, Y: 8, Button: tea.MouseLeft})
	if m.layer != layerNone {
		t.Fatalf("a left click opened a menu: layer=%v", m.layer)
	}
	send(m, tea.MouseClickMsg{X: 30, Y: 8, Button: tea.MouseRight})
	if m.layer != layerActions || m.actions.at != image.Pt(30, 8) {
		t.Fatalf("right-click: layer=%v actions=%+v", m.layer, m.actions)
	}

	// Past the body is the status bar and the keys line: nothing there.
	m = loaded(t, "photos/landscape.png")
	pictureHosts(m, t.TempDir())
	send(m, tea.MouseClickMsg{X: 30, Y: m.bodyHeight(), Button: tea.MouseRight})
	if m.layer != layerNone || m.note != "" {
		t.Fatalf("a right-click on the bar: layer=%v note=%q", m.layer, m.note)
	}
}

func TestVideoRightClickLeavesTheScrubber(t *testing.T) {
	m := loaded(t, "clips/a.mp4")
	pictureHosts(m, t.TempDir())
	m.kind, m.source = "video", nil
	m.video = ntvideo.New()
	last := m.bodyHeight() - 1
	send(m, tea.MouseClickMsg{X: 10, Y: last, Button: tea.MouseRight})
	if m.layer != layerNone {
		t.Fatalf("a right-click on the scrub bar opened a menu")
	}
	send(m, tea.MouseClickMsg{X: 10, Y: last - 1, Button: tea.MouseRight})
	if m.layer != layerActions {
		t.Fatalf("a right-click on the video opened no menu")
	}
	if got := labels(m.actions.entries); got[len(got)-1] != "Save frame as PNG" {
		t.Fatalf("entries = %q", got)
	}
}

func TestPDFPageOpensATemporary(t *testing.T) {
	dir := t.TempDir()
	m := loaded(t, "docs/report.pdf")
	host, _ := pictureHosts(m, dir)
	m.opts.CopyImage = nil
	send(m, document.Result{Generation: m.generation, Kind: "pdf", Page: 1, Pages: 3, Image: m.source})
	send(m, press("a"))
	if got := labels(m.actions.entries); !reflect.DeepEqual(got, []string{"Open with…"}) {
		t.Fatalf("a PDF page offers %q", got)
	}
	send(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	page := filepath.Join(dir, "report-page-1.png")
	if data, err := os.ReadFile(page); err != nil {
		t.Fatalf("the page was not written through Scratch: %v", err)
	} else if _, err := png.Decode(bytes.NewReader(data)); err != nil {
		t.Fatalf("the temporary is not a PNG: %v", err)
	}
	if m.layer != layerOpenWith || m.openWithPicker == nil {
		t.Fatalf("the chooser did not open: layer=%v note=%q", m.layer, m.note)
	}
	send(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if len(host.launched) != 1 || host.launched[0].Path != page || !host.launched[0].Temporary {
		t.Fatalf("launched %+v, want the temporary page", host.launched)
	}
}

var sgr = regexp.MustCompile(`\x1b\[[0-9;:]*m`)

func TestActionMenuTitleIsSafe(t *testing.T) {
	m := loaded(t, "photos/a\x1b]2;x\a.png")
	pictureHosts(m, t.TempDir())
	send(m, press("a"))
	if m.layer != layerActions {
		t.Fatal("no menu")
	}
	if view := sgr.ReplaceAllString(m.View().Content, ""); strings.Contains(view, "\x1b") || strings.Contains(view, "\a") {
		t.Fatalf("the file name put an escape in the view: %q", view)
	}
}

func TestVideoMenuHasSaveFrame(t *testing.T) {
	m := loaded(t, "photos/landscape.png")
	pictureHosts(m, t.TempDir())
	got := labels(m.pictureEntries(pictureItem{name: "v.mp4", path: "v.mp4", video: true, shown: func() image.Image { return m.source }}))
	if len(got) == 0 || got[len(got)-1] != "Save frame as PNG" {
		t.Fatalf("entries = %q", got)
	}
}

func TestActionMenuClosesWhenTheFileChanges(t *testing.T) {
	m := loaded(t, "photos/landscape.png", "photos/other.png")
	pictureHosts(m, t.TempDir())
	send(m, press("a"))
	if m.layer != layerActions {
		t.Fatal("no menu")
	}
	m.clearGraphics()
	if m.layer != layerNone || m.actions != nil {
		t.Fatalf("a stale menu: layer=%v", m.layer)
	}
}

func TestNoPictureMenuForMeshesOrText(t *testing.T) {
	m := loaded(t, "photos/landscape.png")
	pictureHosts(m, t.TempDir())
	m.source = nil
	if _, ok := m.viewPicture(); ok {
		t.Fatal("a view with no picture is offered as one")
	}
	send(m, press("a"), tea.MouseClickMsg{X: 5, Y: 5, Button: tea.MouseRight})
	if m.layer != layerNone {
		t.Fatalf("a menu opened: layer=%v", m.layer)
	}
}

// A picture with a menu takes the mouse, so a right-click reaches it: a
// still, an SVG, a PDF page and a Mermaid file. Where there is nothing to
// offer, the terminal keeps the mouse.
func TestPictureTakesTheMouse(t *testing.T) {
	for _, kind := range []string{"png", "svg", "pdf", "mermaid"} {
		m := loaded(t, "photos/landscape.png")
		pictureHosts(m, t.TempDir())
		send(m, document.Result{Generation: m.generation, Kind: kind, Page: 1, Pages: 1, Image: m.source})
		if got := m.View().MouseMode; got != tea.MouseModeCellMotion {
			t.Fatalf("%s: mouse mode %v", kind, got)
		}
	}
	m := loaded(t, "photos/landscape.png")
	m.source = nil
	if got := m.View().MouseMode; got != tea.MouseModeNone {
		t.Fatalf("no picture: mouse mode %v", got)
	}
	// In the browser with no clipboard and no apps, a picture has no entries.
	m = loaded(t, "photos/landscape.png")
	m.opts.FilesFS = fstest.MapFS{}
	if got := m.View().MouseMode; got != tea.MouseModeNone {
		t.Fatalf("nothing to offer: mouse mode %v", got)
	}
}

// With the mouse taken, the wheel does what the arrow keys did when the
// terminal turned it into them.
func TestPictureWheelIsTheArrowKeys(t *testing.T) {
	keyed, wheeled := loaded(t, "photos/landscape.png"), loaded(t, "photos/landscape.png")
	pictureHosts(keyed, t.TempDir())
	pictureHosts(wheeled, t.TempDir())
	keyed.zoom, wheeled.zoom = 2, 2
	send(keyed, tea.KeyPressMsg{Code: tea.KeyDown}, tea.KeyPressMsg{Code: tea.KeyDown}, tea.KeyPressMsg{Code: tea.KeyUp})
	send(wheeled, tea.MouseWheelMsg{X: 5, Y: 5, Button: tea.MouseWheelDown}, tea.MouseWheelMsg{X: 5, Y: 5, Button: tea.MouseWheelDown}, tea.MouseWheelMsg{X: 5, Y: 5, Button: tea.MouseWheelUp})
	if keyed.panY == 0 || wheeled.panY != keyed.panY {
		t.Fatalf("wheel panned to %v, keys to %v", wheeled.panY, keyed.panY)
	}
}

// A video opens a PNG of the frame on screen, never the video, and is not
// edited; Copy path is the video's.
func TestVideoOpensItsFrame(t *testing.T) {
	dir := t.TempDir()
	m := loaded(t, "photos/landscape.png")
	host, _ := pictureHosts(m, dir)
	p := pictureItem{name: "v.mp4", path: "clips/v.mp4", video: true, shown: func() image.Image { return m.source }}
	entries := m.pictureEntries(p)
	if got := labels(entries); !reflect.DeepEqual(got, []string{"Open with…", "Copy image", "Copy path", "Save frame as PNG"}) {
		t.Fatalf("entries = %q", got)
	}
	m.openActions(p.name, entries, image.Pt(1, 1))
	send(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	frame := filepath.Join(dir, "v.png")
	if _, err := os.Stat(frame); err != nil {
		t.Fatalf("no frame written: %v", err)
	}
	send(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if len(host.launched) != 1 || host.launched[0].Path != frame {
		t.Fatalf("launched %+v, want the frame", host.launched)
	}
}

// A whole-view picture in the browser has no path worth copying.
func TestNoCopyPathInTheBrowser(t *testing.T) {
	m := loaded(t, "photos/landscape.png")
	m.opts.FilesFS = fstest.MapFS{}
	m.opts.CopyImage = func([]byte) error { return nil }
	p, ok := m.viewPicture()
	if !ok {
		t.Fatal("no picture")
	}
	if got := labels(m.pictureEntries(p)); !reflect.DeepEqual(got, []string{"Copy image"}) {
		t.Fatalf("entries = %q", got)
	}
}

// Nothing is offered while a prompt takes the keys, or while loading.
func TestNoItemsUnderPrompts(t *testing.T) {
	right := tea.MouseClickMsg{X: 5, Y: 5, Button: tea.MouseRight}
	for name, set := range map[string]func(*Model){
		"go to line": func(m *Model) { m.lineInput = &lineInput{} },
		"search":     func(m *Model) { m.search = &textSearch{editing: true} },
		"loading":    func(m *Model) { m.loading = true },
	} {
		m := loaded(t, "photos/landscape.png")
		pictureHosts(m, t.TempDir())
		set(m)
		if m.itemsHere() {
			t.Fatalf("%s: items here", name)
		}
		send(m, right)
		if m.layer == layerActions {
			t.Fatalf("%s: a menu opened", name)
		}
	}
}

// The keys line says a only where a menu would have entries.
func TestActionsHintOnlyWithEntries(t *testing.T) {
	m := loaded(t, "photos/landscape.png")
	pictureHosts(m, t.TempDir())
	if !strings.Contains(m.hints(), "a actions") {
		t.Fatalf("hints lack a actions: %s", m.hints())
	}
	m = loaded(t, "photos/landscape.png")
	m.opts.FilesFS = fstest.MapFS{}
	if strings.Contains(m.hints(), "a actions") {
		t.Fatalf("hints offer an empty menu: %s", m.hints())
	}
}

// From the keyboard, the menu of a picture filling the view opens in its
// middle, not in a corner.
func TestPictureMenuFromKeysOpensMidPicture(t *testing.T) {
	m := loaded(t, "photos/landscape.png")
	pictureHosts(m, t.TempDir())
	send(m, press("a"))
	area := m.pictureArea()
	if want := image.Pt(area.Min.X+area.Dx()/2, area.Min.Y+area.Dy()/2); m.layer != layerActions || m.actions.at != want {
		t.Fatalf("layer %v, at %v, want %v", m.layer, m.actions, want)
	}
}
