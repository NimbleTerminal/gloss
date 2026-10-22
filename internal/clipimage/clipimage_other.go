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

// command prefers wl-copy under Wayland, else xclip. Both leave a process
// behind to serve the clipboard, so neither is waited for beyond its start.
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
