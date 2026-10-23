// Package clipimage puts a PNG on the system clipboard as a picture, by the
// tool the operating system provides: osascript on macOS, wl-copy or xclip on
// Linux and the BSDs, PowerShell on Windows. The browser build has none.
package clipimage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// ErrNoTool is returned, wrapped with what to install where there is a tool
// to install, when the system has no tool that can put a picture on the
// clipboard.
var ErrNoTool = errors.New("no tool to copy a picture to the clipboard")

// lookPath and run are variables so tests can stand in for the tools.
var (
	lookPath = exec.LookPath
	run      = func(cmd *exec.Cmd) error { return cmd.Run() }
)

// timeout bounds a tool that hangs, as osascript can waiting on a dialog.
const timeout = 10 * time.Second

// Copy puts png on the clipboard. The bytes go through a temporary file,
// removed when the tool has finished.
func Copy(ctx context.Context, png []byte) error {
	if !supported {
		return ErrNoTool
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	f, err := os.CreateTemp("", "gloss-clip-*.png")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	_, err = f.Write(png)
	if err := errors.Join(err, f.Close()); err != nil {
		return err
	}
	cmd, err := command(ctx, f.Name())
	if err != nil {
		return err
	}
	// What the tool says goes to a file, not a pipe: wl-copy and xclip
	// leave a process behind holding it open, which a pipe would wait for.
	stderr, err := os.CreateTemp("", "gloss-clip-*.txt")
	if err != nil {
		return err
	}
	defer os.Remove(stderr.Name())
	defer stderr.Close()
	cmd.Stderr = stderr
	if err := run(cmd); err != nil {
		if why := firstLine(stderr); why != "" {
			return fmt.Errorf("copying the picture: %w: %s", err, why)
		}
		return fmt.Errorf("copying the picture: %w", err)
	}
	return nil
}

// firstLine is the first line of what the tool wrote, at most 200 bytes of
// it.
func firstLine(f *os.File) string {
	var b [200]byte
	n, _ := f.ReadAt(b[:], 0)
	line, _, _ := strings.Cut(string(b[:n]), "\n")
	return strings.ToValidUTF8(strings.TrimSpace(line), "")
}
