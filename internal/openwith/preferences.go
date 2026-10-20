//go:build !js

package openwith

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"nimbleterminal.dev/ntopenwith"
)

const (
	preferencesFile = "open-with-v1.json"
	legacyFile      = "open-with.json"
)

// v1Invalid marks a damaged preferences file in the current format, which
// saving must not replace.
type v1Invalid struct{ error }

func (e v1Invalid) Unwrap() error { return e.error }

// LoadPreferences reads the saved choices in dir: open-with-v1.json, else the
// open-with.json gloss wrote before ntopenwith. A file that cannot be parsed
// is reported as invalid, not fatal, and gives empty preferences; err is for
// failures to read.
func LoadPreferences(dir string) (prefs ntopenwith.Preferences, invalid error, err error) {
	v1 := filepath.Join(dir, preferencesFile)
	if _, serr := os.Stat(v1); serr == nil {
		prefs, err = ntopenwith.LoadPreferences(v1)
		if err != nil {
			prefs, invalid, err = unreadable(preferencesFile, err)
			if invalid != nil {
				invalid = v1Invalid{invalid}
			}
		}
		return prefs, invalid, err
	} else if !errors.Is(serr, fs.ErrNotExist) {
		return ntopenwith.Preferences{}, nil, serr
	}
	prefs, err = ntopenwith.LoadLegacyPreferences(filepath.Join(dir, legacyFile))
	if err != nil {
		return unreadable(legacyFile, err)
	}
	return prefs, nil, nil
}

// unreadable tells a file that could not be read from one that is damaged,
// whose error then names the file.
func unreadable(name string, err error) (ntopenwith.Preferences, error, error) {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return ntopenwith.Preferences{}, nil, err
	}
	return ntopenwith.Preferences{}, fmt.Errorf("%s: %w", name, err), nil
}

// Saver returns the function that writes changed choices to open-with-v1.json
// in dir. It refuses while that file is damaged, and leaves open-with.json as
// it is.
func Saver(dir string, invalid error) func(ntopenwith.Preferences) error {
	var v1 v1Invalid
	if errors.As(invalid, &v1) {
		return func(ntopenwith.Preferences) error {
			return fmt.Errorf("repair %s before saving: %w", preferencesFile, invalid)
		}
	}
	return func(p ntopenwith.Preferences) error {
		return p.Save(filepath.Join(dir, preferencesFile))
	}
}
