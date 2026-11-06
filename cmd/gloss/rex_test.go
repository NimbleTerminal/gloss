package main

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"nimbleterminal.dev/gloss/skills"
)

// gloss skill install names what it installs: agent, the default, and rex. A
// folder of either name is ./agent or ./rex, as a file named like a verb is.
func TestSkillInstallNames(t *testing.T) {
	for _, tt := range []struct {
		args   []string
		kind   string
		folder string
		detect bool
	}{
		{[]string{"skill", "install"}, "agent", "", true},
		{[]string{"skill", "install", "agent"}, "agent", "", true},
		{[]string{"skill", "install", "agent", "/x/skills"}, "agent", "/x/skills", false},
		{[]string{"skill", "install", "/x/skills"}, "agent", "/x/skills", false},
		{[]string{"skill", "install", "./rex"}, "agent", "./rex", false},
		{[]string{"skill", "install", "rex"}, "rex", "", true},
		{[]string{"skill", "install", "rex", "/x/rex"}, "rex", "/x/rex", false},
	} {
		opts, done, err := parse(tt.args, &bytes.Buffer{})
		if err != nil || done || opts.SkillKind != tt.kind || opts.SkillInstall != tt.folder || opts.SkillDetect != tt.detect {
			t.Errorf("%v: kind=%q folder=%q detect=%v done=%v err=%v", tt.args, opts.SkillKind, opts.SkillInstall, opts.SkillDetect, done, err)
		}
	}
	for _, args := range [][]string{
		{"skill", "install", "rex", "a", "b"},
		{"skill", "install", "agent", "a", "b"},
		{"skill", "install", "rex", ""},
		{"skill", "install", "agent", ""},
		{"skill", "install", "vim"},
	} {
		if opts, _, err := parse(args, &bytes.Buffer{}); err == nil && (opts.SkillKind != "agent" || opts.SkillInstall != "vim") {
			t.Errorf("accepted %v: %+v", args, opts)
		}
	}
	// Nothing to install leaves no kind.
	if opts, _, err := parse([]string{"skill", "show"}, &bytes.Buffer{}); err != nil || opts.SkillKind != "" {
		t.Errorf("skill show: kind=%q err=%v", opts.SkillKind, err)
	}
	var help bytes.Buffer
	parse([]string{"skill", "--help"}, &help)
	for _, line := range []string{"gloss skill install [agent] [FOLDER]", "gloss skill install rex [FOLDER]"} {
		if !strings.Contains(help.String(), line) {
			t.Errorf("the help lacks %q:\n%s", line, help.String())
		}
	}
}

// The Rex integration is two Lua files in Rex's config folder, and a line for
// init.lua that gloss does not write itself.
func TestInstallRex(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "rex")
	var stdout, stderr bytes.Buffer
	if err := installRex(dir, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	actions, companion := filepath.Join(dir, "gloss.lua"), filepath.Join(dir, "gloss-companion.lua")
	if stdout.String() != actions+"\n"+companion+"\n" {
		t.Fatalf("stdout = %q", stdout.String())
	}
	for path, want := range map[string]string{actions: skills.RexActions, companion: skills.RexCompanion} {
		if got, err := os.ReadFile(path); err != nil || string(got) != want {
			t.Fatalf("%s: %v", path, err)
		}
	}
	init := filepath.Join(dir, "init.lua")
	if !strings.Contains(stderr.String(), `require("gloss")`) || !strings.Contains(stderr.String(), init) || !strings.Contains(stderr.String(), "rex config reload") {
		t.Fatalf("stderr does not say what to add to init.lua: %q", stderr.String())
	}
	if _, err := os.Stat(init); !os.IsNotExist(err) {
		t.Fatalf("init.lua was written: %v", err)
	}
	// An init.lua that loads it already gets no hint, however it is spelled.
	for _, line := range []string{"require(\"gloss\")\n", "require 'gloss'\n", "local g = require( \"gloss\" ) -- gloss\n"} {
		if err := os.WriteFile(init, []byte("-- mine\n"+line), 0o644); err != nil {
			t.Fatal(err)
		}
		stdout.Reset()
		stderr.Reset()
		if err := installRex(dir, &stdout, &stderr); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(stderr.String(), "require") || !strings.Contains(stderr.String(), "updated") {
			t.Fatalf("%q: stderr = %q", line, stderr.String())
		}
		if got, _ := os.ReadFile(init); string(got) != "-- mine\n"+line {
			t.Fatalf("init.lua changed: %q", got)
		}
	}
	// One that mentions gloss without loading it still gets the hint.
	if err := os.WriteFile(init, []byte("-- gloss is nice\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stderr.Reset()
	if err := installRex(dir, &stdout, &stderr); err != nil || !strings.Contains(stderr.String(), `require("gloss")`) {
		t.Fatalf("err=%v stderr=%q", err, stderr.String())
	}
}

// Without a folder, the files go where Rex reads its config.
func TestRexConfigDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("REX_CONFIG", "")
	if got, err := rexConfigDir(); err != nil || got != filepath.Join(home, ".config", "rex") {
		t.Errorf("default: %q, %v", got, err)
	}
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "cfg"))
	if got, err := rexConfigDir(); err != nil || got != filepath.Join(home, "cfg", "rex") {
		t.Errorf("XDG_CONFIG_HOME: %q, %v", got, err)
	}
	t.Setenv("REX_CONFIG", filepath.Join(home, "elsewhere", "init.lua"))
	if got, err := rexConfigDir(); err != nil || got != filepath.Join(home, "elsewhere") {
		t.Errorf("REX_CONFIG: %q, %v", got, err)
	}
	t.Setenv("REX_CONFIG", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "")
	if _, err := rexConfigDir(); err == nil {
		t.Error("no home folder, no error")
	}
}

// The Lua drives gloss by its flags: every flag it names must exist, and it
// must speak to the viewer over the socket, not by keystrokes.
func TestRexLuaMatchesTheFlags(t *testing.T) {
	var help bytes.Buffer
	parse([]string{"--help"}, &help)
	for name, lua := range map[string]string{"gloss.lua": skills.RexActions, "gloss-companion.lua": skills.RexCompanion} {
		for _, flag := range regexp.MustCompile(`--[a-z][a-z-]+`).FindAllString(lua, -1) {
			if !strings.Contains(help.String(), flag) {
				t.Errorf("%s names %s, which gloss does not have", name, flag)
			}
		}
		if !strings.Contains(lua, "--remote") {
			t.Errorf("%s does not use --remote", name)
		}
		if strings.Contains(lua, "send-key") {
			t.Errorf("%s types into the viewer", name)
		}
	}
	if !strings.Contains(skills.RexCompanion, "--socket") {
		t.Error("the companion does not start the viewer with --socket")
	}
}
