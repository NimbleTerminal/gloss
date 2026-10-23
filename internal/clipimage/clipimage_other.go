//go:build !darwin && !windows && !js

package clipimage

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
)

const supported = true

// command prefers wl-copy under Wayland, else xclip. Both fork a process
// to serve the clipboard and exit, so waiting for the tool is waiting for
// that fork.
func command(ctx context.Context, file string) (*exec.Cmd, error) {
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		if wlcopy, err := lookPath("wl-copy"); err == nil {
			png, err := os.ReadFile(file)
			if err != nil {
				return nil, err
			}
			cmd := exec.CommandContext(ctx, wlcopy, "--type", "image/png")
			cmd.Stdin = bytes.NewReader(png)
			return cmd, nil
		}
	}
	if xclip, err := lookPath("xclip"); err == nil {
		return exec.CommandContext(ctx, xclip, "-selection", "clipboard", "-t", "image/png", "-i", file), nil
	}
	return nil, fmt.Errorf("%w: install wl-copy or xclip", ErrNoTool)
}
