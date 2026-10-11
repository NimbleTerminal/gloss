package theme

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCatalog(t *testing.T) {
	if len(Builtins()) != 14 {
		t.Fatal(Builtins())
	}
	for _, c := range Builtins() {
		ref, err := Load(c.Ref)
		if err != nil || Resolve(ref) != ref || Get(ref).Background.A != 255 {
			t.Fatal(c, err)
		}
	}
}
func TestCustomSnapshots(t *testing.T) {
	path := filepath.Join(t.TempDir(), "custom.json")
	put := func(s string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(s), 0600); err != nil {
			t.Fatal(err)
		}
	}
	put(`{"name":"test-custom","base":"dark","accent":"#123456"}`)
	a, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if Get(a).Accent != rgb(0x123456) || !Dark(a) || Name(a) != "test-custom" {
		t.Fatal(a, Get(a))
	}
	put(`{"name":"test-custom","base":"light","accent":"#abcdef"}`)
	b, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if a == b || Get(a).Accent != rgb(0x123456) || Get(b).Accent != rgb(0xabcdef) {
		t.Fatal("snapshots changed")
	}
	for _, data := range []string{`{"name":"../bad","base":"light"}`, `{"name":"bad","base":"no"}`, `{"name":"bad","base":"light","accent":"#12xx00"}`, `{"name":"bad","base":"light","syntax":"not-real"}`, `{"name":"bad","base":"light","typo":1}`, `{} {}`, strings.Repeat(" ", 65537)} {
		put(data)
		if _, err := Load(path); err == nil {
			t.Fatal("accepted invalid theme", data[:min(80, len(data))])
		}
	}
}

func TestSavedThemeSnapshot(t *testing.T) {
	dir := t.TempDir()
	for _, ref := range []string{"auto", "light", "nord"} {
		if err := saveChoice(dir, ref); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(filepath.Join(dir, "theme"))
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != ref {
			t.Fatal("saved preset changed identity")
		}

	}
}

func TestSavedCustomThemeSnapshot(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "custom.json")
	if err := os.WriteFile(path, []byte(`{"name":"saved-custom","base":"light","accent":"#456789"}`), 0600); err != nil {
		t.Fatal(err)
	}
	ref, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = saveChoice(dir, ref); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "theme"))
	if err != nil {
		t.Fatal(err)
	}
	s, err := decode(data)
	if err != nil || s.palette != Get(ref) {
		t.Fatal("saved custom palette", err)
	}
}
