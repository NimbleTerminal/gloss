package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	tea "charm.land/bubbletea/v2"
	"nimbleterminal.dev/gloss/internal/app"
	"nimbleterminal.dev/gloss/internal/remote"
)

// A viewer run with --socket NAME answers other programs on a Unix socket,
// and gloss --remote NAME is the program that asks: it opens files in that
// viewer, whatever it is doing, or has it reload or quit, or asks what it
// shows. An editor or a shell script needs nothing but gloss itself. The
// wire shapes are in internal/remote.

// replyWait is how long the socket server gives the viewer to answer a
// request; the viewer answers an open once it has probed the files.
const replyWait = 30 * time.Second

// listenRemote opens the viewer's socket. Each request is sent to the
// program as an app.RemoteMsg, and its answer waits for the viewer's reply.
func listenRemote(name string, send func(tea.Msg)) (*remote.Server, error) {
	path, err := remote.SocketPath(name)
	if err != nil {
		return nil, fmt.Errorf("--socket: %w", err)
	}
	return remote.Listen(path, func(req remote.Request) remote.Response {
		reply := make(chan remote.Response, 1)
		send(app.RemoteMsg{Request: req, Reply: reply})
		select {
		case r := <-reply:
			return r
		case <-time.After(replyWait):
			return remote.Response{Error: "the viewer did not answer in time"}
		}
	})
}

// sendRemote is gloss as a client of a viewer run with --socket. It asks the
// viewer to open the files named, to reload or quit, or how it stands; a
// request the viewer refuses is an error here, with its reason.
func sendRemote(opts options, stdout, stderr io.Writer) error {
	path, err := remote.SocketPath(opts.Remote)
	if err != nil {
		return fmt.Errorf("--remote: %w", err)
	}
	req := remote.Request{Action: "status"}
	switch {
	case opts.Reload:
		req.Action = "reload"
	case opts.Quit:
		req.Action = "quit"
	case len(opts.Files) > 0:
		req.Action = "open"
		if opts.pageExplicit {
			req.Page = opts.Page
		}
		for _, name := range opts.Files {
			if webAddress(name) {
				req.Paths = append(req.Paths, name)
				continue
			}
			// The viewer takes absolute paths; a file that is not there is
			// said so here, before it is asked.
			full, err := filepath.Abs(name)
			if err != nil {
				return err
			}
			if _, err := os.Stat(full); err != nil {
				return err
			}
			req.Paths = append(req.Paths, full)
		}
	}
	resp, err := remote.Send(path, req, 0)
	if err != nil {
		return err
	}
	if opts.JSON {
		enc := json.NewEncoder(stdout)
		if err := enc.Encode(resp); err != nil {
			return err
		}
		if !resp.OK {
			return reported{fmt.Errorf("%s", resp.Error)}
		}
		return nil
	}
	if !resp.OK {
		return fmt.Errorf("%s", resp.Error)
	}
	for _, skipped := range resp.Skipped {
		fmt.Fprintf(stderr, "gloss: skipped %s\n", skipped)
	}
	if req.Action == "status" && resp.File != "" {
		fmt.Fprintln(stdout, resp.File)
	}
	return nil
}
