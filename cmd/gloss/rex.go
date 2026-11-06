package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"nimbleterminal.dev/gloss/skills"
)

// rexFiles are what gloss skill install rex writes, in the order reported.
var rexFiles = []struct {
	name, content string
	mode          os.FileMode
}{
	{"gloss.lua", skills.RexActions, 0o644},
	{"gloss-companion.lua", skills.RexCompanion, 0o644},
	{"grex", skills.RexGrex, 0o755},
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

// installRex writes the Rex integration, gloss.lua, gloss-companion.lua, and
// grex, into the folder named or, when where is empty, into Rex's config folder
// (and grex into ~/.local/bin), adds require("gloss") to init.lua once when it
// is not there yet, and has a running Rex reload its config. The paths written
// go to stdout, one to a line; what was done goes to stderr.
func installRex(where string, stdout, stderr io.Writer) error {
	dir, err := rexConfigDir()
	if where != "" {
		dir, err = expandHome(where)
	}
	if err != nil {
		return err
	}
	for _, f := range rexFiles {
		if err := writeRexFile(filepath.Join(dir, f.name), f.content, f.mode, stdout, stderr); err != nil {
			return err
		}
	}
	if where == "" && runtime.GOOS != "windows" {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			bin := filepath.Join(home, ".local", "bin", "grex")
			if bin != filepath.Join(dir, "grex") {
				if err := writeRexFile(bin, skills.RexGrex, 0o755, stdout, stderr); err != nil {
					return err
				}
			}
		}
	}
	init := filepath.Join(dir, "init.lua")
	added, err := requireGloss(init)
	if err != nil {
		return err
	}
	if added {
		fmt.Fprintf(stderr, "gloss: added require(\"gloss\") to %s\n", init)
		fmt.Fprintln(stdout, init)
	}
	reloadRex(stderr)
	return nil
}

func writeRexFile(path, content string, mode os.FileMode, stdout, stderr io.Writer) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	verb := "installed"
	if _, err := os.Stat(path); err == nil {
		verb = "updated"
	}
	if err := writeFileAtomic(path, []byte(content), mode); err != nil {
		return err
	}
	fmt.Fprintf(stderr, "gloss: Rex integration %s: %s\n", verb, path)
	fmt.Fprintln(stdout, path)
	return nil
}

// requireGloss adds the line that loads gloss.lua to the end of init.lua, when
// no line does yet, making the file when there is none. The file is the user's:
// nothing in it is changed, and the line is added once.
func requireGloss(init string) (added bool, err error) {
	content, err := os.ReadFile(init)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	if requiresGloss.Match(content) {
		return false, nil
	}
	f, err := os.OpenFile(init, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return false, err
	}
	var line string
	if len(content) > 0 && content[len(content)-1] != '\n' {
		line = "\n"
	}
	if len(content) > 0 {
		line += "\n"
	}
	line += "-- gloss: the actions and key that gloss skill install rex wrote, in gloss.lua.\nrequire(\"gloss\")\n"
	_, err = f.WriteString(line)
	return true, errors.Join(err, f.Close())
}

// reloadRex has a running Rex read its config again, when the rex command is
// here to ask; otherwise, or when it cannot, it says what to run. Rex is not
// started for it.
func reloadRex(stderr io.Writer) {
	rex, err := exec.LookPath("rex")
	if err != nil {
		fmt.Fprintln(stderr, "gloss: run `rex config reload` for Rex to pick this up")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, rex, "config", "reload", "--autostart=false").CombinedOutput()
	if err != nil {
		reason := strings.TrimSpace(string(out))
		if reason == "" {
			reason = err.Error()
		}
		fmt.Fprintf(stderr, "gloss: rex config reload: %s\n", reason)
		fmt.Fprintln(stderr, "gloss: run `rex config reload` once Rex is running")
		return
	}
	fmt.Fprintln(stderr, "gloss: Rex reloaded its config (rex config reload)")
}
