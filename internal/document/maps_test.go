package document

import (
	"os"
	"strings"
	"testing"
)

func TestDetectMaps(t *testing.T) {
	gpx := []byte(`<?xml version="1.0"?><gpx version="1.1"><wpt lat="1" lon="2"/></gpx>`)
	geo := []byte(`{"type":"Point","coordinates":[2,1]}`)
	for _, c := range []struct {
		name string
		data []byte
		want string
	}{
		{"walk.gpx", gpx, "gpx"},
		{"", gpx, "gpx"}, // By its root element, as stdin.
		{"places.geojson", geo, "geojson"},
		{"PLACES.GEOJSON", geo, "geojson"},
		{"point.json", geo, "json"}, // JSON is GeoJSON only by name.
		{"", geo, "json"},
	} {
		if kind, err := Detect(c.name, c.data, ""); err != nil || kind != c.want {
			t.Errorf("%q: %q %v, want %q", c.name, kind, err, c.want)
		}
	}
}

func TestLoadMap(t *testing.T) {
	var l Loader
	var generation uint64
	for _, c := range []struct {
		path, format string
		features     int
	}{
		{"../../examples/walk.gpx", "GPX", 4},
		{"../../examples/places.geojson", "GeoJSON", 4},
	} {
		generation++
		r := l.Load(Request{Path: c.path, Generation: generation})
		if r.Err != nil || r.Map == nil || len(r.Map.Data.Features) != c.features {
			t.Fatalf("%s: %+v", c.path, r.Err)
		}
		if r.Image == nil || r.Image.Bounds().Dx() != mapEdge {
			t.Fatalf("%s: no framed picture", c.path)
		}
		if field(r.Info, "Format") != c.format || field(r.Info, "Positions") == "" || field(r.Info, "Bounds") == "" {
			t.Fatalf("%s: info %+v", c.path, r.Info)
		}
		// --info draws nothing.
		generation++
		if d := l.Load(Request{Path: c.path, Generation: generation, Describe: true}); d.Image != nil || d.Map == nil {
			t.Fatalf("%s: describing drew a picture", c.path)
		}
	}
}

func TestMapExportAndView(t *testing.T) {
	m, err := readMap("gpx", []byte(`<gpx><wpt lat="40.7" lon="-74"/></gpx>`))
	if err != nil {
		t.Fatal(err)
	}
	// One place frames at a height of its own rather than failing.
	img, err := ExportImage(Result{Map: m}, 300)
	if err != nil || img.Bounds().Dx() != 300 || img.Bounds().Dy() != 200 {
		t.Fatalf("export %v %v", img, err)
	}
	m.View = &MapView{Lat: 40.7, Lng: -74, Zoom: 3}
	if _, err := ExportImage(Result{Map: m}, 300); err != nil {
		t.Fatalf("export at a view: %v", err)
	}
}

func TestMapErrors(t *testing.T) {
	dir := t.TempDir()
	for name, data := range map[string]string{
		"empty.gpx":      `<gpx></gpx>`,
		"bad.geojson":    `{"type":"Point","coordinates":[0,95]}`,
		"broken.gpx":     `<gpx><wpt lat="1"`,
		"circle.geojson": `{"type":"Circle"}`,
	} {
		path := dir + "/" + name
		if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
		var l Loader
		r := l.Load(Request{Path: path, Generation: 1})
		if r.Err == nil {
			t.Errorf("%s: no error", name)
		}
		if field(r.Info, "Format") == "" {
			t.Errorf("%s: a map that cannot be shown is still described", name)
		}
	}
	var l Loader
	if r := l.Load(Request{Path: dir + "/empty.gpx", Generation: 1}); r.Err == nil || !strings.Contains(r.Err.Error(), "no places") {
		t.Fatalf("empty: %v", r.Err)
	}
}

func TestAcceptMaps(t *testing.T) {
	geo := []byte(`{"type":"Point","coordinates":[2,1]}`)
	gpx := []byte(`<gpx><wpt lat="1" lon="2"/></gpx>`)
	for _, c := range []struct {
		accept, name string
		data         []byte
		ok           bool
	}{
		{"geojson", "a.geojson", geo, true},
		{"maps", "a.geojson", geo, true},
		{"maps", "a.gpx", gpx, true},
		{"gpx", "renamed.txt", gpx, true},
		{"geojson", "a.json", geo, false},
		{"json", "a.geojson", geo, false},
	} {
		a, err := ParseAccept(c.accept)
		if err != nil {
			t.Fatal(err)
		}
		if err := a.Check(c.name, c.data); (err == nil) != c.ok {
			t.Errorf("--accept %s %s: %v", c.accept, c.name, err)
		}
	}
}
