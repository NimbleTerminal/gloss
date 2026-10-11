//go:build !js

package openwith

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/google/shlex"
)

// Editable reports formats whose original file is text, not a converted view.
func Editable(kind string) bool {
	switch kind {
	case "text", "markdown", "json", "html", "svg", "csv", "ipynb":
		return true
	}
	return false
}

func editorSetting() string {
	if value := strings.TrimSpace(os.Getenv("VISUAL")); value != "" {
		return value
	}
	return strings.TrimSpace(os.Getenv("EDITOR"))
}

// Editor prepares an attached process. Only the pager runs it, through
// Bubble Tea's terminal suspension; Launch never runs terminal editors.
func (h *nativeHost) Editor(t Target) (*exec.Cmd, error) {
	return editorCommand(editorSetting(), t)
}

func editorCommand(setting string, t Target) (*exec.Cmd, error) {
	if t.Directory || !Editable(t.Kind) {
		return nil, fmt.Errorf("terminal editing requires a text file")
	}
	path, err := filepath.Abs(t.Path)
	if err != nil || t.Path == "" {
		return nil, fmt.Errorf("invalid editor file path")
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("terminal editing requires a regular file")
	}
	argv, err := editorArguments(setting)
	if err != nil {
		return nil, err
	}
	return exec.Command(argv[0], append(argv[1:], path)...), nil
}

func editorArguments(setting string) ([]string, error) {
	if setting == "" {
		return nil, fmt.Errorf("set VISUAL or EDITOR to a terminal editor, for example EDITOR=nano")
	}
	if len(setting) > 8192 || strings.ContainsRune(setting, 0) {
		return nil, fmt.Errorf("editor setting exceeds 8192 bytes or contains NUL")
	}
	// An exact executable path can contain spaces or Windows backslashes.
	// Otherwise parse arguments with POSIX quoting, without any expansions.
	var argv []string
	var err error
	if executable, err := exec.LookPath(setting); err == nil {
		argv = []string{executable}
	} else {
		argv, err = shlex.Split(setting)
		if err != nil {
			return nil, fmt.Errorf("invalid editor setting: %w", err)
		}
	}
	if len(argv) == 0 || argv[0] == "" || len(argv) > 64 {
		return nil, fmt.Errorf("editor setting needs an executable and at most 64 words")
	}
	executable, err := exec.LookPath(argv[0])
	if err != nil {
		return nil, fmt.Errorf("terminal editor: %w", err)
	}
	argv[0] = executable
	return argv, nil
}
