package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"

	"nimbleterminal.dev/gloss/skills"
)

// rexFiles are what gloss skill install rex writes, in the order reported.
var rexFiles = []struct{ name, content string }{
	{"gloss.lua", skills.RexActions},
	{"gloss-companion.lua", skills.RexCompanion},
}

// requiresGloss matches an init.lua line that loads gloss.lua, however the
// require is spelled: require("gloss"), require 'gloss', require( "gloss" ).
var requiresGloss = regexp.MustCompile(`require\s*\(?\s*["']gloss["']`)

// rexConfigDir is the folder Rex reads init.lua from: the folder of REX_CONFIG
// when set, else rex under XDG_CONFIG_HOME or ~/.config (AppData on Windows).
func rexConfigDir() (string, error) {
	if config := os.Getenv("REX_CONFIG"); config != "" {
		return filepath.Dir(config), nil
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "rex"), nil
	}
	if runtime.GOOS == "windows" {
		if appData := os.Getenv("AppData"); appData != "" {
			return filepath.Join(appData, "rex"), nil
		}
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", errors.New("no home folder to find Rex's config under; name the folder: gloss skill install rex ~/.config/rex")
	}
	return filepath.Join(home, ".config", "rex"), nil
}

// installRex writes the Rex integration, gloss.lua and gloss-companion.lua,
// into the folder named or, when where is empty, into Rex's config folder.
// The paths written go to stdout, one to a line; what was done goes to stderr,
// with the line to add to init.lua when it does not load gloss.lua yet. gloss
// never writes init.lua: it is the user's.
func installRex(where string, stdout, stderr io.Writer) error {
	dir, err := rexConfigDir()
	if where != "" {
		dir, err = expandHome(where)
	}
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, f := range rexFiles {
		path := filepath.Join(dir, f.name)
		verb := "installed"
		if _, err := os.Stat(path); err == nil {
			verb = "updated"
		}
		if err := writeFileAtomic(path, []byte(f.content), 0o644); err != nil {
			return err
		}
		fmt.Fprintf(stderr, "gloss: Rex integration %s: %s\n", verb, path)
		fmt.Fprintln(stdout, path)
	}
	init := filepath.Join(dir, "init.lua")
	if content, err := os.ReadFile(init); err == nil && requiresGloss.Match(content) {
		fmt.Fprintln(stderr, "gloss: run `rex config reload` to pick up the change")
		return nil
	}
	fmt.Fprintf(stderr, "gloss: add this line to %s, then run `rex config reload`:\n  require(\"gloss\")\n", init)
	return nil
}
