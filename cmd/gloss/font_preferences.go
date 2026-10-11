package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"nimbleterminal.dev/gloss/internal/document"
	"nimbleterminal.dev/gloss/internal/theme"
)

type fontPreference struct {
	Font string `json:"font"`
	Mono string `json:"mono,omitempty"`
}

func saveFontChoice(name, mono string) error {
	if _, err := document.ResolveFont(name, mono); err != nil {
		return err
	}
	dir, err := theme.Dir()
	if err != nil {
		return err
	}
	return writeFontChoice(dir, name, mono)
}
func writeFontChoice(dir, name, mono string) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := json.Marshal(fontPreference{name, mono})
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".font-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), filepath.Join(dir, "font"))
}
func savedFontChoice() (string, string, error) {
	dir, err := theme.Dir()
	if err != nil {
		return "", "", err
	}
	return readFontChoice(dir)
}
func readFontChoice(dir string) (string, string, error) {
	f, err := os.Open(filepath.Join(dir, "font"))
	if err != nil {
		return "", "", err
	}
	defer f.Close()
	var p fontPreference
	dec := json.NewDecoder(io.LimitReader(f, 4097))
	dec.DisallowUnknownFields()
	if err = dec.Decode(&p); err != nil {
		return "", "", err
	}
	if err = dec.Decode(new(any)); err != io.EOF {
		return "", "", fmt.Errorf("invalid font preference")
	}
	_, err = document.ResolveFont(p.Font, p.Mono)
	return p.Font, p.Mono, err
}
