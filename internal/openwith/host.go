//go:build !js

package openwith

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

type nativeHost struct {
	mu      sync.Mutex
	catalog Catalog
	dir     string
	prefs   Preferences
	invalid error
}

func NewHost(c Catalog, dir string) (Host, error) {
	p, err := ReadPreferences(dir, c)
	var pathError *os.PathError
	if errors.As(err, &pathError) {
		return nil, err
	}
	return &nativeHost{catalog: c, dir: dir, prefs: p, invalid: err}, err
}
func (h *nativeHost) available(id string) bool {
	return h.catalog.Available[id] && !slices.Contains(h.prefs.Hidden, id)
}
func (h *nativeHost) defaultID(kind string) string {
	if id := h.prefs.Defaults[kind]; id != "" && h.available(id) {
		return id
	}
	return "system"
}
func (h *nativeHost) Choices(t Target) ([]Choice, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out []Choice
	if t.Directory {
		e, _ := h.catalog.entry("reveal")
		return []Choice{{ID: e.ID, Name: e.Name, Available: h.available(e.ID)}}, nil
	}
	def := h.defaultID(t.Kind)
	for _, e := range h.catalog.Entries {
		if e.ID == "reveal" || !e.accepts(t.Kind) || (e.ID != "system" && !h.available(e.ID)) {
			continue
		}
		out = append(out, Choice{ID: e.ID, Name: e.Name, Default: e.ID == def, Available: h.available(e.ID)})
	}
	if len(out) > 1 {
		slices.SortFunc(out[1:], func(a, b Choice) int { return strings.Compare(a.Name, b.Name) })
	}
	e, _ := h.catalog.entry("reveal")
	out = append(out, Choice{ID: e.ID, Name: e.Name, Available: h.available(e.ID)})
	return out, nil
}
func (h *nativeHost) SetDefault(kind, id string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.invalid != nil {
		return fmt.Errorf("repair open-with.json before saving: %w", h.invalid)
	}
	e, ok := h.catalog.entry(id)
	if !slices.Contains(Kinds, kind) || !ok || id == "reveal" || !e.accepts(kind) || !h.available(id) {
		return fmt.Errorf("app cannot be the default for %s", kind)
	}
	p := Preferences{Defaults: maps.Clone(h.prefs.Defaults), Hidden: slices.Clone(h.prefs.Hidden)}
	if id == "system" {
		delete(p.Defaults, kind)
	} else {
		p.Defaults[kind] = id
	}
	if err := WritePreferences(h.dir, p); err != nil {
		return err
	}
	h.prefs = p
	return nil
}
func (h *nativeHost) Launch(t Target, id string) (string, error) {
	path, err := filepath.Abs(t.Path)
	if err != nil || t.Path == "" || strings.ContainsRune(t.Path, 0) {
		return "", fmt.Errorf("invalid file path")
	}
	path = filepath.Clean(path)
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		if !t.Directory || id != "reveal" {
			return "", fmt.Errorf("directories can only be revealed")
		}
	} else if !info.Mode().IsRegular() || t.Directory {
		return "", fmt.Errorf("not a regular file")
	}
	h.mu.Lock()
	fallback := false
	if id == "" {
		id = h.defaultID(t.Kind)
		fallback = id == "system" && h.prefs.Defaults[t.Kind] != ""
	}
	e, ok := h.catalog.entry(id)
	available := h.available(id)
	platform := h.catalog.OS
	h.mu.Unlock()
	if !ok || !available || !e.accepts(t.Kind) {
		return "", fmt.Errorf("app %q is unavailable for this file", id)
	}
	if id == "reveal" {
		switch platform {
		case "darwin":
			if t.Directory {
				e.Exec = e.Exec[:1]
			}
		case "windows":
			if t.Directory {
				e.Exec = e.Exec[:1]
			}
		default:
			if !t.Directory {
				path = filepath.Dir(path)
			}
		}
	}
	if err := Start(e, path); err != nil {
		return "", err
	}
	name := e.Name
	if fallback {
		name += " (saved app unavailable)"
	}
	return name, nil
}
