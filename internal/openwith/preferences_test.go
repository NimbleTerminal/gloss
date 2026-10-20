//go:build !js

package openwith

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"nimbleterminal.dev/ntopenwith"
)

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestLegacyMigrates(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "open-with.json", `{"defaults":{"pdf":"skim"},"hidden":["zed"]}`)
	prefs, invalid, err := LoadPreferences(dir)
	if err != nil || invalid != nil {
		t.Fatalf("invalid=%v err=%v", invalid, err)
	}
	if prefs.Defaults[Open]["pdf"] != "skim" || !slices.Equal(prefs.Hidden, []string{"zed"}) {
		t.Fatalf("%+v", prefs)
	}
}

func TestNoFilesIsEmpty(t *testing.T) {
	prefs, invalid, err := LoadPreferences(filepath.Join(t.TempDir(), "missing"))
	if err != nil || invalid != nil || len(prefs.Defaults) != 0 || len(prefs.Hidden) != 0 {
		t.Fatalf("%+v %v %v", prefs, invalid, err)
	}
}

func TestV1WinsAndLegacyUntouched(t *testing.T) {
	dir := t.TempDir()
	legacy := `{"defaults":{"pdf":"skim"},"hidden":["zed"]}`
	legacyPath := writeFile(t, dir, "open-with.json", legacy)
	writeFile(t, dir, "open-with-v1.json", `{"version":1,"defaults":{"open":{"pdf":"viewer"}},"hidden":[]}`)
	prefs, invalid, err := LoadPreferences(dir)
	if err != nil || invalid != nil {
		t.Fatalf("invalid=%v err=%v", invalid, err)
	}
	if prefs.Defaults[Open]["pdf"] != "viewer" || len(prefs.Hidden) != 0 {
		t.Fatalf("%+v", prefs)
	}
	var p ntopenwith.Preferences
	p.SetDefault(Edit, "text", "x")
	if err := Saver(dir, invalid)(p); err != nil {
		t.Fatal(err)
	}
	if readFile(t, legacyPath) != legacy {
		t.Fatal("legacy file changed")
	}
	got, _, err := LoadPreferences(dir)
	if err != nil || got.Defaults[Edit]["text"] != "x" {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestCorruptPreferencesRefuseSaving(t *testing.T) {
	dir := t.TempDir()
	v1 := writeFile(t, dir, "open-with-v1.json", "{")
	prefs, invalid, err := LoadPreferences(dir)
	if err != nil || invalid == nil || len(prefs.Defaults) != 0 || len(prefs.Hidden) != 0 {
		t.Fatalf("%+v %v %v", prefs, invalid, err)
	}
	if !strings.Contains(invalid.Error(), "open-with-v1.json") {
		t.Fatalf("damaged file not named: %v", invalid)
	}
	err = Saver(dir, invalid)(ntopenwith.Preferences{Hidden: []string{"a"}})
	if err == nil || !strings.Contains(err.Error(), "repair open-with-v1.json") {
		t.Fatalf("%v", err)
	}
	if readFile(t, v1) != "{" {
		t.Fatal("corrupt file overwritten")
	}
}

func TestCorruptLegacyStillSavesV1(t *testing.T) {
	dir := t.TempDir()
	legacy := writeFile(t, dir, "open-with.json", "{")
	_, invalid, err := LoadPreferences(dir)
	if err != nil || invalid == nil {
		t.Fatalf("invalid=%v err=%v", invalid, err)
	}
	if !strings.Contains(invalid.Error(), "open-with.json") || strings.Contains(invalid.Error(), "-v1") {
		t.Fatalf("damaged file not named: %v", invalid)
	}
	var p ntopenwith.Preferences
	p.SetHidden("a", true)
	if err := Saver(dir, invalid)(p); err != nil {
		t.Fatal(err)
	}
	if readFile(t, legacy) != "{" {
		t.Fatal("legacy file changed")
	}
	got, invalid, err := LoadPreferences(dir)
	if err != nil || invalid != nil || !got.IsHidden("a") {
		t.Fatalf("%+v %v %v", got, invalid, err)
	}
}

func TestUnreadablePreferencesAreAnError(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "open-with-v1.json"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, invalid, err := LoadPreferences(dir); err == nil || invalid != nil {
		t.Fatalf("invalid=%v err=%v", invalid, err)
	}
}

func TestLegacyDefaultUnavailableFallsBack(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "open-with.json", `{"defaults":{"pdf":"skim"}}`)
	prefs, _, err := LoadPreferences(dir)
	if err != nil {
		t.Fatal(err)
	}
	src := newFake()
	h := NewHost(src, prefs, (&recorder{}).save)
	l, err := h.Launch(Target{Path: testFile(t, "a.pdf"), Kind: "pdf"}, Open, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(l.Name, " (saved app unavailable)") {
		t.Fatalf("%q", l.Name)
	}
}

func TestLegacyAliasResolves(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "open-with.json", `{"defaults":{"markdown":"vscode"}}`)
	prefs, _, err := LoadPreferences(dir)
	if err != nil {
		t.Fatal(err)
	}
	src := newFake()
	src.apps[Open] = append(src.apps[Open], ntopenwith.App{ID: "mac:x", Name: "Code"})
	layered := ntopenwith.Layered(src, ntopenwith.Alias("vscode", "mac:x"))
	h := NewHost(layered, prefs, (&recorder{}).save)
	l, err := h.Launch(Target{Path: testFile(t, "a.md"), Kind: "markdown"}, Open, "")
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasSuffix(l.Name, "(saved app unavailable)") || len(src.launched) != 1 || src.launched[0] != "open:mac:x" {
		t.Fatalf("%q %v", l.Name, src.launched)
	}
}
