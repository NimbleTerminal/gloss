package app

import (
	"image"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"nimbleterminal.dev/gloss/internal/document"
)

// mapViewing loads a map fixture into the viewer as a finished load would.
func mapViewing(t *testing.T, opts Options) *Model {
	t.Helper()
	opts.Files = []string{samples + "walk.gpx"}
	m := viewing(t, opts, "")
	var l document.Loader
	r := l.Load(document.Request{Path: opts.Files[0], Generation: 1})
	if r.Err != nil {
		t.Fatal(r.Err)
	}
	m.Update(r)
	if m.mapView == nil {
		t.Fatal("the map was not shown")
	}
	return m
}

func TestMapViewKeysAndStatus(t *testing.T) {
	m := mapViewing(t, Options{})
	if m.View().MouseMode != tea.MouseModeCellMotion {
		t.Fatal("the map takes drags and the wheel")
	}
	status := ansi.Strip(m.View().Content)
	if !strings.Contains(status, "GPX · 4 features · zoom") || !strings.Contains(status, "arrows pan") {
		t.Fatalf("status:\n%s", status)
	}
	framed := m.mapView.Zoom()
	// The map's own keys reach it, not the picture zoom.
	m.Update(press("+"))
	if m.mapView.Zoom() != framed+1 || m.zoom != 0 {
		t.Fatalf("+ zoomed %d → %d, picture zoom %d", framed, m.mapView.Zoom(), m.zoom)
	}
	lat, lng := m.mapView.Center()
	m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	if a, b := m.mapView.Center(); a == lat && b == lng {
		t.Fatal("→ did not pan")
	}
	m.Update(press("f"))
	if m.mapView.Zoom() != framed {
		t.Fatalf("f framed at zoom %d, not %d", m.mapView.Zoom(), framed)
	}
	if m.mapExportView().Zoom != framed {
		t.Fatal("an export is not centered as the screen")
	}
}

func TestMapLeavesWithItsFile(t *testing.T) {
	m := mapViewing(t, Options{})
	m.generation++
	m.Update(document.Result{Generation: m.generation, Kind: "png", Page: 1, Pages: 1, Image: image.NewRGBA(image.Rect(0, 0, 8, 8))})
	if m.mapView != nil || m.mapFile != nil {
		t.Fatal("the map stayed after another file was shown")
	}
}

func TestMapInPreviewIsAPicture(t *testing.T) {
	m := viewing(t, Options{Files: []string{samples + "walk.gpx"}}, "")
	m.isPreview = true
	var l document.Loader
	r := l.Load(document.Request{Path: samples + "walk.gpx", Generation: 1, Preview: true})
	m.Update(r)
	if m.mapView != nil || m.source == nil {
		t.Fatal("a preview shows the framed picture, not a live map")
	}
}
