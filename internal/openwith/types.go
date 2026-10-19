// Package openwith describes explicit requests to open local files in host apps.
package openwith

// Target is a snapshot of the file selected by the human.
type Target struct {
	Path, Kind           string
	Directory, Temporary bool
}

// Choice is a host action; command lines never cross this interface.
type Choice struct {
	ID, Name           string
	Default, Available bool
}

// Host is supplied only by an embedding host that permits native launching.
type Host interface {
	Choices(Target) ([]Choice, error)
	Launch(Target, string) (string, error)
	SetDefault(kind, id string) error
}

// Kinds are the canonical preference keys, in listing order.
var Kinds = []string{"image/*", "svg", "pdf", "stl", "3mf", "obj", "ply", "docx", "xlsx", "grist", "csv", "json", "ipynb", "html", "text", "markdown", "mermaid"}

// Kind maps loader image kinds to their shared preference key.
func Kind(kind string) string {
	switch kind {
	case "png", "jpeg", "gif", "webp", "bmp", "tiff", "heic", "heif", "jxl", "image":
		return "image/*"
	}
	return kind
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
