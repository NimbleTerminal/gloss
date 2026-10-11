package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestFontFlagsAndPreferences(t *testing.T) {
	for _, name := range []string{"readable", "editorial", "technical", "compact"} {
		var out bytes.Buffer
		opts, _, err := parse([]string{"--font", name, "--mono", "jetbrains"}, &out)
		if err != nil || opts.Font != name || !opts.fontExplicit {
			t.Fatal(name, err)
		}
	}
	for _, args := range [][]string{{"--font", "unknown"}, {"--mono", "iosevka"}} {
		var out bytes.Buffer
		if _, _, err := parse(args, &out); err == nil {
			t.Fatal("invalid font accepted")
		}
	}
	dir := t.TempDir()
	if err := writeFontChoice(dir, "technical", "jetbrains"); err != nil {
		t.Fatal(err)
	}
	name, mono, err := readFontChoice(dir)
	if err != nil || name != "technical" || mono != "jetbrains" {
		t.Fatal(name, mono, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "font"), []byte(`{"font":"bad"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readFontChoice(dir); err == nil {
		t.Fatal("bad saved font accepted")
	}
	var out bytes.Buffer
	opts, _, err := parse([]string{"--typeset", "--output", "-"}, &out)
	if err != nil || opts.Font != "readable" || opts.Mono != "" {
		t.Fatal("export default changed", err)
	}
}
