package app

import (
	"fmt"
	"image"
	"nimbleterminal.dev/nt3d"
	widget "nimbleterminal.dev/nt3d/widget"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/ansi/kitty"
	"nimbleterminal.dev/gloss/internal/document"
)

func TestPreviewKittyPresentation(t *testing.T) {
	m := New(Options{Files: []string{"a.png"}, Menu: true, Preview: true, Render: "kitty"})
	t.Cleanup(func() { m.Close() })
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	p := m.preview
	_, cmd := m.Update(previewResult{owner: p.kittyID, result: document.Result{
		Generation: p.generation, Kind: "png", Image: image.NewRGBA(image.Rect(0, 0, 32, 32)),
	}})
	drainAnimation(t, m, cmd)
	if !p.hasPictureFront() || !strings.ContainsRune(m.View().Content, kitty.Placeholder) {
		t.Fatal("preview did not present its Kitty image through the parent event loop")
	}
	if m.hasPictureFront() {
		t.Fatal("preview presentation changed the parent image")
	}
	_, cmd = m.Update(tea.WindowSizeMsg{Width: 90, Height: 25})
	drainAnimation(t, m, cmd)
	if p.pictureBusy() || !strings.ContainsRune(m.View().Content, kitty.Placeholder) {
		t.Fatal("resized preview did not finish presentation")
	}
}

func TestMenuSelectionCancelAndOpen(t *testing.T) {
	m := New(Options{Files: []string{"a.pdf", "b.svg", "c.stl"}, Render: "glyph", Page: 1, DPI: 72})
	defer m.Close()
	m.page, m.zoom = 4, 2
	m.Update(press("m"))
	m.menuKey("down")
	if !m.listing() || m.selection != 1 || m.index != 0 || m.page != 4 {
		t.Fatal("browsing changed active file")
	}
	m.closeMenu(false)
	if m.index != 0 || m.page != 4 || m.zoom != 2 {
		t.Fatal("cancel lost active view")
	}
	m.Update(press("m"))
	m.menuKey("end")
	m.menuKey("enter")
	if m.listing() || m.index != 2 || m.page != 1 || m.zoom != 0 || !m.loading {
		t.Fatal("selection did not open")
	}
}

func TestMenuPreviewIsolation(t *testing.T) {
	m := New(Options{Files: []string{"a.pdf", "b.svg"}, Render: "glyph", Menu: true, Preview: true, Page: 1, DPI: 150})
	defer m.Close()
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	if m.preview == nil || m.preview.kittyID == m.kittyID {
		t.Fatal("preview is not independent")
	}
	owner := m.preview.kittyID
	m.menuKey("down")
	m.Update(previewResult{owner: owner, result: document.Result{Generation: m.preview.generation, Kind: "png"}})
	if m.preview.kind != "" {
		t.Fatal("retired preview result accepted")
	}
	owner = m.preview.kittyID
	m.Update(previewResult{owner: owner, result: document.Result{Generation: m.preview.generation, Kind: "svg", Page: 1, Pages: 1}})
	if m.kind != "" || m.index != 0 || m.preview.kind != "svg" {
		t.Fatal("preview result affected active document")
	}
	m.menuKey("v")
	m.Update(previewResult{owner: owner, result: document.Result{Generation: 999, Kind: "stl"}})
	if m.preview != nil || m.kind != "" {
		t.Fatal("stale preview was revived")
	}
}

func TestMenuScrollAndResize(t *testing.T) {
	files := make([]string, 100)
	for i := range files {
		files[i] = fmt.Sprintf("folder/file-%03d.svg", i)
	}
	m := New(Options{Files: files, Menu: true, Render: "glyph", Page: 1})
	defer m.Close()
	for _, sz := range []tea.WindowSizeMsg{{Width: 100, Height: 24}, {Width: 20, Height: 6}, {Width: 2, Height: 3}} {
		m.Update(sz)
		m.menuKey("end")
		view := m.View().Content
		lines := strings.Split(view, "\n")
		if len(lines) > sz.Height {
			t.Fatal("menu overflows height")
		}
		for _, line := range lines {
			if ansi.StringWidth(line) > sz.Width {
				t.Fatal("menu overflows width")
			}
		}
		if sz.Width >= 20 && !strings.Contains(m.menuView(), "100") {
			t.Fatal("selection not scrolled into view")
		}
	}
}

