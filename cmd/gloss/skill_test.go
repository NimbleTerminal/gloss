package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nimbleterminal.dev/gloss/skills"
)

func TestSkillInstall(t *testing.T) {
	// The old --install needs the old --skill.
	if _, _, err := parse([]string{"--install"}, &bytes.Buffer{}); err == nil {
		t.Fatal("--install alone was accepted")
	}
	// A named skills folder gets gloss/SKILL.md, whole; the path is the answer.
	dir := t.TempDir()
	root := filepath.Join(dir, "skills")
	for _, args := range [][]string{{"skill", "install", root}, {"--skill", "--install=" + root}} {
		opts, done, err := parse(args, &bytes.Buffer{})
		if err != nil || done || opts.SkillInstall != root {
			t.Fatalf("parse %v = %+v, done=%v, %v", args, opts, done, err)
		}
	}
	opts := options{SkillInstall: root}
	var stdout, stderr bytes.Buffer
	if err := installSkill(opts.SkillInstall, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	written := filepath.Join(root, "gloss", "SKILL.md")
	if strings.TrimSpace(stdout.String()) != written {
		t.Fatalf("stdout = %q, want %q", stdout.String(), written)
	}
	content, err := os.ReadFile(written)
	if err != nil || string(content) != skills.Gloss {
		t.Fatalf("%s: %v", written, err)
	}
	// Again, and it is an update, said as such.
	stderr.Reset()
	if err := installSkill(root, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr.String(), "updated") {
		t.Fatalf("reinstall not called an update: %q", stderr.String())
	}
}

func TestSkillInstallAuto(t *testing.T) {
	// Detection looks in the home folder; give it one, with Claude Code in it.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("KIMI_CODE_HOME", "")
	if err := os.Mkdir(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if err := installSkill("auto", &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	claude := filepath.Join(home, ".claude", "skills", "gloss", "SKILL.md")
	shared := filepath.Join(home, ".agents", "skills", "gloss", "SKILL.md")
	if stdout.String() != claude+"\n"+shared+"\n" || !strings.Contains(stderr.String(), "Claude Code") || !strings.Contains(stderr.String(), "Codex") {
		t.Fatalf("stdout = %q, stderr = %q", stdout.String(), stderr.String())
	}
	for _, path := range []string{claude, shared} {
		if _, err := os.Stat(path); err != nil {
			t.Fatal(err)
		}
	}
	// Kimi Code's home, moved by KIMI_CODE_HOME, is found as well.
	kimi := filepath.Join(home, "kimi-home")
	t.Setenv("KIMI_CODE_HOME", kimi)
	if err := os.Mkdir(kimi, 0o755); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	if err := installSkill("auto", &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), filepath.Join(kimi, "skills", "gloss", "SKILL.md")) {
		t.Fatalf("Kimi Code's folder missed: %q", stdout.String())
	}
}

func TestSkillInstallSharedAlways(t *testing.T) {
	// The shared folder is written whether or not anything is found: the
	// agents that read it, Codex among them, do not make it (issue 6).
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("KIMI_CODE_HOME", "")
	var stdout, stderr bytes.Buffer
	if err := installSkill("auto", &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	shared := filepath.Join(home, ".agents", "skills", "gloss", "SKILL.md")
	if strings.TrimSpace(stdout.String()) != shared || !strings.Contains(stderr.String(), "Codex") {
		t.Fatalf("stdout = %q, stderr = %q", stdout.String(), stderr.String())
	}
	if content, err := os.ReadFile(shared); err != nil || string(content) != skills.Gloss {
		t.Fatalf("%s: %v", shared, err)
	}
}

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip(err)
	}
	if expandHome("~/skills") != filepath.Join(home, "skills") || expandHome("~") != home || expandHome("/tmp/x") != "/tmp/x" {
		t.Fatalf("expandHome: ~=%q ~/skills=%q", expandHome("~"), expandHome("~/skills"))
	}
}

func TestSkillVerb(t *testing.T) {
	// Bare install looks for the agents here.
	opts, done, err := parse([]string{"skill", "install"}, &bytes.Buffer{})
	if err != nil || done || opts.SkillInstall != "auto" {
		t.Fatalf("skill install = %+v, done=%v, %v", opts, done, err)
	}
	for _, args := range [][]string{{"skill", "uninstall"}, {"skill", "show", "x"}, {"skill", "install", "a", "b"}, {"skill", "--bogus"}} {
		if _, _, err := parse(args, &bytes.Buffer{}); err == nil {
			t.Errorf("accepted %v", args)
		}
	}
	// Help is help after the subcommand too (issue 6: gloss skill install
	// --help once installed the skill under a folder named --help).
	for _, args := range [][]string{{"skill", "--help"}, {"skill", "-h"}, {"skill", "install", "--help"}, {"skill", "install", "-h"}, {"skill", "show", "--help"}, {"skill", "schema", "--help"}} {
		var out bytes.Buffer
		opts, done, err := parse(args, &out)
		if err != nil || !done || opts.SkillInstall != "" || !strings.Contains(out.String(), "gloss skill install [FOLDER]") {
			t.Fatalf("%v: done=%v err=%v opts=%+v\n%s", args, done, err, opts, out.String())
		}
	}
	// An option it does not know is not a folder either.
	for _, args := range [][]string{{"skill", "install", "--bogus"}, {"skill", "install", "-x"}, {"skill", "show", "--bogus"}} {
		if opts, _, err := parse(args, &bytes.Buffer{}); err == nil || opts.SkillInstall != "" {
			t.Errorf("accepted %v: opts=%+v err=%v", args, opts, err)
		}
	}
	// The old options are not in the help, so they cannot be learned from it.
	var help bytes.Buffer
	parse([]string{"--help"}, &help)
	if strings.Contains(help.String(), "--skill") || strings.Contains(help.String(), "--install") {
		t.Fatalf("--help lists the options the skill verb replaced:\n%s", help.String())
	}
}
