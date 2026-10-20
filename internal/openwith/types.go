// Package openwith adapts nimbleterminal.dev/ntopenwith to gloss: its kinds,
// its old app IDs, and the Reveal and Copy path rows its chooser adds.
package openwith

import (
	"os/exec"

	"nimbleterminal.dev/ntopenwith"
)

// Verb is what to do with a file: Open or Edit.
type Verb = ntopenwith.Verb

const (
	Open = ntopenwith.Open
	Edit = ntopenwith.Edit
)

// Target is a snapshot of the file selected by the human.
type Target struct {
	Path, Kind           string
	Directory, Temporary bool
}

// Choice is a host action; command lines never cross this interface.
type Choice struct {
	ID, Name                     string
	Default, Available, Terminal bool
}

// Launched names the app started. Cmd is set only for a terminal app: it is
// unstarted, for the pager to run with the terminal attached.
type Launched struct {
	Name string
	Cmd  *exec.Cmd
}

// Host is supplied only by an embedding host that permits native launching.
// Launch with an empty id resolves the app; its errors wrap
// ntopenwith.ErrChoose and ntopenwith.ErrNoEditor unchanged.
type Host interface {
	Choices(Target, Verb) ([]Choice, error)
	Launch(Target, Verb, string) (Launched, error)
	SetDefault(kind string, v Verb, id string) error
}

// AppPreference describes an installed app's visibility in all file choosers.
type AppPreference struct {
	ID, Name string
	Enabled  bool
}

// Manager is an optional native-host capability for editing app visibility.
// System default and Reveal are always available and cannot be hidden here.
type Manager interface {
	Apps() []AppPreference
	SetEnabled(id string, enabled bool) error
}
