//go:build !js

package openwith

import (
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestEditorCommand(t *testing.T) {
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "- a $(touch nope); file.md")
	if err := os.WriteFile(path, []byte("text"), 0600); err != nil {
		t.Fatal(err)
	}
	target := Target{Path: path, Kind: "markdown"}
	cmd, err := editorCommand(strconv.Quote(binary)+` --wait 'two words' '$HOME'`, target)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{binary, "--wait", "two words", "$HOME", path}
	if !reflect.DeepEqual(cmd.Args, want) {
		t.Fatalf("args %q, want %q", cmd.Args, want)
	}
	if cmd.Stdin != nil || cmd.Stdout != nil || cmd.Stderr != nil {
		t.Fatal("editor streams must be attached by Bubble Tea")
	}
	for _, setting := range []string{"", `"unterminated`, "gloss-no-such-editor-123456", strings.Repeat("x", 8193), "bad\x00setting"} {
		if _, err := editorCommand(setting, target); err == nil {
			t.Fatalf("accepted %q", setting)
		}
	}
	for _, kind := range []string{"pdf", "docx", "xlsx", "png", "stl", "grist"} {
		target.Kind = kind
		if _, err := editorCommand(binary, target); err == nil {
			t.Fatalf("editing converted/binary %s", kind)
		}
	}
	target.Kind, target.Path = "text", t.TempDir()
	if _, err := editorCommand(binary, target); err == nil {
		t.Fatal("edited directory")
	}
}

func TestEditorPreferenceAndChoices(t *testing.T) {
	t.Setenv("VISUAL", "preferred -f")
	t.Setenv("EDITOR", "fallback")
	if editorSetting() != "preferred -f" {
		t.Fatal("VISUAL not preferred")
	}
	t.Setenv("VISUAL", " ")
	if editorSetting() != "fallback" {
		t.Fatal("EDITOR fallback missing")
	}
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("EDITOR", binary)
	h, err := NewHost(testCatalog(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range Kinds {
		choices, err := h.Choices(Target{Kind: kind})
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, c := range choices {
			if c.ID == "terminal-editor" {
				found = true
				if !c.Available {
					t.Fatal("configured editor unavailable")
				}
			}
		}
		if found != Editable(kind) {
			t.Fatalf("editor offered for %s: %v", kind, found)
		}
	}
	if err := h.SetDefault("text", "terminal-editor"); err == nil {
		t.Fatal("editor persisted as background launch")
	}
	t.Setenv("EDITOR", "")
	choices, _ := h.Choices(Target{Kind: "text"})
	for _, c := range choices {
		if c.ID == "terminal-editor" && c.Available {
			t.Fatal("unconfigured editor advertised available")
		}
	}
}
