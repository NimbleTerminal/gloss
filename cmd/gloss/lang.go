package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"nimbleterminal.dev/gloss/internal/i18n"
	"nimbleterminal.dev/gloss/internal/theme"
)

// language is the tag gloss's own words follow: --lang when given, else, in
// the viewer, the environment's (GLOSS_LANG, then the choice saved by the
// language picker, then LC_ALL, LC_MESSAGES, LANG). What agents read stays
// English: the headless modes and a detached start or server never look at
// the environment, and a detached page follows the browser of the human who
// opens it. Only the terminal viewer asks saved, which may be nil, for the
// saved choice ("" when none); a --serve page never does. "" is English.
func language(opts options, stdinTTY bool, getenv func(string) string, saved func() string) string {
	switch {
	case opts.Lang != "":
		return opts.Lang
	case opts.Output != "" || opts.OutputDir != "" || opts.Info || opts.Apps || opts.Grep != nil || opts.Text || opts.Print || opts.SkillDetect || opts.SkillInstall != "":
		return ""
	case opts.Detached != "" || (opts.Pick || opts.Serve || opts.PickWeb) && !stdinTTY:
		return ""
	}
	if tag, err := i18n.Parse(getenv("GLOSS_LANG")); err == nil {
		return tag.String()
	}
	if saved != nil && !opts.Serve && !opts.PickWeb {
		if tag := saved(); tag != "" {
			return tag
		}
	}
	return i18n.FromEnv(getenv)
}

type langPreference struct {
	Lang string `json:"lang"`
}

// saveLangChoice keeps the language picker's choice; "" (System) forgets it.
func saveLangChoice(tag string) error {
	dir, err := theme.Dir()
	if err != nil {
		return err
	}
	return writeLangChoice(dir, tag)
}

func writeLangChoice(dir, tag string) error {
	path := filepath.Join(dir, "lang")
	if tag == "" {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	parsed, err := i18n.Parse(tag)
	if err != nil {
		return fmt.Errorf("%q: not a language tag", tag)
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := json.Marshal(langPreference{parsed.String()})
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".lang-*")
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
	return os.Rename(f.Name(), path)
}

// savedLangChoice is the saved choice, or "" when there is none or it is
// unreadable: a bad file is no reason not to start.
func savedLangChoice() string {
	dir, err := theme.Dir()
	if err != nil {
		return ""
	}
	return readLangChoice(dir)
}

func readLangChoice(dir string) string {
	f, err := os.Open(filepath.Join(dir, "lang"))
	if err != nil {
		return ""
	}
	defer f.Close()
	var p langPreference
	dec := json.NewDecoder(io.LimitReader(f, 4097))
	dec.DisallowUnknownFields()
	if dec.Decode(&p) != nil || dec.Decode(new(any)) != io.EOF {
		return ""
	}
	tag, err := i18n.Parse(p.Lang)
	if err != nil {
		return ""
	}
	return tag.String()
}