func TestPreviewMouseCoordinatesAndCapture(t *testing.T) {
	m := New(Options{Files: []string{"a.stl"}, Menu: true, Preview: true, Render: "glyph"})
	defer m.Close()
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m.preview.loading = false
	m.preview.chart = widget.New(nil, m.previewWidth(), m.bodyHeight(), widget.WithRenderMode(nt3d.Software))
	if m.View().MouseMode != tea.MouseModeAllMotion {
		t.Fatal("preview does not request mouse events")
	}
	before := m.preview.chart.Camera()
	left := m.width - m.previewWidth()
	for i := 0; i < 100; i++ {
		m.Update(tea.MouseClickMsg{X: left + 20, Y: 10, Button: tea.MouseLeft})
		m.Update(tea.MouseMotionMsg{X: left + 23, Y: 12, Button: tea.MouseLeft})
		m.Update(tea.MouseReleaseMsg{X: 1, Y: 12, Button: tea.MouseLeft})
		if m.preview.chart.Camera().Beta != before.Beta {
			break
		}
		time.Sleep(time.Millisecond)
	}
	after := m.preview.chart.Camera()
	if after.Beta != before.Beta-6 || after.Alpha != before.Alpha+4 {
		t.Fatalf("preview orbit failed: %+v -> %+v", before, after)
	}
	m.Update(tea.MouseMotionMsg{X: left + 30, Y: 12, Button: tea.MouseLeft})
	if m.preview.chart.Camera() != after || m.previewDrag {
		t.Fatal("outside release did not end drag")
	}
	m.Update(tea.MouseClickMsg{X: 5, Y: 10, Button: tea.MouseLeft})
	m.Update(tea.MouseMotionMsg{X: left + 25, Y: 10, Button: tea.MouseLeft})
	if m.preview.chart.Camera() != after {
		t.Fatal("list click reached chart")
	}
	m.Update(tea.MouseWheelMsg{X: left + 20, Y: 10, Button: tea.MouseWheelUp})
	if m.preview.chart.Camera().Distance == after.Distance {
		t.Fatal("preview wheel did not zoom")
	}
}

func TestPreviewDisposalDeletesAllGraphics(t *testing.T) {
	for _, action := range []string{"switch", "hide", "close", "narrow"} {
		t.Run(action, func(t *testing.T) {
			m := New(Options{Files: []string{"a.stl", "b.png"}, Menu: true, Preview: true, Render: "kitty"})
			defer m.Close()
			m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
			old := m.preview
			mesh, err := document.ParseSTL([]byte(facet))
			if err != nil {
				t.Fatal(err)
			}
			old.opts.Render3D = "software"
			drainAnimation(t, m, old.showMesh(document.Result{Mesh: mesh}, nil))
			if !old.meshKitty() || old.chart.PictureStatus().Presented == 0 {
				t.Fatal("mesh was not presented in Kitty mode")
			}

			old.markdown = newMarkdownView(&document.Markdown{Source: []byte("![picture](pic.png)"), Images: []document.MarkdownImage{{Destination: "pic.png", Image: image.NewRGBA(image.Rect(0, 0, 2, 2))}}}, old.terminal)
			old.loading = false
			var inlineID string
			drainAnimation(t, old, tea.Batch(old.markdown.setKitty(true), old.layoutMarkdown(), old.markdown.syncPictures(true)), func(msg tea.Msg) {
				if raw, ok := msg.(tea.RawMsg); ok {
					text := fmt.Sprint(raw.Msg)
					if strings.Contains(text, "a=T") {
						_, rest, found := strings.Cut(text, "i=")
						if found {
							inlineID = strings.SplitN(rest, ",", 2)[0]
						}
					}
				}
			})
			if inlineID == "" {
				t.Fatal("inline preview was not transmitted")
			}

			var cmd tea.Cmd
			switch action {
			case "switch":
				cmd = m.menuKey("down")
			case "hide":
				cmd = m.menuKey("v")
			case "close":
				cmd = m.closeMenu(false)
			case "narrow":
				_, cmd = m.Update(tea.WindowSizeMsg{Width: 40, Height: 30})
			}
			// Sequence commands contain a slice of commands, just like BatchMsg.
			output := collectRaw(cmd)
			deleted := map[string]bool{}
			for _, command := range strings.Split(output, "a=d,d=I,i=")[1:] {
				id := strings.SplitN(command, ",", 2)[0]
				deleted[id] = true
			}
			for _, id := range []string{inlineID} {
				if !deleted[id] {
					t.Fatalf("%s: missing delete for %s in %q", action, id, output)
				}
			}
			if len(deleted) < 2 {
				t.Fatalf("%s: missing mesh image delete in %q", action, output)
			}
			if old.chart != nil {
				t.Fatal("renderer remains open")
			}
			if m.preview != nil && m.preview.kittyID == old.kittyID {
				t.Fatal("replacement reused retired image IDs")
			}
		})
	}
}

func collectRaw(cmd tea.Cmd) string {
	if cmd == nil {
		return ""
	}
	msg := cmd()
	if raw, ok := msg.(tea.RawMsg); ok {
		return fmt.Sprint(raw.Msg)
	}
	value := reflect.ValueOf(msg)
	var out strings.Builder
	if value.IsValid() && value.Kind() == reflect.Slice {
		for i := 0; i < value.Len(); i++ {
			if child, ok := value.Index(i).Interface().(tea.Cmd); ok {
				out.WriteString(collectRaw(child))
			}
		}
	}
	return out.String()
}

