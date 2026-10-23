package document

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

// AcceptFilter restricts a session to the named content kinds. An empty filter
// accepts every supported kind. It never trusts --type or a binary extension.
type AcceptFilter string

func ParseAccept(value string) (AcceptFilter, error) {
	if value == "" {
		return "", nil
	}
	for _, item := range strings.Split(value, ",") {
		switch strings.TrimSpace(item) {
		case "image", "images", "image/*", "svg", "pdf", "stl", "3mf", "obj", "ply", "docx", "xlsx", "grist", "csv", "json", "jsonl", "ndjson", "ipynb", "html", "text", "markdown", "md", "mermaid", "video", "video/*", "geojson", "gpx", "pmtiles", "map", "maps":
		default:
			return "", fmt.Errorf("--accept: unknown format %q; use gloss format names, or image/*", item)
		}
	}
	return AcceptFilter(value), nil
}

// Check identifies the bytes without a forced type or misleading extension.
// Text formats without a distinctive signature retain their filename hint.
// This checks format, not complete document validity; loaders still validate.
func (a AcceptFilter) Check(name string, data []byte) error {
	if a == "" {
		return nil
	}
	kind, err := Detect("", data, "")
	if err != nil {
		return fmt.Errorf("content does not match --accept %s: %w", a, err)
	}
	if kind == "text" {
		if hinted, e := Detect(name, data, ""); e == nil {
			switch hinted {
			case "text", "markdown", "csv", "json", "ipynb", "html", "mermaid", "obj", "geojson":
				kind = hinted
			}
		}
	}
	// GeoJSON is JSON known by its name.
	if kind == "json" && strings.EqualFold(filepath.Ext(name), ".geojson") {
		kind = "geojson"
	}
	for _, item := range strings.Split(string(a), ",") {
		item = strings.TrimSpace(item)
		switch item {
		case "image", "images", "image/*":
			switch kind {
			case "png", "jpeg", "gif", "webp", "bmp", "tiff", "heic", "heif", "jxl", "svg":
				return nil
			}
		case "video/*":
			item = "video"
		case "map", "maps":
			if kind == "geojson" || kind == "gpx" || kind == "pmtiles" {
				return nil
			}
		case "md":
			item = "markdown"
		case "jsonl", "ndjson":
			item = "json"
		}
		if item == kind {
			return nil
		}
	}
	return fmt.Errorf("received %s; required --accept %s", kind, a)
}

func (a AcceptFilter) CheckFile(files fs.FS, name string) error {
	if a == "" {
		return nil
	}
	// A video or a PMTiles archive is known by its first bytes, and is never
	// read whole.
	if head, err := readHead(files, name, videoHeadBytes); err == nil && (sniffVideo(head) || sniffPMTiles(head)) {
		return a.Check(name, head)
	}
	data, err := readFileFrom(files, name)
	if err != nil {
		return err
	}
	return a.Check(name, data)
}
