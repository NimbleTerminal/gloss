package main

import (
	"os/exec"
	"runtime"
)

// browse opens an address in the user's default browser: the page of a
// --serve, or a link clicked in a document.
func browse(address string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", address).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", address).Start()
	}
	return exec.Command("xdg-open", address).Start()
}
