//go:build windows

package main

import tea "charm.land/bubbletea/v2"

// reloadOnSignal does nothing on Windows, which has no SIGUSR1; the socket
// still takes a reload.
func reloadOnSignal(func(tea.Msg)) func() { return func() {} }
