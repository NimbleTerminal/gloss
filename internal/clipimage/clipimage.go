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
)

// ErrNoTool is returned, wrapped with what to install, when the system has no
// tool that can put a picture on the clipboard.
var ErrNoTool = errors.New("no tool to copy a picture to the clipboard")

// lookPath and run are variables so tests can stand in for the tools.
var (
	lookPath = exec.LookPath
	run      = func(cmd *exec.Cmd) error { return cmd.Run() }
)

// Copy puts png on the clipboard. The bytes go through a temporary file,
// removed when the tool has finished.
func Copy(ctx context.Context, png []byte) error {
	if !supported {
		return fmt.Errorf("%w", ErrNoTool)
	}
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
	if err := run(cmd); err != nil {
		return fmt.Errorf("copying the picture: %w", err)
	}
	return nil
}
