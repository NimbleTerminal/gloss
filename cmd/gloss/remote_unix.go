//go:build !windows

package main

import (
	"os"
	"os/signal"
	"syscall"

	tea "charm.land/bubbletea/v2"
	"nimbleterminal.dev/gloss/internal/app"
	"nimbleterminal.dev/gloss/internal/remote"
)

// reloadOnSignal has the viewer load its file again on SIGUSR1, so a build
// that rewrites a file in place can refresh it with pkill -USR1 gloss. The
// function returned stops listening.
func reloadOnSignal(send func(tea.Msg)) func() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGUSR1)
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-signals:
				send(app.RemoteMsg{Request: remote.Request{Action: "reload"}})
			case <-done:
				return
			}
		}
	}()
	return func() {
		signal.Stop(signals)
		close(done)
	}
}
