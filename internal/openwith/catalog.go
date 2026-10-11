//go:build !js

package openwith

import (
	"io/fs"
	"path/filepath"
	"slices"
)

// Entry is compiled-in argv and compatible kinds, never decoded from disk.
type Entry struct {
	ID, Name    string
	Kinds, Exec []string
}
type Probes struct {
	LookPath func(string) (string, error)
	Stat     func(string) (fs.FileInfo, error)
	Home     string
}
type Catalog struct {
	Entries   []Entry
	Available map[string]bool
	OS        string
}

func (c Catalog) entry(id string) (Entry, bool) {
	for _, e := range c.Entries {
		if e.ID == id {
			return e, true
		}
	}
	return Entry{}, false
}
func (e Entry) accepts(kind string) bool {
	return e.ID == "system" || e.ID == "reveal" || slices.Contains(e.Kinds, kind)
}

// Discover probes a fixed catalog. The explicit platform makes its policy testable.
func Discover(goos string, p Probes) (Catalog, error) {
	c := Catalog{Available: map[string]bool{}, OS: goos}
	add := func(id, name string, kinds []string, args []string) {
		found, err := p.LookPath(args[0])
		if err == nil {
			args = slices.Clone(args)
			args[0] = found
			c.Available[id] = true
		}
		c.Entries = append(c.Entries, Entry{id, name, kinds, args})
	}
	switch goos {
	case "darwin":
		add("system", "System default", nil, []string{"open"})
		add("reveal", "Reveal in Finder", nil, []string{"open", "-R"})
	case "windows":
		add("system", "System default", nil, []string{"rundll32.exe", "url.dll,FileProtocolHandler"})
		add("reveal", "Reveal in Explorer", nil, []string{"explorer.exe", "/select,"})
	default:
		add("system", "System default", nil, []string{"xdg-open"})
		add("reveal", "Open containing folder", nil, []string{"xdg-open"})
	}
	text := []string{"markdown", "text", "json", "html", "ipynb", "csv"}
	seeds := []struct {
		id, name, bundle, command string
		kinds                     []string
	}{
		{"preview", "Preview", "Preview", "", []string{"image/*", "pdf"}},
		{"skim", "Skim", "Skim", "", []string{"pdf"}},
		{"vscode", "VS Code", "Visual Studio Code", "code", text},
		{"cursor", "Cursor", "Cursor", "cursor", text},
		{"zed", "Zed", "Zed", "zed", text},
		{"sublime", "Sublime Text", "Sublime Text", "subl", text},
	}
	for _, s := range seeds {
		if goos == "darwin" && s.bundle != "" {
			bundle := ""
			for _, base := range []string{filepath.Join(p.Home, "Applications"), "/Applications", "/System/Applications"} {
				path := filepath.Join(base, s.bundle+".app")
				if info, err := p.Stat(path); err == nil && info.IsDir() {
					bundle = path
					break
				}
			}
			add(s.id, s.name, s.kinds, []string{"open", "-a", bundle})
			c.Available[s.id] = c.Available[s.id] && bundle != ""
		} else if goos == "linux" && s.command != "" {
			add(s.id, s.name, s.kinds, []string{s.command})
		}
	}
	return c, nil
}
