package app

import (
	tea "charm.land/bubbletea/v2"
	"nimbleterminal.dev/ntmaps"
	ntpicture "nimbleterminal.dev/ntpicture"

	"nimbleterminal.dev/gloss/internal/document"
)

// showMap puts a loaded map in the body through ntmaps: the file's places,
// lines and areas over no tiles, so nothing is fetched, framed to fit. Its
// picture shares the pager's terminal session and image IDs; the arrows,
// hjkl, +/- and the mouse move it.
func (m *Model) showMap(v document.Result, cleanup tea.Cmd) tea.Cmd {
	m.source = nil
	view := ntmaps.NewWithConfig(ntmaps.Config{
		Cols: m.width, Rows: m.bodyHeight(),
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
