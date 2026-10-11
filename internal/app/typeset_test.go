package app

import (
	"fmt"
	"image"
	"strings"
	"testing"
	"testing/fstest"

	tea "charm.land/bubbletea/v2"
	"github.com/NimbleMarkets/ntcharts/v2/picture"
	"nimbleterminal.dev/gloss/internal/document"
)

func TestTypesetViewer(t *testing.T) {
	files := fstest.MapFS{"page.md": &fstest.MapFile{Data: []byte("# A typeset page\n\n" + strings.Repeat("Café and a paragraph.\n\n", 100))}}
	m := New(Options{Files: []string{"page.md"}, FilesFS: files, MarkdownMode: "typeset", Render: "glyph", DPI: 150, Page: 1})
	t.Cleanup(func() { _ = m.Close() })
	m.width, m.height = 80, 30
	drainAnimation(t, m, m.load(false))
	if m.typeset == nil || m.source == nil || m.kind != "markdown" {
		t.Fatal("not direct Markdown")
	}
	cmd, _ := m.documentKey("space")
	drainAnimation(t, m, cmd)
	if m.typesetTop == 0 {
		t.Fatal("did not scroll")
	}
	_, cmd = m.Update(tea.WindowSizeMsg{Width: 45, Height: 20})
	drainAnimation(t, m, cmd)
	if m.typesetWidth != m.typesetMeasure() {
		t.Fatal("resize did not reflow")
	}
	cmd, _ = m.documentKey("s")
	drainAnimation(t, m, cmd)
	if m.markdown == nil || !m.markdown.raw {
		t.Fatal("source toggle")
	}
	_, cmd = m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	drainAnimation(t, m, cmd)
	if m.markdown == nil || !m.markdown.raw {
		t.Fatal("resize closed source")
	}
	cmd, _ = m.documentKey("s")
	drainAnimation(t, m, cmd)
	if m.markdown != nil {
		t.Fatal("did not return to typeset")
	}
	cmd, _ = m.documentKey("/")
	drainAnimation(t, m, cmd)
	if m.markdown == nil || !m.markdown.raw || m.search == nil {
		t.Fatal("search should open source")
	}
}

func TestTypesetWaitsForInitialSize(t *testing.T) {
	m := New(Options{Files: []string{"x.md"}, FilesFS: fstest.MapFS{"x.md": &fstest.MapFile{Data: []byte("# First page")}}, MarkdownMode: "typeset", Render: "glyph", DPI: 72, Page: 1})
	t.Cleanup(func() { _ = m.Close() })
	if cmd := m.load(false); cmd != nil || !m.typesetWaitSize || m.generation != 0 {
		t.Fatal("typeset before viewport size was known")
	}
	_, cmd := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	drainAnimation(t, m, cmd)
	if m.typeset == nil || m.typesetWidth != m.typesetMeasure() || m.generation != 1 {
		t.Fatal("initial load did not use viewport width")
	}
}

func TestTypesetToggle(t *testing.T) {
	m := New(Options{Files: []string{"page.md"}, FilesFS: fstest.MapFS{"page.md": &fstest.MapFile{Data: []byte("# Café\n\nA paragraph.")}}, Render: "glyph", DPI: 72, Page: 1})
	t.Cleanup(func() { _ = m.Close() })
	m.width, m.height = 80, 24
	drainAnimation(t, m, m.load(false))
	if m.kind != "markdown" || m.markdown == nil {
		t.Fatal("expected default text Markdown")
	}
	queryDocument(t, m, "paragraph")
	cmd, handled := m.documentKey("t")
	if !handled || !m.loading || m.search != nil {
		t.Fatal("toggle must load asynchronously and clear search")
	}
	generation := m.generation
	if repeated, _ := m.documentKey("t"); repeated != nil || m.generation != generation {
		t.Fatal("repeated toggle during loading started another load")
	}
	drainAnimation(t, m, cmd)
	if m.typeset == nil || m.markdown != nil || m.source == nil {
		t.Fatal("t did not enter typeset")
	}
	cmd, _ = m.documentKey("s")
	drainAnimation(t, m, cmd)
	cmd, _ = m.documentKey("t")
	drainAnimation(t, m, cmd)
	if m.typeset != nil || m.markdown == nil || m.markdown.raw || m.source != nil || m.page != 1 {
		t.Fatal("t from source did not return to rendered text")
	}
	// A late resize response from the old PDF must not restore it.
	drainAnimation(t, m, m.typesetReflowed(typesetReflowResult{owner: m.kittyID, result: document.Result{Generation: generation, Typeset: &document.Markdown{}}}))
	if m.typeset != nil || m.markdown == nil {
		t.Fatal("obsolete reflow restored typeset")
	}
	m.kind = "html"
	if cmd, _ := m.documentKey("t"); cmd != nil || m.opts.MarkdownMode != "text" {
		t.Fatal("converted HTML should not toggle Markdown backend")
	}
}

func TestTypesetToggleFailureCanReturnToText(t *testing.T) {
	m := New(Options{Files: []string{"page.md"}, Render: "glyph", Page: 1})
	t.Cleanup(func() { _ = m.Close() })
	m.width, m.height, m.kind = 80, 24, "markdown"
	cmd, _ := m.documentKey("t")
	// Exercise the ordinary loader-error path without compiling a PDF.
	_ = cmd
	drainAnimation(t, m, m.loaded(document.Result{Generation: m.generation, Err: fmt.Errorf("typeset budget exceeded")}))
	cmd, handled := m.documentKey("t")
	if !handled || cmd == nil || m.opts.MarkdownMode != "text" {
		t.Fatal("cannot leave typeset after an error")
	}
}

