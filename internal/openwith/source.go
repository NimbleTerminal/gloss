//go:build !js

package openwith

import (
	"runtime"

	"nimbleterminal.dev/ntopenwith"
	"nimbleterminal.dev/ntopenwith/system"
)

// Source is the system's apps, with gloss's old IDs kept as aliases, QuickTime
// Player kept from editing images, and the terminal editors after them.
func Source() ntopenwith.Source {
	layers := []ntopenwith.Source{system.System()}
	layers = append(layers, aliases(runtime.GOOS)...)
	return ntopenwith.Layered(append(layers,
		ntopenwith.HideFor([]ntopenwith.Verb{ntopenwith.Edit}, []string{"image/*"}, "mac:com.apple.QuickTimePlayerX"),
		ntopenwith.List(ntopenwith.TerminalEditor()),
		ntopenwith.List(ntopenwith.TerminalEditors()...),
	)...)
}

// aliases map the IDs gloss saved before it used ntopenwith to the apps they
// named on goos.
func aliases(goos string) []ntopenwith.Source {
	var table [][2]string
	switch goos {
	case "darwin":
		table = [][2]string{
			{"vscode", "mac:com.microsoft.VSCode"},
			{"cursor", "mac:com.todesktop.230313mzl4w4u92"},
			{"zed", "mac:dev.zed.Zed"},
			{"sublime", "mac:com.sublimetext.4"},
			{"preview", "mac:com.apple.Preview"},
			{"skim", "mac:net.sourceforge.skim-app.skim"},
		}
	case "linux", "freebsd", "openbsd", "netbsd":
		table = [][2]string{
			{"vscode", "xdg:code.desktop"},
			{"cursor", "xdg:cursor.desktop"},
			{"zed", "xdg:dev.zed.Zed.desktop"},
			{"sublime", "xdg:sublime_text.desktop"},
		}
	}
	out := make([]ntopenwith.Source, 0, len(table))
	for _, a := range table {
		out = append(out, ntopenwith.Alias(a[0], a[1]))
	}
	return out
}
