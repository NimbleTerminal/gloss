package document

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"nimbleterminal.dev/ntmaps"
	"nimbleterminal.dev/ntmaps/tile"
	"nimbleterminal.dev/ntmaps/tile/pmtiles"
)

// A PMTiles archive is a whole tile set in one file, often gigabytes, so it
// is never read into memory, as other files are: the archive's directories
// say where each tile is, and ntmaps reads those parts as the map needs
// them, each read bounded by pmtiles' limits rather than MaxFileBytes. It is
// known by its first bytes or its name. Only raster archives (PNG or JPEG
// tiles) can be drawn; a vector archive is described, not shown.

// pmtilesMagic begins every PMTiles archive; the byte after it is the
// version.
const pmtilesMagic = "PMTiles"

// sniffPMTiles says whether data begins as a PMTiles archive.
func sniffPMTiles(data []byte) bool { return bytes.HasPrefix(data, []byte(pmtilesMagic)) }

// isPMTilesFile says whether the file at path is a PMTiles archive, by
// --type, its first bytes or its name.
func isPMTilesFile(files fs.FS, path, forced string) bool {
	if forced != "" {
		return forced == "pmtiles"
	}
	if strings.EqualFold(filepath.Ext(path), ".pmtiles") {
		return true
	}
	head, err := readHead(files, path, len(pmtilesMagic))
	return err == nil && sniffPMTiles(head)
}

// openPMTiles opens an archive: from its path on the host, or from memory
// when the files are embedded or in the browser.
func openPMTiles(files fs.FS, path string) (*pmtiles.Archive, pmtiles.Header, int64, error) {
	var head []byte
	var size int64
	open := func() (*pmtiles.Archive, error) { return pmtiles.Open(path, pmtiles.Options{}) }
	if files == nil {
		info, err := os.Stat(path)
		if err != nil {
			return nil, pmtiles.Header{}, 0, err
		}
		if !info.Mode().IsRegular() {
			return nil, pmtiles.Header{}, 0, ErrNotRegular
		}
		size = info.Size()
		if head, err = readHead(nil, path, pmtiles.HeaderSize); err != nil {
			return nil, pmtiles.Header{}, size, err
		}
	} else {
		data, err := readFileFrom(files, path)
		if err != nil {
			return nil, pmtiles.Header{}, 0, err
		}
		size, head = int64(len(data)), data
		name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		open = func() (*pmtiles.Archive, error) {
			return pmtiles.New(bytes.NewReader(data), pmtiles.Options{Name: name})
		}
	}
	h, err := pmtiles.ParseHeader(head)
	if err != nil {
		return nil, h, size, err
	}
	a, err := open()
	if errors.Is(err, pmtiles.ErrUnsupported) && h.TileType == pmtiles.TypeMVT {
		err = errors.New("pmtiles: a vector archive; gloss draws only raster (PNG or JPEG) tiles")
	}
	return a, h, size, err
}

// loadPMTiles opens an archive and keeps it open, as a PDF is kept, while
// the viewer shows it: the map reads its tiles as it pans and zooms. A
// picture of it, framed on the archive's bounds, stands for it in previews,
// thumbnails, exports and --print.
func (l *Loader) loadPMTiles(q Request) (out Result) {
	out.Generation, out.Kind, out.Page, out.Pages = q.Generation, "pmtiles", 1, 1
	if l.archive == nil || l.path != q.Path {
		_ = l.closePDF()
		a, h, size, err := openPMTiles(l.Files, q.Path)
		file := fileFields(l.Files, q.Path, int(min(size, int64(^uint(0)>>1))), q.Stdin)
		out.Info = append(file, pmtilesFields(h, a)...)
		if err != nil {
			out.Err = err
			return out
		}
		l.path, l.archive, l.archiveInfo = q.Path, a, out.Info
	}
	out.Info = l.archiveInfo
	out.Map = &Map{Tiles: l.archive}
	if !q.Describe && !q.TextOnly {
		out.Image, out.Err = out.Map.picture(q.MaxEdge)
	}
	return out
}

// pmtilesFields describes an archive from its header and, once it opens,
// its metadata.
func pmtilesFields(h pmtiles.Header, a *pmtiles.Archive) []Field {
	if h.Version == 0 {
		return Section("Map", Field{"Format", "PMTiles"})
	}
	tiles := map[uint8]string{pmtiles.TypeMVT: "vector (MVT)", pmtiles.TypePNG: "PNG", pmtiles.TypeJPEG: "JPEG", pmtiles.TypeWebP: "WebP", pmtiles.TypeAVIF: "AVIF"}[h.TileType]
	var name, attribution string
	if a != nil {
		attribution = a.Info().Attribution
		if meta, err := a.Metadata(); err == nil {
			var m struct {
				Name string `json:"name"`
			}
			if json.Unmarshal(meta, &m) == nil {
				name = m.Name
			}
		}
	}
	return Section("Map",
		Field{"Format", "PMTiles v" + strconv.Itoa(int(h.Version))},
		Field{"Name", strings.Map(printable, name)},
		Field{"Tiles", tiles},
		Field{"Tile count", strconv.FormatUint(h.AddressedTiles, 10)},
		Field{"Zooms", fmt.Sprintf("%d to %d", h.MinZoom, h.MaxZoom)},
		Field{"Bounds", fmt.Sprintf("%s, %s to %s, %s", coordinate(h.MinLat), coordinate(h.MinLon), coordinate(h.MaxLat), coordinate(h.MaxLon))},
		Field{"Attribution", attribution},
	)
}

// archiveFrame is the box an archive shows at first: the bounds its header
// declares, or the whole world when they say nothing usable.
func archiveFrame(a *pmtiles.Archive) ntmaps.Bounds {
	south, west, north, east := a.Bounds()
	b := ntmaps.Bounds{South: south, West: west, North: north, East: east}
	if !b.Valid() || south >= north || west >= east {
		return ntmaps.Bounds{South: -tile.MaxLatitude, West: -180, North: tile.MaxLatitude, East: 180}
	}
	return b
}