func TestTypesetCoalescesAndRejectsObsoleteDraw(t *testing.T) {
	m := New(Options{Files: []string{"x.md"}, FilesFS: fstest.MapFS{"x.md": &fstest.MapFile{Data: []byte(strings.Repeat("A paragraph.\n\n", 100))}}, MarkdownMode: "typeset", Render: "glyph", DPI: 150, Page: 1})
	t.Cleanup(func() { _ = m.Close() })
	m.width, m.height = 80, 24
	drainAnimation(t, m, m.load(false))
	old := m.source
	m.typesetTop = 100
	queued := m.queueTypesetReflow()
	msg := queued().(typesetResize)
	job := m.reflowTypeset(msg)
	if job == nil {
		t.Fatal("no asynchronous draw")
	}
	m.typesetTop = 400
	_ = m.queueTypesetReflow()
	result := job().(typesetReflowResult)
	cmd := m.typesetReflowed(result)
	if m.source != old || m.loading {
		t.Fatal("obsolete draw replaced/hid old view")
	}
	drainAnimation(t, m, cmd)
	if m.typesetTop != 400 || m.source == old {
		t.Fatal("latest scroll was not drawn")
	}
	cmd, _ = m.documentKey("t")
	drainAnimation(t, m, cmd)
	_ = m.typesetReflowed(result)
	if m.typeset != nil || m.markdown == nil {
		t.Fatal("late draw restored typeset after toggle")
	}
}

func TestTextTypesetToggle(t *testing.T) {
	for _, path := range []string{"note.txt", "main.go", "data.json"} {
		t.Run(path, func(t *testing.T) {
			text := "# This *stays* literal\n"
			if path == "main.go" {
				text = "package main\nfunc main() {}\n"
			}
			if path == "data.json" {
				text = "{\"value\": 42}"
			}
			m := New(Options{Files: []string{path}, FilesFS: fstest.MapFS{path: {Data: []byte(text)}}, Render: "glyph", Page: 1})
			defer m.Close()
			m.width, m.height = 80, 24
			drainAnimation(t, m, m.load(false))
			kind := m.kind
			if m.markdown == nil || m.typeset != nil {
				t.Fatal("not normal text by default")
			}
			cmd, ok := m.documentKey("t")
			if !ok {
				t.Fatal("t not handled")
			}
			drainAnimation(t, m, cmd)
			if m.source == nil || m.typeset == nil || m.kind != kind {
				t.Fatal("did not typeset text")
			}
			cmd, _ = m.documentKey("s")
			drainAnimation(t, m, cmd)
			if m.markdown == nil || m.markdown.raw {
				t.Fatal("text view exposed generated fences")
			}
			cmd, _ = m.documentKey("s")
			drainAnimation(t, m, cmd)
			cmd, _ = m.documentKey("/")
			drainAnimation(t, m, cmd)
			if m.search == nil || m.markdown == nil || m.markdown.raw {
				t.Fatal("search did not open text")
			}
			m.clearSearch()
			cmd, _ = m.documentKey("t")
			drainAnimation(t, m, cmd)
			if m.source != nil || m.typeset != nil || m.markdown == nil || m.kind != kind {
				t.Fatal("did not restore normal text")
			}
		})
	}
}

// With Kitty graphics the typeset raster is the picture's own cell area in
// device pixels, so the picture component copies it instead of resampling.
// Glyph rendering keeps the layout-scale raster.
func TestTypesetRastersToKittyCellArea(t *testing.T) {
	t.Cleanup(func() { picture.ForceKittyCapability(picture.KittyCapabilityUnknown) })
	files := fstest.MapFS{"page.md": &fstest.MapFile{Data: []byte("# A typeset page\n\n" + strings.Repeat("Café and a paragraph.\n\n", 100))}}
	m := New(Options{Files: []string{"page.md"}, FilesFS: files, MarkdownMode: "typeset", Render: "kitty", DPI: 150, Page: 1})
	t.Cleanup(func() { _ = m.Close() })
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	drainAnimation(t, m, m.load(false))
	if m.typeset == nil || m.source == nil || m.pic.Mode() != picture.PictureKitty {
		t.Fatal("not typeset Markdown under Kitty")
	}
	cw, ch := m.pic.CellPixelSize()
	want := image.Pt(m.width*cw, m.bodyHeight()*ch)
	if got := m.source.Bounds().Size(); got != want || m.typesetShown == nil || m.typesetShown.Pixels != want {
		t.Fatalf("raster %v, want the cell area %v (shown %+v)", got, want, m.typesetShown)
	}
	if !m.typesetMatches() {
		t.Fatal("fresh raster should match the viewport")
	}
	_, cmd := m.Update(tea.WindowSizeMsg{Width: 60, Height: 24})
	drainAnimation(t, m, cmd)
	cw, ch = m.pic.CellPixelSize()
	if want = image.Pt(m.width*cw, m.bodyHeight()*ch); m.source.Bounds().Size() != want {
		t.Fatalf("after resize raster %v, want %v", m.source.Bounds().Size(), want)
	}
	glyph := New(Options{Files: []string{"page.md"}, FilesFS: files, MarkdownMode: "typeset", Render: "glyph", DPI: 150, Page: 1})
	t.Cleanup(func() { _ = glyph.Close() })
	glyph.width, glyph.height = 80, 30
	drainAnimation(t, glyph, glyph.load(false))
	if glyph.typesetShown == nil || glyph.typesetShown.Pixels != (image.Point{}) {
		t.Fatalf("glyph rendering asked for device pixels: %+v", glyph.typesetShown)
	}
}
