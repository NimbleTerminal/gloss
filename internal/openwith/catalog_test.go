//go:build !js

package openwith

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestCatalogDiscovery(t *testing.T) {
	dir := t.TempDir()
	bundle := filepath.Join(dir, "Applications", "Skim.app")
	if err := os.MkdirAll(bundle, 0700); err != nil {
		t.Fatal(err)
	}
	probes := Probes{Home: dir, Stat: os.Stat, LookPath: func(s string) (string, error) {
		if s == "open" || s == "xdg-open" || s == "code" {
			return "/bin/" + s, nil
		}
		return "", fs.ErrNotExist
	}}
	for _, platform := range []string{"darwin", "linux", "windows", "freebsd"} {
		c, err := Discover(platform, probes)
		if err != nil {
			t.Fatal(err)
		}
		seen := map[string]bool{}
		for _, e := range c.Entries {
			if seen[e.ID] {
				t.Fatal("duplicate", e.ID)
			}
			seen[e.ID] = true
		}
		if !seen["system"] || !seen["reveal"] {
			t.Fatal("missing reserved actions")
		}
		if platform == "darwin" && !c.Available["skim"] {
			t.Fatal("user bundle not found")
		}
		if platform == "linux" && !c.Available["vscode"] {
			t.Fatal("PATH editor not found")
		}
		if platform == "windows" && c.Available["system"] {
			t.Fatal("invented executable")
		}
	}
}

func TestEveryEnabledCatalogCommand(t *testing.T) {
	info, err := os.Stat(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	probes := Probes{Home: "/home/test", Stat: func(string) (fs.FileInfo, error) { return info, nil }, LookPath: func(s string) (string, error) { return "/bin/" + s, nil }}
	for _, platform := range []string{"darwin", "linux", "windows"} {
		c, err := Discover(platform, probes)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range c.Entries {
			if !c.Available[e.ID] {
				t.Fatalf("not probed: %s/%s", platform, e.ID)
			}
			name, args, err := BuildCommand(e, "/tmp/file name, Ω.txt")
			if err != nil || name != e.Exec[0] || args[len(args)-1] != "/tmp/file name, Ω.txt" {
				t.Fatalf("%s/%s: %q %q %v", platform, e.ID, name, args, err)
			}
			if platform == "darwin" && e.ID != "system" && e.ID != "reveal" {
				if len(args) != 3 || args[0] != "-a" || filepath.Ext(args[1]) != ".app" {
					t.Fatalf("bad bundle command: %q", args)
				}
			}
			if platform == "linux" && len(args) != 1 {
				t.Fatalf("unexpected Linux args: %q", args)
			}
		}
	}
}
