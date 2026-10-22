package clipimage

import (
	"context"
	"fmt"
	"os/exec"
)

const supported = true

// command reads the file as a PNG picture into the clipboard; the path is an
// argument of the script, so it needs no quoting.
func command(ctx context.Context, file string) (*exec.Cmd, error) {
	osascript, err := lookPath("osascript")
	if err != nil {
		return nil, fmt.Errorf("%w: osascript is missing", ErrNoTool)
	}
	return exec.CommandContext(ctx, osascript,
		"-e", "on run argv",
		"-e", "set the clipboard to (read (POSIX file (item 1 of argv)) as «class PNGf»)",
		"-e", "end run",
		file), nil
}
