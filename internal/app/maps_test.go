package app

import (
	"context"
	"image"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"nimbleterminal.dev/ntmaps/geocode"
	"nimbleterminal.dev/ntmaps/tile"

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

func TestPMTilesViewer(t *testing.T) {
	opts := Options{Files: []string{samples + "islands.pmtiles"}}
	m := viewing(t, opts, "")
	var l document.Loader
	m.Update(l.Load(document.Request{Path: opts.Files[0], Generation: 1}))
	if m.mapView == nil || m.mapView.TileSource() == nil {
		t.Fatal("the archive's tiles are not the map's")
	}
	if status := ansi.Strip(m.View().Content); !strings.Contains(status, "PMTiles · zoom") {
		t.Fatalf("status:\n%s", status)
	}
}

// testTiles is a tile source that draws every tile one color, offline.
type testTiles struct{}

func (testTiles) Info() tile.Info { return tile.Info{Name: "test tiles", MaxZoom: 18} }
func (testTiles) Tile(context.Context, tile.ID) (image.Image, error) {
	return image.NewRGBA(image.Rect(0, 0, 256, 256)), nil
}

func TestMapProviderTilesAndPlaces(t *testing.T) {
	asked := 0
	finder := geocode.Func(func(_ context.Context, query string) (geocode.Result, error) {
		asked++
		if query == "nowhere" {
			return geocode.Result{}, geocode.ErrNotFound
		}
		return geocode.Result{Lat: 48.85, Lng: 2.35, Name: "Paris\x1b[31m, France"}, nil
	})
	m := mapViewing(t, Options{Maps: &MapProvider{Tiles: testTiles{}, Geocoder: finder}})
	if _, ok := m.mapView.TileSource().(testTiles); !ok {
		t.Fatal("the named provider's tiles are not under the file")
	}
	if !strings.Contains(m.hints(), "/ find a place") {
		t.Fatalf("hints: %s", m.hints())
	}
	m.Update(press("/"))
	for _, k := range []string{"q", "+", "x"} {
		m.Update(press(k)) // Typed, not quitting or zooming.
	}
	if m.placeInput == nil || string(m.placeInput.draft) != "q+x" || m.exiting {
		t.Fatal("the prompt did not take the keys")
	}
	m.placeInput.draft = []rune("Paris")
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil || m.placeInput != nil {
		t.Fatal("Enter asked nothing")
	}
	found := findPlace(finder, m.mapView, "Paris")().(placeFound)
	m.Update(found)
	if lat, lng := m.mapView.Center(); lat != 48.85 || lng != 2.35 || m.mapView.Zoom() != placeZoom {
		t.Fatalf("centered on %v,%v zoom %d", lat, lng, m.mapView.Zoom())
	}
	if m.note != "Paris[31m, France" {
		t.Fatalf("note %q", m.note)
	}
	m.Update(findPlace(finder, m.mapView, "nowhere")())
	if !strings.Contains(m.note, "no place found") {
		t.Fatalf("not found: %q", m.note)
	}
	// An answer for a map that is gone changes nothing.
	stale := found
	stale.owner = nil
	m.note = ""
	m.Update(stale)
	if m.note != "" {
		t.Fatal("a stale answer was shown")
	}
}

func TestMapWithoutProvider(t *testing.T) {
	m := mapViewing(t, Options{})
	if m.mapView.TileSource() != nil {
		t.Fatal("tiles with no provider named")
	}
	m.Update(press("/"))
	if m.placeInput != nil || !strings.Contains(m.note, "GLOSS_GEOCODER") {
		t.Fatalf("no geocoder: prompt %v, note %q", m.placeInput != nil, m.note)
	}
}

func TestArchiveKeepsItsTilesUnderAProvider(t *testing.T) {
	opts := Options{Files: []string{samples + "islands.pmtiles"}, Maps: &MapProvider{Tiles: testTiles{}}}
	m := viewing(t, opts, "")
	var l document.Loader
	m.Update(l.Load(document.Request{Path: opts.Files[0], Generation: 1}))
	if _, online := m.mapView.TileSource().(testTiles); online {
		t.Fatal("an archive's own tiles gave way to the provider's")
	}
}
