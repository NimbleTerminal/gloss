// Package handoff is gloss's localhost handoff protocol: the temporary,
// token-guarded page that shows files or asks a person for them (--serve,
// --pick, --pick-web), the detached sessions an agent starts without a
// terminal and later asks after (--status, --resume, --cancel), the upload
// quotas, the viewer's idle lifetime, and the terminal QR of a network
// request. Its JSON results and exit codes are gloss's contract with the
// agents that call it, described by gloss skill schema.
package handoff

import (
	"context"
	"io"
	"time"

	"nimbleterminal.dev/gloss/internal/app"
)

// Config is what a handoff takes from gloss's command line.
type Config struct {
	app.Options // The page's viewer: what it shows, accepts, and asks.

	Listen, AdvertiseHost string        // Where a --pick-web request listens, and the host its address names.
	Timeout               time.Duration // How long a pick waits, or a page is kept; zero for the defaults.
	JSON                  bool          // Answer as JSON rather than as lines of paths.
	PickWeb               bool          // The upload-only request page, with no terminal viewer.

	Detached string // Set on the server a detached start runs: its token.
	Resume   string // Token of a detached session whose answer is wanted.
	Status   string // Token of a detached session whose state is wanted, now.
	Cancel   string // Token of a detached session to end, files and all.

	// Arguments readies a detached start's file arguments inside its private
	// folder before the server runs apart: stdin is saved there and addresses
	// downloaded, so the server needs neither.
	Arguments func(ctx context.Context, args []string, dir string, stdin io.Reader) ([]string, error)
}
