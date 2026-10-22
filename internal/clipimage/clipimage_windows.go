package clipimage

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

const supported = true

// clipFileVariable carries the path to the script, which needs no quoting.
const clipFileVariable = "GLOSS_CLIP_FILE"

const clipScript = `Add-Type -AssemblyName System.Windows.Forms,System.Drawing; ` +
	`$i = [System.Drawing.Image]::FromFile($env:` + clipFileVariable + `); ` +
	`[Windows.Forms.Clipboard]::SetImage($i); $i.Dispose()`

// command sets the clipboard picture from the file. SetImage needs a
// single-threaded apartment, hence -STA.
func command(ctx context.Context, file string) (*exec.Cmd, error) {
	powershell, err := lookPath("powershell")
	if err != nil {
		return nil, fmt.Errorf("%w: powershell is missing", ErrNoTool)
	}
	cmd := exec.CommandContext(ctx, powershell, "-NoProfile", "-STA", "-Command", clipScript)
	cmd.Env = append(os.Environ(), clipFileVariable+"="+file)
	return cmd, nil
}
