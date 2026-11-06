package remote

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// socket is a short path for a test's socket: a test's temporary folder can
// be too deep for one.
func socket(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "gloss-remote")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return filepath.Join(dir, "s.sock")
}

func TestSocketPath(t *testing.T) {
	path, err := SocketPath("rex-preview")
	if err != nil {
		t.Fatal(err)
	}
	want, _ := filepath.Abs(filepath.Join(os.TempDir(), "gloss-rex-preview.sock"))
	if path != want {
		t.Fatalf("path=%q want %q", path, want)
	}
	for _, bad := range []string{"", "a b", "-lead", ".hidden", strings.Repeat("x", 65), "a:b"} {
		if _, err := SocketPath(bad); err == nil {
			t.Fatalf("%q was accepted as a name", bad)
		}
	}
	own := filepath.Join(filepath.Dir(socket(t)), "x.sock")
	if path, err := SocketPath(own); err != nil || path != own {
		t.Fatalf("path %q: %q %v", own, path, err)
	}
	if _, err := SocketPath("/" + strings.Repeat("d/", 60) + "s.sock"); err == nil {
		t.Fatal("an overlong path was accepted")
	}
}

func TestRoundTrip(t *testing.T) {
	path := socket(t)
	var got Request
	s, err := Listen(path, func(req Request) Response {
		got = req
		return Response{OK: true, Added: len(req.Paths), File: req.Paths[0], Page: req.Page}
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if s.Path() != path {
		t.Fatal(s.Path())
	}
	if info, err := os.Stat(path); err != nil || info.Mode()&os.ModeSocket == 0 || info.Mode().Perm() != 0o600 {
		t.Fatalf("socket file: %v %v", info, err)
	}
	resp, err := Send(path, Request{Action: "open", Paths: []string{"/a.pdf", "/b.png"}, Page: 3}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got.Action != "open" || len(got.Paths) != 2 || got.Page != 3 {
		t.Fatalf("server saw %+v", got)
	}
	if !resp.OK || resp.Added != 2 || resp.File != "/a.pdf" || resp.Page != 3 {
		t.Fatalf("client got %+v", resp)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("socket file left: %v", err)
	}
	if _, err := Send(path, Request{Action: "status"}, 0); !errors.Is(err, ErrNoViewer) {
		t.Fatalf("after close: %v", err)
	}
}

func TestBadRequests(t *testing.T) {
	path := socket(t)
	s, err := Listen(path, func(Request) Response { t.Error("handler ran"); return Response{} })
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, line := range []string{"not json\n", "{}\n", "{\"action\":\"\"}\n", ""} {
		conn, err := net.Dial("unix", path)
		if err != nil {
			t.Fatal(err)
		}
		conn.Write([]byte(line))
		if line == "" {
			conn.(*net.UnixConn).CloseWrite()
		}
		buf := make([]byte, 4096)
		n, _ := conn.Read(buf)
		conn.Close()
		if !strings.Contains(string(buf[:n]), `"ok":false`) || !strings.Contains(string(buf[:n]), "bad request") {
			t.Fatalf("%q answered %q", line, buf[:n])
		}
	}
}

func TestStaleAndBusySockets(t *testing.T) {
	path := socket(t)
	// What a dead viewer left behind is replaced.
	dead, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	dead.(*net.UnixListener).SetUnlinkOnClose(false)
	dead.Close()
	if _, err := os.Lstat(path); err != nil {
		t.Fatalf("test needs a stale socket file: %v", err)
	}
	s, err := Listen(path, func(Request) Response { return Response{OK: true} })
	if err != nil {
		t.Fatalf("stale socket not replaced: %v", err)
	}
	defer s.Close()
	// A live viewer is not displaced.
	if _, err := Listen(path, func(Request) Response { return Response{} }); err == nil || !strings.Contains(err.Error(), "already listening") {
		t.Fatalf("second listener: %v", err)
	}
	if resp, err := Send(path, Request{Action: "status"}, 0); err != nil || !resp.OK {
		t.Fatalf("first listener disturbed: %+v %v", resp, err)
	}
	// A plain file is never removed.
	file := filepath.Join(filepath.Dir(path), "f.sock")
	os.WriteFile(file, []byte("x"), 0o600)
	if _, err := Listen(file, func(Request) Response { return Response{} }); err == nil || !strings.Contains(err.Error(), "not a socket") {
		t.Fatalf("plain file: %v", err)
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatal("plain file was removed")
	}
}

func TestSendWaitsNoLongerThanAsked(t *testing.T) {
	path := socket(t)
	release := make(chan struct{})
	s, err := Listen(path, func(Request) Response { <-release; return Response{OK: true} })
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	defer close(release)
	start := time.Now()
	_, err = Send(path, Request{Action: "reload"}, 50*time.Millisecond)
	if err == nil || time.Since(start) > 2*time.Second {
		t.Fatalf("err=%v after %v", err, time.Since(start))
	}
}
