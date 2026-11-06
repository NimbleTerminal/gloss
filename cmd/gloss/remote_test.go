package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"nimbleterminal.dev/gloss/internal/app"
	"nimbleterminal.dev/gloss/internal/remote"
)

// writeTestPNG writes a small picture at path.
func writeTestPNG(t *testing.T, path string) string {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	return path
}

// shortSocket is a socket path short enough for every system.
func shortSocket(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "gloss-remote")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return filepath.Join(dir, "v.sock")
}

func TestRemoteFlags(t *testing.T) {
	opts, _, err := parse([]string{"--remote", "preview", "--page", "3", "a.pdf"}, &bytes.Buffer{})
	if err != nil || opts.Remote != "preview" || !opts.pageExplicit || opts.Page != 3 || len(opts.Files) != 1 {
		t.Fatalf("%+v %v", opts, err)
	}
	opts, _, err = parse([]string{"--remote", "preview", "--reload"}, &bytes.Buffer{})
	if err != nil || !opts.Reload || opts.pageExplicit {
		t.Fatalf("%+v %v", opts, err)
	}
	if opts, _, err = parse([]string{"--remote", "preview", "--quit", "--json"}, &bytes.Buffer{}); err != nil || !opts.Quit || !opts.JSON {
		t.Fatalf("%+v %v", opts, err)
	}
	if opts, _, err = parse([]string{"--socket", "preview", "--pick", "a.pdf"}, &bytes.Buffer{}); err != nil || opts.Socket != "preview" || !opts.Pick {
		t.Fatalf("%+v %v", opts, err)
	}
	for _, args := range [][]string{
		{"--remote", "preview", "--serve"}, {"--remote", "preview", "--pick"}, {"--remote", "preview", "--output", "x.png", "a.pdf"},
		{"--remote", "preview", "--info", "a.pdf"}, {"--remote", "preview", "--text", "a.pdf"}, {"--remote", "preview", "--print", "a.pdf"},
		{"--remote", "preview", "--socket", "other", "a.pdf"}, {"--remote", "preview", "--fetch", "a.pdf"}, {"--remote", "preview", "--glob", "images"},
		{"--remote", "preview", "--reload", "--quit"}, {"--remote", "preview", "--reload", "a.pdf"}, {"--remote", "preview", "--quit", "a.pdf"},
		{"--remote", "preview", "--page", "3"}, {"--reload"}, {"--quit"}, {"--remote", "bad name", "a.pdf"}, {"--remote", ""},
		{"--socket", "preview", "--serve", "a.pdf"}, {"--socket", "preview", "--output", "x.png", "a.pdf"}, {"--socket", "preview", "--info", "a.pdf"},
		{"--socket", "preview", "--print", "a.pdf"}, {"--socket", "preview", "--pick-web"}, {"--socket", "bad name", "a.pdf"},
	} {
		if _, _, err := parse(args, &bytes.Buffer{}); err == nil {
			t.Errorf("accepted %v", args)
		}
	}
}

