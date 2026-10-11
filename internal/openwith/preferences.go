//go:build !js

package openwith

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
)

const maxPreferences = 64 << 10

type Preferences struct {
	Defaults map[string]string `json:"defaults"`
	Hidden   []string          `json:"hidden"`
}

func ReadPreferences(dir string, c Catalog) (Preferences, error) {
	p := Preferences{Defaults: map[string]string{}}
	f, err := os.Open(filepath.Join(dir, "open-with.json"))
	if os.IsNotExist(err) {
		return p, nil
	}
	if err != nil {
		return p, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, maxPreferences+1))
	if err != nil {
		return p, err
	}
	if len(b) > maxPreferences {
		return p, fmt.Errorf("open-with preferences exceed 64 KiB")
	}
	if b = bytes.TrimSpace(b); len(b) == 0 || b[0] != '{' {
		return p, fmt.Errorf("open-with preferences must be a JSON object")
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	var raw Preferences
	if err := dec.Decode(&raw); err != nil {
		return p, err
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return p, fmt.Errorf("trailing open-with preference data")
	}
	for kind, id := range raw.Defaults {
		e, ok := c.entry(id)
		if slices.Contains(Kinds, kind) && ok && id != "reveal" && e.accepts(kind) && id != "system" {
			p.Defaults[kind] = id
		}
	}
	for _, id := range raw.Hidden {
		if _, ok := c.entry(id); ok && id != "system" && id != "reveal" && !slices.Contains(p.Hidden, id) {
			p.Hidden = append(p.Hidden, id)
		}
	}
	return p, nil
}

func WritePreferences(dir string, p Preferences) error {
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	if len(b) > maxPreferences {
		return fmt.Errorf("open-with preferences exceed 64 KiB")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".open-with-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), filepath.Join(dir, "open-with.json"))
}
