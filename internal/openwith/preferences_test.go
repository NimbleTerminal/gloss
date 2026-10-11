//go:build !js

package openwith

import (
	"bytes"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func testCatalog() Catalog {
	return Catalog{OS: "darwin", Entries: []Entry{{ID: "system", Name: "System default", Exec: []string{"open"}}, {ID: "reveal", Name: "Reveal", Exec: []string{"open", "-R"}}, {ID: "skim", Name: "Skim", Kinds: []string{"pdf"}, Exec: []string{"open", "-a", "Skim"}}}, Available: map[string]bool{"system": true, "reveal": true, "skim": true}}
}
func TestPreferences(t *testing.T) {
	dir := t.TempDir()
	c := testCatalog()
	good := Preferences{Defaults: map[string]string{"pdf": "skim"}, Hidden: []string{"skim"}}
	if err := WritePreferences(dir, good); err != nil {
		t.Fatal(err)
	}
	p, err := ReadPreferences(dir, c)
	if err != nil || p.Defaults["pdf"] != "skim" || len(p.Hidden) != 1 {
		t.Fatalf("%+v %v", p, err)
	}
	for _, data := range [][]byte{
		append([]byte(`{"defaults":{},"hidden":[]}`), bytes.Repeat([]byte(" "), 65537)...),
		[]byte(`{} {}`), []byte(`{"exec":["sh"]}`), []byte(`{"defaults":1}`),
	} {
		if err := os.WriteFile(filepath.Join(dir, "open-with.json"), data, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadPreferences(dir, c); err == nil {
			t.Fatalf("accepted %q", data[:min(40, len(data))])
		}
	}
	data := []byte(`{"defaults":{"pdf":"skim","text":"skim","bad":"skim"},"hidden":["system","reveal","unknown"]}`)
	if err := os.WriteFile(filepath.Join(dir, "open-with.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	p, err = ReadPreferences(dir, c)
	if err != nil || len(p.Defaults) != 1 || len(p.Hidden) != 0 {
		t.Fatalf("%+v %v", p, err)
	}
}
func TestHostDefaults(t *testing.T) {
	c := testCatalog()
	dir := t.TempDir()
	h, err := NewHost(c, dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.SetDefault("pdf", "skim"); err != nil {
		t.Fatal(err)
	}
	choices, err := h.Choices(Target{Kind: "pdf"})
	if err != nil {
		t.Fatal(err)
	}
	if !choices[1].Default {
		t.Fatalf("%+v", choices)
	}
	if err := h.SetDefault("text", "skim"); err == nil {
		t.Fatal("incompatible default")
	}
	if err := h.SetDefault("pdf", "reveal"); err == nil {
		t.Fatal("reveal default")
	}
	c.Available["skim"] = false
	h, err = NewHost(c, dir)
	if err != nil {
		t.Fatal(err)
	}
	choices, err = h.Choices(Target{Kind: "pdf"})
	if err != nil || !choices[0].Default {
		t.Fatalf("%+v %v", choices, err)
	}
	if err := h.SetDefault("pdf", "system"); err != nil {
		t.Fatal(err)
	}
	p, err := ReadPreferences(dir, c)
	if err != nil || len(p.Defaults) != 0 {
		t.Fatalf("%+v %v", p, err)
	}
}
func TestHostRejectsInvalidTargets(t *testing.T) {
	h, err := NewHost(testCatalog(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []Target{{Path: "https://example.com"}, {Path: t.TempDir()}, {Path: "/missing", Kind: "pdf"}} {
		if _, err := h.Launch(target, "system"); err == nil {
			t.Fatalf("accepted %+v", target)
		}
	}
}

func TestMalformedPreferencesCannotBeOverwritten(t *testing.T) {
	dir := t.TempDir()
	name := filepath.Join(dir, "open-with.json")
	old := []byte(`{"exec":"bad"}`)
	if err := os.WriteFile(name, old, 0600); err != nil {
		t.Fatal(err)
	}
	h, err := NewHost(testCatalog(), dir)
	if h == nil || err == nil {
		t.Fatal("expected fallback host and diagnostic")
	}
	if err := h.SetDefault("pdf", "skim"); err == nil {
		t.Fatal("overwrote malformed preferences")
	}
	got, err := os.ReadFile(name)
	if err != nil || !bytes.Equal(got, old) {
		t.Fatal("changed file")
	}
}
func TestFailedSavePreservesDefault(t *testing.T) {
	dir := t.TempDir()
	h, err := NewHost(testCatalog(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "open-with.json"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := h.SetDefault("pdf", "skim"); err == nil {
		t.Fatal("save succeeded over directory")
	}
	choices, err := h.Choices(Target{Kind: "pdf"})
	if err != nil || !choices[0].Default {
		t.Fatal("default changed after failed save")
	}
}

func TestPreferenceNullRejected(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "open-with.json"), []byte("null"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadPreferences(dir, testCatalog()); err == nil {
		t.Fatal("accepted null preference file")
	}
}

func TestConcurrentDefaults(t *testing.T) {
	h, err := NewHost(testCatalog(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 8 {
				if err := h.SetDefault("pdf", "skim"); err != nil {
					t.Error(err)
				}
				if _, err := h.Choices(Target{Kind: "pdf"}); err != nil {
					t.Error(err)
				}
			}
		})
	}
	wg.Wait()
}

func TestDirectoryOnlyOffersReveal(t *testing.T) {
	h, err := NewHost(testCatalog(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	choices, err := h.Choices(Target{Directory: true})
	if err != nil || len(choices) != 1 || choices[0].ID != "reveal" {
		t.Fatalf("%+v %v", choices, err)
	}
}

func TestManageApps(t *testing.T) {
	dir := t.TempDir()
	h, err := NewHost(testCatalog(), dir)
	if err != nil {
		t.Fatal(err)
	}
	manager := h.(Manager)
	if err := h.SetDefault("pdf", "skim"); err != nil {
		t.Fatal(err)
	}
	for _, enabled := range []bool{false, true} {
		if err := manager.SetEnabled("skim", enabled); err != nil {
			t.Fatal(err)
		}
		apps := manager.Apps()
		if len(apps) != 1 || apps[0].Enabled != enabled {
			t.Fatalf("apps: %+v", apps)
		}
		reopened, err := NewHost(testCatalog(), dir)
		if err != nil {
			t.Fatal(err)
		}
		choices, err := reopened.Choices(Target{Kind: "pdf"})
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, c := range choices {
			if c.ID == "skim" {
				found = true
				if !c.Default {
					t.Fatal("restored app lost default")
				}
			}
		}
		if found != enabled || choices[0].Default == enabled {
			t.Fatalf("choices: %+v", choices)
		}
	}
	for _, id := range []string{"system", "reveal", "unknown"} {
		if err := manager.SetEnabled(id, false); err == nil {
			t.Fatalf("managed %s", id)
		}
	}
	// A failed save must leave the in-memory preference unchanged.
	broken := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(broken, nil, 0600); err != nil {
		t.Fatal(err)
	}
	h.(*nativeHost).dir = broken
	if err := manager.SetEnabled("skim", false); err == nil {
		t.Fatal("save succeeded")
	}
	if !manager.Apps()[0].Enabled {
		t.Fatal("failed save changed preference")
	}
}
