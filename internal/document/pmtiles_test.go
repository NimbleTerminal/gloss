package document

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"nimbleterminal.dev/ntmaps/tile"
	"nimbleterminal.dev/ntmaps/tile/pmtiles"
)

const islands = "../../examples/islands.pmtiles"

func TestDetectPMTiles(t *testing.T) {
	data, err := os.ReadFile(islands)
	if err != nil {
		t.Fatal(err)
	}
	if kind, err := Detect("", data[:16], ""); err != nil || kind != "pmtiles" {
		t.Fatalf("by its bytes: %q %v", kind, err)
	}
	dir := t.TempDir()
	renamed := filepath.Join(dir, "tiles.bin")
	if err := os.WriteFile(renamed, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if !isPMTilesFile(nil, renamed, "") || !isPMTilesFile(nil, filepath.Join(dir, "missing.pmtiles"), "") {
		t.Fatal("an archive is known by its bytes or its name")
	}
	if isPMTilesFile(nil, "../../examples/walk.gpx", "") || isPMTilesFile(nil, renamed, "image") {
		t.Fatal("not an archive")
	}
}

func TestLoadPMTiles(t *testing.T) {
	var l Loader
	r := l.Load(Request{Path: islands, Generation: 1})
	if r.Err != nil || r.Kind != "pmtiles" || r.Map == nil || r.Map.Tiles == nil {
		t.Fatalf("%+v", r.Err)
	}
	if r.Image == nil || r.Image.Bounds().Dx() != mapEdge {
		t.Fatal("no framed picture")
	}
	for label, want := range map[string]string{"Format": "PMTiles v3", "Name": "Islands", "Tiles": "PNG", "Tile count": "85", "Zooms": "0 to 3", "Attribution": "gloss examples"} {
		if got := field(r.Info, label); got != want {
			t.Errorf("%s = %q, want %q", label, got, want)
		}
	}
	// The archive stays open for the viewer, and an export at its view reads it.
	r.Map.View = &MapView{Lat: 10, Lng: 20, Zoom: 3}
	if img, err := ExportImage(r, 300); err != nil || img.Bounds().Dx() != 300 {
		t.Fatalf("export at a view: %v", err)
	}
	if d := l.Load(Request{Path: islands, Generation: 2, Describe: true}); d.Err != nil || d.Image != nil || field(d.Info, "Format") == "" {
		t.Fatalf("describing: %v", d.Err)
	}
	// Another file closes it.
	if l.Load(Request{Path: "../../examples/walk.gpx", Generation: 3}); l.archive != nil {
		t.Fatal("the archive outlived its file")
	}
	if _, err := r.Map.Tiles.TileData(t.Context(), tile.ID{}); err == nil {
		t.Fatal("a closed archive still read")
	}
}

func TestPMTilesErrors(t *testing.T) {
	data, err := os.ReadFile(islands)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	vector := append([]byte(nil), data...)
	vector[99] = pmtiles.TypeMVT
	for name, body := range map[string][]byte{
		"vector.pmtiles":    vector,
		"truncated.pmtiles": data[:200],
		"short.pmtiles":     data[:20],
		"text.pmtiles":      []byte("not an archive"),
	} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, body, 0o600); err != nil {
			t.Fatal(err)
		}
		var l Loader
		r := l.Load(Request{Path: path, Generation: 1})
		if r.Err == nil {
			t.Errorf("%s: no error", name)
		}
		if name == "vector.pmtiles" && (!strings.Contains(r.Err.Error(), "vector") || field(r.Info, "Tiles") != "vector (MVT)") {
			t.Errorf("vector: %v, info %+v", r.Err, r.Info)
		}
	}
}

func TestPMTilesInMemory(t *testing.T) {
	data, err := os.ReadFile(islands)
	if err != nil {
		t.Fatal(err)
	}
	l := Loader{Files: fstest.MapFS{"islands.pmtiles": {Data: data}}}
	if r := l.Load(Request{Path: "islands.pmtiles", Generation: 1}); r.Err != nil || r.Image == nil {
		t.Fatalf("embedded: %v", r.Err)
	}
}

func TestAcceptPMTiles(t *testing.T) {
	for accept, ok := range map[string]bool{"pmtiles": true, "maps": true, "map": true, "image/*": false, "gpx": false} {
		a, err := ParseAccept(accept)
		if err != nil {
			t.Fatal(err)
		}
		if err := a.CheckFile(nil, islands); (err == nil) != ok {
			t.Errorf("--accept %s: %v", accept, err)
		}
	}
}
