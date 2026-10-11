//go:build !js

package openwith

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

type child interface {
	Start() error
	Wait() error
}
type starter func(string, ...string) child

// BuildCommand substitutes a whole path argument without invoking a shell.
func BuildCommand(entry Entry, path string) (string, []string, error) {
	if len(entry.Exec) == 0 || entry.Exec[0] == "" || strings.Contains(entry.Exec[0], "{}") || !filepath.IsAbs(path) || strings.ContainsRune(path, 0) {
		return "", nil, fmt.Errorf("invalid open-with command or path")
	}
	args := make([]string, 0, len(entry.Exec))
	found := false
	for _, arg := range entry.Exec[1:] {
		if arg == "{}" {
			arg, found = path, true
		} else if strings.Contains(arg, "{}") {
			return "", nil, fmt.Errorf("invalid path placeholder")
		}
		args = append(args, arg)
	}
	if !found {
		args = append(args, path)
	}
	return entry.Exec[0], args, nil
}

func startWith(start starter, entry Entry, path string) error {
	name, args, err := BuildCommand(entry, path)
	if err != nil {
		return err
	}
	cmd := start(name, args...)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

// Start submits a launch and reaps the child without waiting for app exit.
func Start(entry Entry, path string) error {
	return startWith(func(name string, args ...string) child { return exec.Command(name, args...) }, entry, path)
}
