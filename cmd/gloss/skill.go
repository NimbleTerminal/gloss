package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"nimbleterminal.dev/gloss/skills"
)

// skillHome is a skills folder an agent reads, detected by the presence of
// detect: the agent's home. The shared folder has no detect: agents that read
// it, Codex among them, do not make it, so it is always written.
type skillHome struct {
	agent, detect, root string
}

// skillHomes are the user-level skills folders gloss knows, in the order they
// are reported.
func skillHomes() []skillHome {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	kimi := os.Getenv("KIMI_CODE_HOME")
	if kimi == "" {
		kimi = filepath.Join(home, ".kimi-code")
	}
	shared := filepath.Join(home, ".agents", "skills")
	return []skillHome{
		{"Claude Code", filepath.Join(home, ".claude"), filepath.Join(home, ".claude", "skills")},
		{"Kimi Code", kimi, filepath.Join(kimi, "skills")},
		{"shared agents folder", "", shared},
	}
}

// installSkill writes the embedded skill as gloss/SKILL.md under the folder
// named or, when where is empty, under the shared agents folder and the folder
// of each agent found on this machine. The paths written go to stdout, one to
// a line; who each is for goes to stderr. A skill already there is updated: it
// is meant to match the binary.
func installSkill(where string, stdout, stderr io.Writer) error {
	var homes []skillHome
	if where == "" {
		for _, h := range skillHomes() {
			if h.detect == "" {
				homes = append(homes, h)
			} else if _, err := os.Stat(h.detect); err == nil {
				homes = append(homes, h)
			}
		}
		if len(homes) == 0 {
			return errors.New("no home folder to install under; name one: gloss skill install $HOME/.agents/skills")
		}
	} else {
		root, err := expandHome(where)
		if err != nil {
			return err
		}
		homes = []skillHome{{root: root}}
	}
	for _, h := range homes {
		dir := filepath.Join(h.root, "gloss")
		if err := clearDanglingLink(dir, stderr); err != nil {
			return err
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		path := filepath.Join(dir, "SKILL.md")
		verb := "installed"
		if _, err := os.Stat(path); err == nil {
			verb = "updated"
		}
		if err := writeFileAtomic(path, []byte(skills.Gloss), 0o644); err != nil {
			return err
		}
		if h.agent != "" {
			fmt.Fprintf(stderr, "gloss: skill %s for %s: %s\n", verb, h.agent, path)
		} else {
			fmt.Fprintf(stderr, "gloss: skill %s: %s\n", verb, path)
		}
		fmt.Fprintln(stdout, path)
	}
	return nil
}

// clearDanglingLink removes dir when it is a symbolic link to nothing, such as
// one left by a moved checkout, so a real folder can take its place. A link to
// a folder that exists was made on purpose, and one whose target cannot be
// checked may have been: neither is replaced nor written through.
func clearDanglingLink(dir string, stderr io.Writer) error {
	info, err := os.Lstat(dir)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		return nil
	}
	target, err := os.Readlink(dir)
	if err != nil {
		return err
	}
	if _, err := os.Stat(dir); err == nil {
		return fmt.Errorf("%s is a link to %s; remove it to install the skill from this binary, or keep it to use that copy", dir, target)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("%s is a link to %s, which cannot be checked: %w", dir, target, err)
	}
	if err := os.Remove(dir); err != nil {
		return err
	}
	fmt.Fprintf(stderr, "gloss: replaced a broken link to %s: %s\n", target, dir)
	return nil
}

// writeFileAtomic writes the file whole or not at all: to a temporary file in
// the same folder, then renamed over the name. A name that is a symbolic link
// is replaced, and what it pointed at is left alone.
func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".gloss-skill-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // Gone once renamed; otherwise, not left behind.
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := errors.Join(tmp.Chmod(mode), tmp.Close()); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// expandHome resolves a leading ~/ , which the shell leaves alone after =.
// Without a home folder it is an error, not a folder named ~.
func expandHome(path string) (string, error) {
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot resolve %s: %w", path, err)
	}
	return filepath.Join(home, path[1:]), nil
}
