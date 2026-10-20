package handoff

import (
	"bytes"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"nimbleterminal.dev/gloss/internal/app"
)

// What a session keeps from the page is bounded, over every drop, and a drop
// that fails gives back what it took.
func TestServedDropsShareASessionBudget(t *testing.T) {
	old := maxSessionBytes
	maxSessionBytes = 100
	t.Cleanup(func() { maxSessionBytes = old })
	s := serving(t, app.Options{Pick: true})
	sixty := bytes.Repeat([]byte("x"), 60)
	code, first := drop(t, s.URL+"drop", nil, upload{"a.txt", sixty})
	if code != http.StatusOK || len(first) != 1 {
		t.Fatalf("the first drop: %d %q", code, first)
	}
	defer os.RemoveAll(filepath.Dir(first[0]))
	// 60 and 60 is over 100, in two requests: the second is refused whole.
	if code, paths := drop(t, s.URL+"drop", nil, upload{"b.txt", sixty}); code != http.StatusRequestEntityTooLarge || len(paths) != 0 {
		t.Fatalf("over the session's budget: %d %q", code, paths)
	}
	entries, _ := os.ReadDir(filepath.Dir(first[0]))
	if len(entries) != 1 {
		t.Fatalf("the refused drop left files behind: %v", entries)
	}
	// What a refused drop took was given back: 30 fits in the 40 left.
	if code, paths := drop(t, s.URL+"drop", nil, upload{"c.txt", bytes.Repeat([]byte("y"), 30)}); code != http.StatusOK || len(paths) != 1 {
		t.Fatalf("a small drop after a refused one: %d %q", code, paths)
	}
	// Several files in one request that together are too many: none is kept.
	if code, paths := drop(t, s.URL+"drop", nil, upload{"d.txt", bytes.Repeat([]byte("z"), 5)}, upload{"e.txt", sixty}); code != http.StatusRequestEntityTooLarge || len(paths) != 0 {
		t.Fatalf("one drop over the budget: %d %q", code, paths)
	}
	if code, paths := drop(t, s.URL+"drop", nil, upload{"f.txt", bytes.Repeat([]byte("w"), 5)}); code != http.StatusOK || len(paths) != 1 {
		t.Fatalf("the refused drop's files were not given back: %d %q", code, paths)
	}
}

func TestDroppedNamesAreUsable(t *testing.T) {
	for _, name := range []string{"report.pdf", "a b.png", "über.svg", "dots.in.name.csv"} {
		if !usableName(name) {
			t.Errorf("%q was refused", name)
		}
	}
	for _, name := range []string{"a\x00b", "tab\there", "bell\a", "del\x7f"} {
		if usableName(name) {
			t.Errorf("%q was accepted", name)
		}
	}
	// Where a name can mean a device or a stream, it may not.
	windowsOnly := []string{"CON", "nul.txt", "Com1.png", "a:b", "stream:evil.png", "a*b", "trailing.", "trailing "}
	for _, name := range windowsOnly {
		if got := usableName(name); got == (runtime.GOOS == "windows") {
			t.Errorf("%q: usableName = %v on %s", name, got, runtime.GOOS)
		}
	}
}

// A server that does not come up is stopped, and one that dies at once is
// said so without waiting out the grace period.
func TestDetachedStartStopsAServerThatDoesNotComeUp(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell scripts stand in for the server")
	}
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "pid")
	script := func(name, body string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		return path
	}
	oldExe, oldGrace := executable, startGrace
	t.Cleanup(func() { executable, startGrace = oldExe, oldGrace })
	startGrace = 20 * time.Second

	// Dies at once: reported at once, not after the grace period.
	executable = func() (string, error) { return script("dies", "exit 3"), nil }
	began := time.Now()
	var stdout, stderr bytes.Buffer
	err := Detach([]string{"--serve"}, Config{}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "stopped") || time.Since(began) > 5*time.Second {
		t.Fatalf("a server that died: err=%v after %v", err, time.Since(began))
	}

	// Alive but never up: stopped when the grace runs out.
	startGrace = 500 * time.Millisecond
	executable = func() (string, error) { return script("hangs", "echo $$ > "+pidFile+"\nexec sleep 60"), nil }
	err = Detach([]string{"--serve"}, Config{}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "did not start") {
		t.Fatalf("a server that hung: %v", err)
	}
	raw, rerr := os.ReadFile(pidFile)
	pid, perr := strconv.Atoi(strings.TrimSpace(string(raw)))
	if rerr != nil || perr != nil {
		t.Fatalf("the stand-in server never wrote its pid: %v %v", rerr, perr)
	}
	for end := time.Now().Add(3 * time.Second); running(pid); {
		if time.Now().After(end) {
			t.Fatalf("the server that did not start is still running (pid %d)", pid)
		}
		time.Sleep(25 * time.Millisecond)
	}
}
