package handoff

import (
	"context"
	"io"
	"path/filepath"
	"time"
)

// DefaultListen is where a --pick-web request listens unless told otherwise:
// loopback, on a port the system chooses.
const DefaultListen = defaultPickListen

// ValidToken says whether s has the form of a session token, as --resume,
// --status and --cancel name one.
func ValidToken(s string) bool { return tokenPattern.MatchString(s) }

// CheckNetwork reports what is wrong with a --listen and --advertise-host pair.
func CheckNetwork(listen, advertised string) error {
	_, err := parsePickNetwork(listen, advertised)
	return err
}

// Start opens the page cfg asks for, a viewer or the upload-only request,
// and sets its lifetime: a show-only viewer started apart from a terminal
// renews an idle window, and a detached session's drops, address and
// cancellation follow its token. end stops the session; the page closes when
// ctx is done.
func Start(ctx context.Context, end context.CancelFunc, cfg Config) (*Server, error) {
	var s *Server
	var err error
	if cfg.PickWeb {
		s, err = serveWebPick(ctx, cfg)
	} else {
		s, err = serve(ctx, cfg.Options)
	}
	if err != nil {
		return nil, err
	}
	if !cfg.PickWeb {
		idle := time.Duration(0)
		var save func(time.Time)
		if cfg.Detached != "" && !cfg.Pick && cfg.Timeout == 0 {
			idle = defaultDetachedTimeout
			dir, _ := detachedPaths(cfg.Detached)
			// The lease lives inside the existing private folder. A renewal
			// cannot recreate the session state removed by --cancel.
			save = func(deadline time.Time) {
				_ = writeState(filepath.Join(dir, "viewer-lease.json"), state{Deadline: deadline})
			}
		}
		s.lifetime.configure(cfg.Timeout, idle, save)
	}
	if cfg.Detached != "" {
		// Drops land in the folder the start made, and its state says where we are.
		dir, _ := detachedPaths(cfg.Detached)
		s.useFolder(dir)
		announce(cfg.Detached, s.URL)
		go watchCancel(cfg.Detached, end, ctx.Done())
	}
	go func() {
		<-ctx.Done()
		s.Close()
		s.ended.Do(func() { close(s.done) })
	}()
	return s, nil
}

// WaitWebPick waits for the person answering an upload-only request to send
// their files, or to decline. A network request shows its address as a QR
// code on the terminal while it waits, when there is a terminal to show it.
func (s *Server) WaitWebPick(ctx context.Context, cfg Config, stderr io.Writer) (paths []string, message string, err error) {
	if pickQRAvailable(cfg, stderr) {
		return waitWebPickQR(ctx, s, cfg, stderr)
	}
	return s.waitWebPick(ctx, cfg.Timeout)
}

// AnswerPicked writes a pick's answer as --resume would: the paths, one to a
// line, or the JSON object with the requester's message.
func AnswerPicked(paths []string, message string, asJSON bool, stdout io.Writer) error {
	return answer(state{Status: statusPicked, Paths: paths, Message: message}, asJSON, stdout)
}
