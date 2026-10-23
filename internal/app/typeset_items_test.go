package app

import (
	"bytes"
	"errors"
	"image"
	"image/png"
	"strings"
	"testing"
	"testing/fstest"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"nimbleterminal.dev/gloss/internal/document"
	"nimbleterminal.dev/nttypeset"
)

func pngOf(t *testing.T, w, h int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// typesetItemsModel is a typeset Markdown page with a picture of w×h pixels.
func typesetItemsModel(t *testing.T, source string, w, h int, opts Options) *Model {
	t.Helper()
	opts.Files = []string{"doc.md"}
	opts.FilesFS = fstest.MapFS{"doc.md": {Data: []byte(source)}, "x.png": {Data: pngOf(t, w, h)}}
	opts.MarkdownMode, opts.Render, opts.DPI, opts.Page = "typeset", "glyph", 150, 1
	m := New(opts)
	t.Cleanup(func() { _ = m.Close() })
	m.width, m.height = 80, 30
	drainAnimation(t, m, m.load(false))
	if m.typeset == nil || m.source == nil || m.markdown != nil {
		t.Fatalf("not a typeset page: %v", m.err)
	}
	return m
}

func pressDrain(t *testing.T, m *Model, k tea.KeyPressMsg) {
	t.Helper()
	_, cmd := m.Update(k)
	drainAnimation(t, m, cmd)
}

// focused is the index of the item the focus is on, -1 for none.
func focused(m *Model) int {
	for i, it := range m.typesetItems {
		if rectsEqual(it.Rects, m.typesetFocus) {
			return i
		}
	}
	return -1
}

func TestTypesetTabAndMenu(t *testing.T) {
	m := typesetItemsModel(t, "![a](x.png)\n\n[l](https://e.org/)\n\n|h|i|\n|-|-|\n|1|2|\n\n[^1]\n\n[^1]: note", 8, 4, Options{})
	if len(m.typesetItems) != 3 || !m.tabItems() {
		t.Fatalf("items %+v, tab %v", m.typesetItems, m.tabItems())
	}
	plain := m.source
	pressDrain(t, m, tab)
	if focused(m) != 0 || !rectsEqual(m.typesetFocusShown, m.typesetFocus) {
		t.Fatalf("focus %v on %d, shown %v", m.typesetFocus, focused(m), m.typesetFocusShown)
	}
	if m.source == plain {
		t.Fatal("the page was not drawn again with the focus")
	}
	pressDrain(t, m, tab)
	if focused(m) != 1 {
		t.Fatalf("second Tab on %d", focused(m))
	}
	pressDrain(t, m, tea.KeyPressMsg{Code: 'a', Text: "a"})
	if m.layer != layerActions || strings.Join(entryLabels(m.actions.entries), ",") != "Copy address" {
		t.Fatalf("link menu: layer %v %+v", m.layer, m.actions)
	}
	pressDrain(t, m, tea.KeyPressMsg{Code: tea.KeyEscape})
	pressDrain(t, m, shiftTab)
	if focused(m) != 0 {
		t.Fatalf("Shift-Tab on %d", focused(m))
	}
	pressDrain(t, m, shiftTab)
	if focused(m) != 2 {
		t.Fatalf("Shift-Tab from the first on %d", focused(m))
	}
	pressDrain(t, m, tea.KeyPressMsg{Code: 'a', Text: "a"})
	if m.layer != layerActions || strings.Join(entryLabels(m.actions.entries), ",") != "Copy as TSV,Copy as Markdown" {
		t.Fatalf("table menu: %+v", m.actions)
	}
}

// Tab goes past the part of the page laid so far, laying more of it.
func TestTypesetTabLaysMore(t *testing.T) {
	source := strings.Repeat("A paragraph of words to fill the page.\n\n", 120) + "[far](https://e.org/)\n"
	m := typesetItemsModel(t, source, 8, 4, Options{})
	if len(m.typesetItems) != 0 {
		t.Fatalf("items laid early: %+v", m.typesetItems)
	}
	pressDrain(t, m, tab)
	if focused(m) != 0 || m.typesetItems[0].URL != "https://e.org/" {
		t.Fatalf("focus %d on %+v", focused(m), m.typesetItems)
	}
	if r := m.typesetFocus[0]; r.Min.Y < m.typesetShown.Top || r.Max.Y > m.typesetShown.Top+m.typesetShown.Height {
		t.Fatalf("focus %v off the view %+v", r, m.typesetShown)
	}
}

// itemCell is the body cell nearest the middle of an item's first rect.
func itemCell(m *Model, it document.TypesetItem) image.Point {
	shown, b := m.typesetShown, m.source.Bounds()
	view := nttypeset.View{Top: shown.Top, Left: shown.Left, Scale: float64(b.Dx()) / float64(shown.Width)}
	r := view.Project(it.Rects[0]).Add(b.Min)
	mid := image.Pt((r.Min.X+r.Max.X)/2, (r.Min.Y+r.Max.Y)/2)
	best, far := image.Pt(-1, -1), 1<<30
	for y := range m.bodyHeight() {
		for x := range m.width {
			p, ok := m.typesetPixel(x, y, false)
			if d := p.Sub(mid); ok && d.X*d.X+d.Y*d.Y < far {
				best, far = image.Pt(x, y), d.X*d.X+d.Y*d.Y
			}
		}
	}
	return best
}

func TestTypesetRightClickPicture(t *testing.T) {
	files := fstest.MapFS{"doc.md": {Data: []byte("Some words.\n\n![a](x.png)\n\nMore words.")}, "x.png": {Data: pngOf(t, 2000, 1000)}}
	m := New(Options{Files: []string{"doc.md"}, FilesFS: files, MarkdownMode: "typeset", Render: "glyph", DPI: 150, Page: 1, CopyImage: func([]byte) error { return nil }})
	t.Cleanup(func() { _ = m.Close() })
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m.terminal.Update(uv.CellSizeEvent{Width: 10, Height: 22})
	drainAnimation(t, m, m.load(false))
	if len(m.typesetItems) != 1 || m.typesetItems[0].Kind != document.TypesetPicture {
		t.Fatalf("items %+v", m.typesetItems)
	}
	at := itemCell(m, m.typesetItems[0])
	m.Update(tea.MouseClickMsg{X: at.X, Y: at.Y, Button: tea.MouseRight})
	if m.layer != layerActions || len(m.actions.entries) == 0 || m.actions.entries[0].label != "Copy image" {
		t.Fatalf("picture menu at %v: layer %v %+v", at, m.layer, m.actions)
	}
	// What it copies is the picture itself, at its own size, not the
	// screen's cut of it.
	if p, ok := m.typesetItemAt(at.X, at.Y); !ok || p.picture.shown == nil || p.picture.shown().Bounds().Size() != image.Pt(2000, 1000) {
		t.Fatalf("picture item %+v %v", p, ok)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m.note = ""
	m.Update(tea.MouseClickMsg{X: 0, Y: 0, Button: tea.MouseRight})
	if m.layer == layerActions || m.note != "" {
		t.Fatalf("right-click on the margin: layer %v note %q", m.layer, m.note)
	}
}

// On a long page, Tab past what is laid lays on to the next item without
// drawing every screen on the way; any other key gives the Tab up.
func TestTypesetTabLaysAheadOnALongPage(t *testing.T) {
	source := strings.Repeat("A paragraph of words to fill the page.\n\n", 1500) + "[far](https://e.org/)\n"
	m := typesetItemsModel(t, source, 8, 4, Options{})
	before := m.generation
	pressDrain(t, m, tab)
	if focused(m) != 0 || m.typesetItems[0].URL != "https://e.org/" {
		t.Fatalf("focus %d on %+v", focused(m), m.typesetItems)
	}
	if renders := m.generation - before; renders > 3 {
		t.Fatalf("%d pages drawn to reach the link", renders)
	}

	m = typesetItemsModel(t, source, 8, 4, Options{})
	_, tabbed := m.Update(tab)
	_, moved := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	drainAnimation(t, m, tea.Batch(tabbed, moved))
	if m.typesetTabPending != 0 || m.typesetFocus != nil || m.typesetShown.Top > 1000 {
		t.Fatalf("the Tab went on: pending %d focus %v top %d", m.typesetTabPending, m.typesetFocus, m.typesetShown.Top)
	}
}

// A failed page view gives up the place it was going to.
func TestTypesetFailureForgetsTheGoal(t *testing.T) {
	m := typesetItemsModel(t, "words", 8, 4, Options{})
	m.typesetGoTop, m.typesetTabPending = 500, 1
	m.typesetReflowed(typesetReflowResult{m.ownerID, m.typesetRevision, document.Result{Generation: m.generation, Err: errors.New("boom")}})
	if m.typesetGoTop != -1 || m.typesetTabPending != 0 {
		t.Fatalf("go top %d, pending %d", m.typesetGoTop, m.typesetTabPending)
	}
}
