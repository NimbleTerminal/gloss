package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"nimbleterminal.dev/gloss/internal/document"
)

// A range of pages is listed in full, so a range must be bounded: --page
// 1-2000000000 once asked for gigabytes, and one ending at the largest integer
// looped for ever.
func TestPageRangesAreBounded(t *testing.T) {
	for _, bad := range []string{"1-2000000000", "1-9223372036854775807", "5-99999", "1-5000,5001-10001", "1-10001"} {
		done := make(chan error, 1)
		go func() { _, err := parsePages(bad); done <- err }()
		select {
		case err := <-done:
			if err == nil || !strings.Contains(err.Error(), "more than") {
				t.Errorf("--page %s: err = %v, want a refusal", bad, err)
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("--page %s did not return: it is being listed", bad)
		}
	}
	for good, n := range map[string]int{"1-10000": 10000, "1-5000,5001-10000": 10000, "3": 1, "2-5": 4, "99999999999": 1, "1,3-4": 3} {
		got, err := parsePages(good)
		if err != nil || len(got.pages) != n {
			t.Errorf("--page %s: %d pages, %v; want %d", good, len(got.pages), err, n)
		}
	}
}

func FuzzParsePages(f *testing.F) {
	for _, seed := range []string{"3", "1-5", "1,3-5", "all", " 2 - 4 ", "0", "-1", "5-2", "1-", "-", ",", "1-9223372036854775807", "9223372036854775808"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, s string) {
		r, err := parsePages(s)
		if err == nil && !r.all && (len(r.pages) == 0 || len(r.pages) > document.MaxPages) {
			t.Fatalf("%q gave %d pages", s, len(r.pages))
		}
	})
}

// The skill is written whole or not at all, and a symbolic link where its file
// goes is replaced, not written through to what it points at.
func TestSkillInstallDoesNotWriteThroughASymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symbolic links need privileges on Windows")
	}
	root := t.TempDir()
	skillDir := filepath.Join(root, "skills", "gloss")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	precious := filepath.Join(root, "precious.txt")
	if err := os.WriteFile(precious, []byte("mine"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(precious, filepath.Join(skillDir, "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	var out, errs bytes.Buffer
	if err := installSkill(filepath.Join(root, "skills"), &out, &errs); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(precious); string(got) != "mine" {
		t.Fatalf("the install wrote through the link: %q", got)
	}
	info, err := os.Lstat(filepath.Join(skillDir, "SKILL.md"))
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o644 {
		t.Fatalf("SKILL.md afterwards: %v %v", info, err)
	}
	entries, _ := os.ReadDir(skillDir)
	if len(entries) != 1 {
		t.Fatalf("a temporary file was left: %v", entries)
	}
}
