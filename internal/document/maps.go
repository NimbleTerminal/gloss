package document

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"strconv"
	"strings"

	"nimbleterminal.dev/ntmaps"
	"nimbleterminal.dev/ntmaps/geodata"
	"nimbleterminal.dev/ntmaps/tile"
)

// Map is a file of things on a map, GeoJSON or GPX: its places, lines and
// areas, as ntmaps' geodata reads them. The viewer draws them over no tiles,
// so nothing is fetched.
type Map struct {
	Data geodata.Data
	View *MapView // Where an export is centered, as the viewer has it; nil frames the whole file.
}

// MapView is a map's center and zoom.
type MapView struct {
	Lat, Lng float64
	Zoom     int
}

// mapLimits bound a GeoJSON or GPX file: the input cap, and geodata's own
// limits on its positions and features.
var mapLimits = geodata.Limits{Bytes: MaxFileBytes}

// mapEdge is the longer side of a map drawn for an export, a preview or
// --print, unless --max-edge says otherwise.
const mapEdge = 1200

// errEmptyMap is a file with nothing to place: no position in it.
var errEmptyMap = errors.New("the file has no places, lines or areas to show")

// readMap reads a GeoJSON or GPX file.
func readMap(kind string, data []byte) (*Map, error) {
	var (
		d   geodata.Data
		err error
	)
	if kind == "gpx" {
		d, err = geodata.ReadGPX(bytes.NewReader(data), mapLimits)
	} else {
		d, err = geodata.ReadGeoJSON(bytes.NewReader(data), mapLimits)
	}
	if err != nil {
		return nil, err
	}
	return &Map{Data: d}, nil
}

// Frame is the box the map shows at first: around everything in the file,
// kept within the Mercator latitudes and given a little height when it has
// none, as for one place or a line due east.
func (m *Map) Frame() ntmaps.Bounds {
	b := m.Data.Bounds
	b.South = max(-tile.MaxLatitude, min(tile.MaxLatitude, b.South))
	b.North = max(-tile.MaxLatitude, min(tile.MaxLatitude, b.North))
	if b.South >= b.North {
		b.South, b.North = b.South-1e-9, b.North+1e-9
	}
	return b
}

// picture draws the map once on no tiles, framed or at its View, at most
// maxEdge (mapEdge when zero) along its longer side: what an export, a
// preview, a thumbnail and --print show.
func (m *Map) picture(maxEdge int) (image.Image, error) {
	if m.Data.Points == 0 {
		return nil, errEmptyMap
	}
	edge := mapEdge
	if maxEdge > 0 {
		edge = maxEdge
	}
	o := ntmaps.RenderOptions{
		Width: edge, Height: max(1, edge*2/3),
		Markers: m.Data.Markers(), Paths: m.Data.Paths(),
	}
	if m.View != nil {
		o.Lat, o.Lng, o.Zoom = m.View.Lat, m.View.Lng, m.View.Zoom
	} else {
		frame := m.Frame()
		o.Bounds = &frame
	}
	return ntmaps.Render(context.Background(), o)
}

// mapFields describes the file for the info panel.
func mapFields(kind string, m *Map) []Field {
	format := "GeoJSON"
	if kind == "gpx" {
		format = "GPX"
	}
	if m == nil {
		return Section("Map", Field{"Format", format})
	}
	var places, lines, areas int
	var names []string
	for _, f := range m.Data.Features {
		places += len(f.Places)
		lines += len(f.Lines)
		areas += len(f.Areas)
		if f.Name != "" && len(names) < mapNames {
			names = append(names, f.Name)
		}
	}
	count := func(n int) string {
		if n == 0 {
			return ""
		}
		return strconv.Itoa(n)
	}
	named := strings.Join(names, ", ")
	if more := namedFeatures(m.Data.Features) - len(names); more > 0 {
		named += fmt.Sprintf(" and %d more", more)
	}
	var bounds string
	if m.Data.Points > 0 {
		b := m.Data.Bounds
		bounds = fmt.Sprintf("%s, %s to %s, %s", coordinate(b.South), coordinate(b.West), coordinate(b.North), coordinate(b.East))
	}
	return Section("Map",
		Field{"Format", format},
		Field{"Features", strconv.Itoa(len(m.Data.Features))},
		Field{"Places", count(places)},
		Field{"Lines", count(lines)},
		Field{"Areas", count(areas)},
		Field{"Positions", strconv.Itoa(m.Data.Points)},
		Field{"Bounds", bounds},
		Field{"Names", named},
	)
}

// mapNames is how many feature names the info panel lists.
const mapNames = 8

func namedFeatures(fs []geodata.Feature) int {
	n := 0
	for _, f := range fs {
		if f.Name != "" {
			n++
		}
	}
	return n
}

func coordinate(v float64) string { return strconv.FormatFloat(v, 'f', 5, 64) }
