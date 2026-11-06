package main

import (
	"bytes"
	"os"
	"os/exec"
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

// The Rex integration is two Lua files in Rex's config folder, and one line
// added to init.lua, once, which is made when it is missing.
func TestInstallRex(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // No rex here: the reload is left to the user.
	dir := filepath.Join(t.TempDir(), "rex")
	var stdout, stderr bytes.Buffer
	if err := installRex(dir, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	actions, companion, grex, init := filepath.Join(dir, "gloss.lua"), filepath.Join(dir, "gloss-companion.lua"), filepath.Join(dir, "grex"), filepath.Join(dir, "init.lua")
	if stdout.String() != actions+"\n"+companion+"\n"+grex+"\n"+init+"\n" {
		t.Fatalf("stdout = %q", stdout.String())
	}
	for path, want := range map[string]string{actions: skills.RexActions, companion: skills.RexCompanion, grex: skills.RexGrex} {
		if got, err := os.ReadFile(path); err != nil || string(got) != want {
			t.Fatalf("%s: %v", path, err)
		}
	}
	if info, err := os.Stat(grex); err != nil || info.Mode()&0o111 == 0 {
		t.Fatalf("grex mode = %v, %v", info, err)
	}
	made, err := os.ReadFile(init)
	if err != nil || !requiresGloss.Match(made) || !strings.HasSuffix(string(made), "\n") {
		t.Fatalf("init.lua = %q, %v", made, err)
	}
	if !strings.Contains(stderr.String(), `require("gloss")`) || !strings.Contains(stderr.String(), init) || !strings.Contains(stderr.String(), "rex config reload") {
		t.Fatalf("stderr does not say what was added or what to run: %q", stderr.String())
	}
	// An init.lua that loads it already is left as it is, however it spells
	// the require, and only the update of the two files is reported.
	for _, line := range []string{"require(\"gloss\")\n", "require 'gloss'\n", "local g = require( \"gloss\" ) -- gloss\n"} {
		if err := os.WriteFile(init, []byte("-- mine\n"+line), 0o644); err != nil {
			t.Fatal(err)
		}
		stdout.Reset()
		stderr.Reset()
		if err := installRex(dir, &stdout, &stderr); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(stderr.String(), "require") || !strings.Contains(stderr.String(), "updated") || strings.Contains(stdout.String(), init) {
			t.Fatalf("%q: stdout = %q stderr = %q", line, stdout.String(), stderr.String())
		}
		if got, _ := os.ReadFile(init); string(got) != "-- mine\n"+line {
			t.Fatalf("init.lua changed: %q", got)
		}
	}
	// One with other content, mentioning gloss or not, and no final newline,
	// gets the line added after what is there, which stays whole.
	if err := os.WriteFile(init, []byte("-- gloss is nice\nrex.bind(\"cmd+d\", \"pane.split\")"), 0o644); err != nil {
		t.Fatal(err)
	}
	stderr.Reset()
	if err := installRex(dir, &stdout, &stderr); err != nil || !strings.Contains(stderr.String(), `require("gloss")`) {
		t.Fatalf("err=%v stderr=%q", err, stderr.String())
	}
	got, _ := os.ReadFile(init)
	if !strings.HasPrefix(string(got), "-- gloss is nice\nrex.bind(\"cmd+d\", \"pane.split\")\n") || !requiresGloss.Match(got) || strings.Count(string(got), "require") != 1 {
		t.Fatalf("init.lua = %q", got)
	}
	// Once more, and nothing is added again.
	if err := installRex(dir, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if again, _ := os.ReadFile(init); string(again) != string(got) {
		t.Fatalf("init.lua changed again: %q", again)
	}
}

// With rex on the PATH, the config is reloaded for the user; what rex says
// when that fails is passed on, and the install still succeeds.
func TestInstallRexReloads(t *testing.T) {
	bin := t.TempDir()
	t.Setenv("PATH", bin)
	calls := filepath.Join(bin, "calls")
	fake := "#!/bin/sh\necho \"$@\" >> " + calls + "\n"
	if err := os.WriteFile(filepath.Join(bin, "rex"), []byte(fake), 0o755); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "rex")
	var stdout, stderr bytes.Buffer
	if err := installRex(dir, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(calls); string(got) != "config reload --autostart=false\n" {
		t.Fatalf("rex was run with %q", got)
	}
	if !strings.Contains(stderr.String(), "reloaded") || strings.Contains(stderr.String(), "then run") {
		t.Fatalf("stderr = %q", stderr.String())
	}
	// A rex that fails, as when no server runs, is quoted, and the user told.
	if err := os.WriteFile(filepath.Join(bin, "rex"), []byte("#!/bin/sh\necho 'Error: no server is running' >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	stderr.Reset()
	if err := installRex(dir, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr.String(), "no server is running") || !strings.Contains(stderr.String(), "rex config reload") {
		t.Fatalf("stderr = %q", stderr.String())
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

// grex splits in the direction named (right by default) and passes the
// remaining arguments to gloss.
func TestGrexSplitsAndForwardsArguments(t *testing.T) {
	bin := t.TempDir()
	calls := filepath.Join(bin, "calls")
	fake := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> " + calls + "\n"
	if err := os.WriteFile(filepath.Join(bin, "rex"), []byte(fake), 0o755); err != nil {
		t.Fatal(err)
	}
	grex := filepath.Join(bin, "grex")
	if err := os.WriteFile(grex, []byte(skills.RexGrex), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":/usr/bin:/bin")
	t.Setenv("REX", filepath.Join(bin, "rex"))
	t.Setenv("GLOSS", "/custom/gloss")
	wd := t.TempDir()
	for _, tt := range []struct {
		args []string
		want string
	}{
		{[]string{"a.pdf", "--page", "3"}, "split --split=right -c " + wd + " -- /custom/gloss a.pdf --page 3"},
		{[]string{"left", "a.pdf", "--page", "3"}, "split --split=left -c " + wd + " -- /custom/gloss a.pdf --page 3"},
		{[]string{"below", "--menu", "a.png", "b.png"}, "split --split=below -c " + wd + " -- /custom/gloss --menu a.png b.png"},
		{[]string{"up", "notes.md"}, "split --split=above -c " + wd + " -- /custom/gloss notes.md"},
		{[]string{"down"}, "split --split=below -c " + wd + " -- /custom/gloss"},
	} {
		_ = os.Remove(calls)
		cmd := exec.Command(grex, tt.args...)
		cmd.Dir = wd
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %v\n%s", tt.args, err, out)
		}
		got, err := os.ReadFile(calls)
		if err != nil {
			t.Fatal(err)
		}
		if strings.TrimSpace(string(got)) != tt.want {
			t.Errorf("%v: got %q, want %q", tt.args, strings.TrimSpace(string(got)), tt.want)
		}
	}

	// When GLOSS is unset and PATH has a pre-0.6 gloss (without --socket),
	// grex skips it for the 0.6+ binary in ~/go/bin/gloss, matching gloss.lua.
	t.Setenv("GLOSS", "")
	home := t.TempDir()
	t.Setenv("HOME", home)
	oldGloss := filepath.Join(bin, "gloss")
	if err := os.WriteFile(oldGloss, []byte("#!/bin/sh\necho 'Usage: gloss [file]'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	goBin := filepath.Join(home, "go", "bin")
	if err := os.MkdirAll(goBin, 0o755); err != nil {
		t.Fatal(err)
	}
	newGloss := filepath.Join(goBin, "gloss")
	if err := os.WriteFile(newGloss, []byte("#!/bin/sh\necho '  --socket string'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(calls)
	cmd := exec.Command(grex, "model.3mf")
	cmd.Dir = wd
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("fallback: %v\n%s", err, out)
	}
	got, err := os.ReadFile(calls)
	if err != nil {
		t.Fatal(err)
	}
	want := "split --split=right -c " + wd + " -- " + newGloss + " model.3mf"
	if strings.TrimSpace(string(got)) != want {
		t.Errorf("fallback: got %q, want %q", strings.TrimSpace(string(got)), want)
	}
}