func TestRemoteSendsWhatTheViewerTakes(t *testing.T) {
	path := shortSocket(t)
	var got []remote.Request
	server, err := remote.Listen(path, func(req remote.Request) remote.Response {
		got = append(got, req)
		switch req.Action {
		case "open":
			return remote.Response{OK: true, Added: 1, File: req.Paths[0], Page: req.Page, Skipped: req.Paths[1:], Files: 2}
		case "status":
			return remote.Response{OK: true, File: "/shown.pdf", Kind: "pdf", Page: 2, Pages: 9, Files: 1}
		case "quit":
			return remote.Response{Error: "the viewer is quitting"}
		}
		return remote.Response{OK: true}
	})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	picture := writeTestPNG(t, filepath.Join(t.TempDir(), "p.png"))
	other := writeTestPNG(t, filepath.Join(t.TempDir(), "q.png"))
	var stdout, stderr bytes.Buffer
	wd, _ := os.Getwd()
	relative, _ := filepath.Rel(wd, other)
	if err := run([]string{"--remote", path, "--page", "4", picture, relative}); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Action != "open" || got[0].Page != 4 || len(got[0].Paths) != 2 || got[0].Paths[0] != picture || got[0].Paths[1] != other {
		t.Fatalf("viewer saw %+v", got)
	}
	// Plain output says nothing of a success, and names what was skipped.
	if err := sendRemote(options{Remote: path, Options: app.Options{Files: []string{picture, other}}}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if stdout.Len() != 0 || !strings.Contains(stderr.String(), "skipped "+other) {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	// A file that is not there is refused before the viewer is asked.
	asked := len(got)
	if err := sendRemote(options{Remote: path, Options: app.Options{Files: []string{filepath.Join(t.TempDir(), "gone.png")}}}, &stdout, &stderr); err == nil || len(got) != asked {
		t.Fatalf("missing file: err=%v asked=%d", err, len(got)-asked)
	}
	// An address goes as it is: the viewer fetches, if it may.
	if err := sendRemote(options{Remote: path, Options: app.Options{Files: []string{"https://example.com/a.pdf"}}}, &stdout, &stderr); err != nil || got[len(got)-1].Paths[0] != "https://example.com/a.pdf" {
		t.Fatalf("address: %v %+v", err, got[len(got)-1])
	}
	// With no file, the file shown; as JSON, the whole answer.
	stdout.Reset()
	if err := sendRemote(options{Remote: path}, &stdout, &stderr); err != nil || stdout.String() != "/shown.pdf\n" {
		t.Fatalf("status: %v %q", err, stdout.String())
	}
	stdout.Reset()
	if err := sendRemote(options{Remote: path, JSON: true}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	var resp remote.Response
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil || !resp.OK || resp.Kind != "pdf" || resp.Pages != 9 {
		t.Fatalf("status JSON: %v %q", err, stdout.String())
	}
	if got[len(got)-1].Action != "status" {
		t.Fatalf("viewer saw %+v", got[len(got)-1])
	}
	// Reload and quit are their own requests; a refusal is an error, with
	// its reason, and as JSON the answer is printed and the error reported.
	if err := sendRemote(options{Remote: path, Reload: true}, &stdout, &stderr); err != nil || got[len(got)-1].Action != "reload" {
		t.Fatalf("reload: %v %+v", err, got[len(got)-1])
	}
	err = sendRemote(options{Remote: path, Quit: true}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "quitting") || got[len(got)-1].Action != "quit" {
		t.Fatalf("quit: %v %+v", err, got[len(got)-1])
	}
	stdout.Reset()
	err = sendRemote(options{Remote: path, Quit: true, JSON: true}, &stdout, &stderr)
	if !errors.As(err, new(reported)) || !strings.Contains(stdout.String(), `"ok":false`) {
		t.Fatalf("quit as JSON: %v %q", err, stdout.String())
	}
	// No viewer: an error that says so, exit 1.
	server.Close()
	err = run([]string{"--remote", path})
	if !errors.Is(err, remote.ErrNoViewer) || !strings.Contains(err.Error(), path) {
		t.Fatalf("no viewer: %v", err)
	}
}

func TestListenRemoteAsksTheViewer(t *testing.T) {
	path := shortSocket(t)
	sent := make(chan tea.Msg, 1)
	server, err := listenRemote(path, func(msg tea.Msg) {
		sent <- msg
		if v, ok := msg.(app.RemoteMsg); ok && v.Request.Action == "status" {
			v.Reply <- remote.Response{OK: true, File: "/a.pdf"}
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	resp, err := remote.Send(path, remote.Request{Action: "status"}, 0)
	if err != nil || !resp.OK || resp.File != "/a.pdf" {
		t.Fatalf("%+v %v", resp, err)
	}
	msg := <-sent
	if v, ok := msg.(app.RemoteMsg); !ok || v.Request.Action != "status" || v.Reply == nil {
		t.Fatalf("viewer got %#v", msg)
	}
	// A name resolves to the temporary folder.
	named, err := listenRemote("gloss-test-"+strings.ToLower(t.Name()), func(tea.Msg) {})
	if err != nil {
		t.Fatal(err)
	}
	defer named.Close()
	if !strings.HasPrefix(named.Path(), filepath.Clean(os.TempDir())) || !strings.HasSuffix(named.Path(), ".sock") {
		t.Fatal(named.Path())
	}
	if _, err := listenRemote("no good", func(tea.Msg) {}); err == nil {
		t.Fatal("bad name accepted")
	}
}

func TestReloadOnSignalStops(t *testing.T) {
	stop := reloadOnSignal(func(tea.Msg) {})
	done := make(chan struct{})
	go func() { stop(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("stop hung")
	}
}