func TestMenuSuspendsHiddenDocument(t *testing.T) {
	m := New(Options{Files: []string{"a.png"}, Menu: true, Preview: true, Render: "glyph"})
	defer m.Close()
	m.Init()
	if !m.suspended || m.generation != 0 {
		t.Fatal("menu loaded an invisible main document")
	}
	m.Update(document.Result{Generation: m.generation, Kind: "png", Image: image.NewRGBA(image.Rect(0, 0, 2, 2))})
	if m.source != nil {
		t.Fatal("hidden document accepted a late image")
	}
	m.closeMenu(false)
	if m.suspended || !m.loading || m.generation == 0 {
		t.Fatal("closing menu did not resume main document")
	}
}

func TestPreviewDrawsMeshesAsTheViewerDoes(t *testing.T) {
	// The GPU comes first in the preview too, and software is what
	// nt3d falls back to; a renderer asked for by name is kept.
	for _, mode := range []string{"auto", "software", "wireframe"} {
		m := New(Options{Files: []string{"a.stl", "b.stl"}, Render: "glyph", Render3D: mode, Menu: true, Preview: true, Page: 1, DPI: 150})
		m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
		if m.preview == nil || m.preview.opts.Render3D != mode {
			t.Fatalf("--3d %s: the preview draws with %q", mode, m.preview.opts.Render3D)
		}
		if m.preview.opts.DPI != 96 {
			t.Fatalf("previews of PDFs stay at reduced DPI: %d", m.preview.opts.DPI)
		}
		m.Close()
	}
}

func TestListColumnFitsItsNames(t *testing.T) {
	m := viewing(t, Options{Files: []string{samples + "shapes.svg", samples + "readme.md"}, Menu: true, Preview: true}, "svg")
	send(m, tea.WindowSizeMsg{Width: 120, Height: 30})
	// Short names take a narrow column; the preview has the rest.
	if lw := m.listWidth(); lw > 24 || m.previewWidth() != 120-lw-1 {
		t.Fatalf("list %d wide, preview %d", lw, m.previewWidth())
	}
	// A long name widens it, up to the old bound.
	m.opts.Files = append(m.opts.Files, samples+strings.Repeat("a-very-long-name-", 5)+".png")
	if lw := m.listWidth(); lw != min(40, 120/2) {
		t.Fatalf("list %d wide for a long name", lw)
	}
	// Never so narrow that a number and a name cannot show.
	m.opts.Files = []string{"a.png"}
	if lw := m.listWidth(); lw < 16 {
		t.Fatalf("list %d wide", lw)
	}
}

func TestPreviewIsOnlyThePicture(t *testing.T) {
	m := viewing(t, Options{Files: []string{samples + "tetrahedron.stl", samples + "readme.md"}, Menu: true, Preview: true, Render3D: "software"}, "stl")
	send(m, tea.WindowSizeMsg{Width: 120, Height: 30})
	if m.preview == nil {
		t.Fatal("no preview")
	}
	view := ansi.Strip(m.preview.View().Content)
	for _, keys := range []string{"q quit", "m files", "[1/", "e export", "f fit", "5 projection"} {
		if strings.Contains(view, keys) {
			t.Errorf("the preview offers keys of its own: %q in\n%s", keys, view)
		}
	}
	if rows := strings.Count(view, "\n") + 1; rows != m.bodyHeight() {
		t.Errorf("the preview is %d rows in a body of %d", rows, m.bodyHeight())
	}
	// The whole screen says the keys once: the list's.
	whole := plain(m)
	if hints := whole[len(whole)-1]; !strings.Contains(hints, "Enter open") || strings.Count(strings.Join(whole, "\n"), "Enter open") != 1 {
		t.Errorf("hints: %s", hints)
	}
}

func TestListColumnsAreJustified(t *testing.T) {
	files := make([]string, 12)
	for i := range files {
		files[i] = samples + "shapes.svg"
	}
	m := viewing(t, Options{Files: files, Menu: true}, "svg")
	send(m, tea.WindowSizeMsg{Width: 60, Height: 30})
	lines := plain(m)
	// Every name starts in the same column, whether its number has one
	// digit or two.
	col := strings.Index(lines[0], "shapes.svg")
	for i := 0; i < 12; i++ {
		if strings.Index(lines[i], "shapes.svg") != col {
			t.Fatalf("row %d: %q", i+1, lines[i])
		}
	}
	if !strings.HasPrefix(lines[8], "    9  ") || !strings.HasPrefix(lines[9], "   10  ") {
		t.Fatalf("numbers not right-aligned:\n%s\n%s", lines[8], lines[9])
	}
}

// A browser's dropped files are listed by their names, not under the folder
// that holds them in memory.
func TestMenuListsDroppedFilesByName(t *testing.T) {
	m := New(Options{Files: []string{"dropped/plate.3mf", "notes/dropped/a.txt"}, Render: "glyph", Menu: true, Page: 1, DPI: 72})
	defer m.Close()
	m.Update(tea.WindowSizeMsg{Width: 60, Height: 10})
	view := ansi.Strip(m.menuView())
	if strings.Contains(view, "dropped/plate") || !strings.Contains(view, "1  plate.3mf") || !strings.Contains(view, "notes/dropped/a.txt") {
		t.Fatalf("menu:\n%s", view)
	}
}
