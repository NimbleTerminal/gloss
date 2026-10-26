package app

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"nimbleterminal.dev/ntmaps"
	"nimbleterminal.dev/ntmaps/geocode"
	"nimbleterminal.dev/ntmaps/tile"
	ntpicture "nimbleterminal.dev/ntpicture"

	"nimbleterminal.dev/gloss/internal/document"
	"nimbleterminal.dev/gloss/internal/i18n"
)

// MapProvider is an online map service the user named: tiles drawn under a
// GeoJSON or GPX file, and place lookup on any map. gloss has none of its
// own. Either may be nil.
type MapProvider struct {
	Tiles    tile.Source
	Geocoder geocode.Geocoder
}

// placeInput is the find-a-place prompt: the name typed so far.
type placeInput struct{ draft []rune }

// placeFound is a place lookup's answer, for the map it was asked over.
type placeFound struct {
	owner *ntmaps.Model
	query string
	place geocode.Result
	err   error
}

const (
	maxPlaceRunes = 200              // The longest place name the prompt takes.
	placeDeadline = 20 * time.Second // How long a lookup may take, pacing included.
	placeZoom     = 12               // The zoom a found place is shown at.
)

// showMap puts a loaded map in the body through ntmaps, framed to fit: a
// file's places, lines and areas over no tiles, or a PMTiles archive's
// tiles, read from the file; nothing is fetched. Its picture shares the
// pager's terminal session and image IDs; the arrows, hjkl, +/- and the
// mouse move it.
func (m *Model) showMap(v document.Result, cleanup tea.Cmd) tea.Cmd {
	m.source = nil
	source := v.Map.Source()
	if source == nil && m.opts.Maps != nil && m.opts.Maps.Tiles != nil {
		source = m.opts.Maps.Tiles
	}
	view := ntmaps.NewWithConfig(ntmaps.Config{
		Source: source, Cols: m.width, Rows: m.bodyHeight(),
		Session: m.terminal, NextImageID: ntpicture.NextImageID,
		KittyMedium: ntpicture.KittyMediumSharedMemory,
		PreferKitty: m.autoKitty || m.kitty, DisableDiskCache: true,
	})
	view.SetMarkers(v.Map.Data.Markers())
	view.SetPaths(v.Map.Data.Paths())
	view.SetBounds(v.Map.Frame())
	m.mapView, m.mapFile = &view, v.Map
	m.keepLayer()
	var mode tea.Cmd
	if !m.autoKitty && !m.kitty {
		mode = view.SetRenderMode(ntmaps.RenderGlyph)
	}
	return tea.Sequence(cleanup, mode, m.mapView.Init())
}

// clearMap takes the map's picture down and releases it.
func (m *Model) clearMap() tea.Cmd {
	m.placeInput = nil
	if m.mapView == nil {
		return nil
	}
	view := m.mapView
	m.mapView, m.mapFile = nil, nil
	cleanup := view.Dispose()
	view.Close()
	return cleanup
}

// updateMap passes the map a message: its tiles and pictures, and the keys
// and mouse the pager leaves to it.
func (m *Model) updateMap(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	*m.mapView, cmd = m.mapView.Update(msg)
	return cmd
}

// mapKey gives a map the keys it shares with other views: f and 0 frame the
// whole file again, and g switches between glyphs and Kitty graphics.
func (m *Model) mapKey(k string) (tea.Cmd, bool) {
	switch k {
	case "/":
		if m.opts.Maps == nil || m.opts.Maps.Geocoder == nil {
			m.note = "finding a place needs --fetch and " + geocoderVariable
			return nil, true
		}
		m.placeInput = &placeInput{}
		return nil, true
	case "0", "f":
		m.mapView.SetBounds(m.mapFile.Frame())
		return m.mapView.Init(), true
	case "g":
		m.autoKitty = false
		mode := ntmaps.RenderKitty
		if m.mapView.RenderMode() == ntmaps.RenderKitty {
			mode = ntmaps.RenderGlyph
		}
		m.kitty = mode == ntmaps.RenderKitty
		return m.mapView.SetRenderMode(mode), true
	}
	return nil, false
}

// mapExportView is where an export of the map is centered: as on screen.
func (m *Model) mapExportView() *document.MapView {
	if m.mapView == nil {
		return nil
	}
	lat, lng := m.mapView.Center()
	return &document.MapView{Lat: lat, Lng: lng, Zoom: m.mapView.Zoom()}
}

// geocoderVariable names the setting that turns place lookup on.
const geocoderVariable = "GLOSS_GEOCODER"

// placeKey edits the find-a-place prompt: Enter asks the provider, once,
// and Esc closes it.
func (m *Model) placeKey(msg tea.KeyPressMsg) tea.Cmd {
	in := m.placeInput
	switch msg.String() {
	case "ctrl+c":
		return m.quit()
	case "esc":
		m.placeInput = nil
	case "enter":
		m.placeInput = nil
		query := strings.TrimSpace(string(in.draft))
		if query == "" || m.mapView == nil {
			return nil
		}
		m.note = "finding " + query + "…"
		return findPlace(m.opts.Maps.Geocoder, m.mapView, query)
	case "backspace":
		if len(in.draft) > 0 {
			in.draft = in.draft[:len(in.draft)-1]
		}
	case "ctrl+u":
		in.draft = nil
	default:
		for _, r := range msg.Text {
			if unicode.IsSpace(r) {
				r = ' '
			}
			if len(in.draft) < maxPlaceRunes && unicode.IsPrint(r) {
				in.draft = append(in.draft, r)
			}
		}
	}
	return nil
}

// status is the prompt as the hint line shows it.
func (in *placeInput) status(width int) string {
	return ansi.Truncate(" "+i18n.T("app.place.prompt", "Find a place:")+" "+string(in.draft)+"\x1b[7m▏\x1b[27m  "+i18n.T("app.place.keys", "Enter find · Esc cancel"), width, "")
}

// findPlace asks the geocoder where query is, off the event loop.
func findPlace(g geocode.Geocoder, owner *ntmaps.Model, query string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), placeDeadline)
		defer cancel()
		place, err := g.Geocode(ctx, query)
		return placeFound{owner: owner, query: query, place: place, err: err}
	}
}

// showPlace centers the map on a place found, or says why there is none.
// An answer for a map no longer shown is dropped.
func (m *Model) showPlace(msg placeFound) tea.Cmd {
	if m.mapView == nil || msg.owner != m.mapView {
		return nil
	}
	switch {
	case errors.Is(msg.err, geocode.ErrNotFound):
		m.note = "no place found for " + msg.query
		return nil
	case msg.err != nil:
		m.note = "finding a place failed: " + plainNote(msg.err.Error())
		return nil
	}
	m.note = msg.query
	if name := plainNote(msg.place.Name); name != "" {
		m.note = name
	}
	m.mapView.SetLatLng(msg.place.Lat, msg.place.Lng, min(placeZoom, maxZoom(m.mapView)))
	return m.mapView.Init()
}

// maxZoom is the deepest zoom the map's tiles go to.
func maxZoom(v *ntmaps.Model) int {
	if s := v.TileSource(); s != nil && s.Info().MaxZoom > 0 {
		return s.Info().MaxZoom
	}
	return placeZoom
}

// plainNote makes text from a server one short line with no terminal
// controls in it.
func plainNote(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return ' '
		}
		if !unicode.IsPrint(r) {
			return -1
		}
		return r
	}, s)
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > 120 {
		s = string(r[:120]) + "…"
	}
	return s
}
