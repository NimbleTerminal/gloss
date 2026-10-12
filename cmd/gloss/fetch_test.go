package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"nimbleterminal.dev/gloss/internal/app"
)

func TestWebAddress(t *testing.T) {
	for arg, want := range map[string]bool{
		"https://example.com/a.png":   true,
		"HTTP://example.com/":         true,
		"http://":                     false,
		"./http://example.com/a.png":  false,
		`C:\pictures\a.png`:           false,
		"notes.md":                    false,
		"ftp://example.com/a.png":     false,
		"https://example.com/a b.png": true,
	} {
		if got := webAddress(arg); got != want {
			t.Errorf("webAddress(%q) = %v, want %v", arg, got, want)
		}
	}
}

func TestDownloadArgumentsFetchesAddresses(t *testing.T) {
	srv := httptest.NewServer(http.FileServer(http.Dir("../../examples")))
	defer srv.Close()
	var folder fetchFolder
	t.Cleanup(func() { folder.discard(nil) })
	files, origins, err := downloadArguments(context.Background(), []string{"../../examples/shapes.svg", srv.URL + "/shapes.svg"}, &folder, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 || files[0] != "../../examples/shapes.svg" {
		t.Fatalf("files=%v", files)
	}
	if folder.dir == "" || !folder.holds(files[1]) || filepath.Base(files[1]) != "shapes.svg" {
		t.Fatalf("download %q is not in the fetch folder %q", files[1], folder.dir)
	}
	if _, err := os.Stat(files[1]); err != nil {
		t.Fatal(err)
	}
	if origins[files[1]] != srv.URL+"/shapes.svg" || len(origins) != 1 {
		t.Fatalf("origins=%v", origins)
	}
	if folder.holds("../../examples/shapes.svg") || folder.holds(folder.dir+"/../x") {
		t.Fatal("holds claims files outside the folder")
	}
	folder.discard(nil)
	if _, err := os.Stat(folder.dir); !os.IsNotExist(err) {
		t.Fatalf("folder left behind: %v", err)
	}
}

func TestDownloadArgumentsLeavesFilesAlone(t *testing.T) {
	var folder fetchFolder
	files, _, err := downloadArguments(context.Background(), []string{"a.png", "-", `C:\b.png`}, &folder, "", nil)
	if err != nil || strings.Join(files, " ") != `a.png - C:\b.png` {
		t.Fatalf("files=%v err=%v", files, err)
	}
	if folder.dir != "" {
		t.Fatal("a folder was made with nothing to download")
	}
}

func TestDownloadArgumentsRefusedByEnvironment(t *testing.T) {
	var hit atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hit.Store(true) }))
	defer srv.Close()
	t.Setenv("GLOSS_NO_FETCH", "1")
	var folder fetchFolder
	t.Cleanup(func() { folder.discard(nil) })
	_, _, err := downloadArguments(context.Background(), []string{srv.URL + "/a.png"}, &folder, "", nil)
	if err == nil || !strings.Contains(err.Error(), "GLOSS_NO_FETCH") {
		t.Fatalf("err=%v", err)
	}
	if hit.Load() || folder.dir != "" {
		t.Fatal("the network was used while disabled")
	}
	t.Setenv("GLOSS_NO_FETCH", "")
	if _, _, err := downloadArguments(context.Background(), []string{srv.URL + "/a.png"}, &folder, "", nil); !hit.Load() || err != nil && strings.Contains(err.Error(), "GLOSS_NO_FETCH") {
		t.Fatalf("an empty GLOSS_NO_FETCH still disables: hit=%v err=%v", hit.Load(), err)
	}
}

func TestDownloadArgumentsOtherScheme(t *testing.T) {
	var folder fetchFolder
	_, _, err := downloadArguments(context.Background(), []string{"ftp://example.com/a.png"}, &folder, "", nil)
	if err == nil || !strings.Contains(err.Error(), "only http and https") {
		t.Fatalf("err=%v", err)
	}
}

func TestDownloadArgumentsReportsFailure(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	var folder fetchFolder
	t.Cleanup(func() { folder.discard(nil) })
	_, _, err := downloadArguments(context.Background(), []string{srv.URL + "/missing.png"}, &folder, "", nil)
	if err == nil || !strings.Contains(err.Error(), "HTTP 404") || !strings.Contains(err.Error(), srv.URL) {
		t.Fatalf("err=%v", err)
	}
}

func TestDownloadArgumentsRedactsCredentials(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	var folder fetchFolder
	t.Cleanup(func() { folder.discard(nil) })
	address := strings.Replace(srv.URL, "http://", "http://user:secret@", 1) + "/missing.png"
	_, _, err := downloadArguments(context.Background(), []string{address}, &folder, "", nil)
	if err == nil || !strings.Contains(err.Error(), "HTTP 404") || strings.Contains(err.Error(), "secret") || !strings.Contains(err.Error(), "user:xxxxx@") {
		t.Fatalf("err=%v", err)
	}
}

func TestDownloadArgumentsNeedsAHost(t *testing.T) {
	var folder fetchFolder
	_, _, err := downloadArguments(context.Background(), []string{"http:///a.png"}, &folder, "", nil)
	if err == nil || !strings.Contains(err.Error(), "no host") {
		t.Fatalf("err=%v", err)
	}
}

func TestDownloadArgumentsCanKeepGoing(t *testing.T) {
	srv := httptest.NewServer(http.FileServer(http.Dir("../../examples")))
	defer srv.Close()
	var folder fetchFolder
	t.Cleanup(func() { folder.discard(nil) })
	failed := map[string]error{}
	args := []string{srv.URL + "/missing.png", "../../examples/shapes.svg", srv.URL + "/shapes.svg"}
	files, origins, err := downloadArguments(context.Background(), args, &folder, "", func(address string, err error) { failed[address] = err })
	if err != nil || len(files) != 3 || files[0] != args[0] || files[1] != args[1] || !folder.holds(files[2]) {
		t.Fatalf("files=%v err=%v", files, err)
	}
	if len(failed) != 1 || failed[args[0]] == nil || !strings.Contains(failed[args[0]].Error(), "HTTP 404") {
		t.Fatalf("failed=%v", failed)
	}
	if origins[files[2]] != args[2] {
		t.Fatalf("origins=%v", origins)
	}
}

func TestDownloadArgumentsSendTheUserAgent(t *testing.T) {
	agent := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		agent <- r.UserAgent()
		http.FileServer(http.Dir("../../examples")).ServeHTTP(w, r)
	}))
	defer srv.Close()
	folder := fetchFolder{agent: "custom/2"}
	t.Cleanup(func() { folder.discard(nil) })
	if _, _, err := downloadArguments(context.Background(), []string{srv.URL + "/shapes.svg"}, &folder, "", nil); err != nil {
		t.Fatal(err)
	}
	if got := <-agent; got != "custom/2" {
		t.Fatalf("User-Agent = %q", got)
	}
}

func TestUserAgentOption(t *testing.T) {
	t.Setenv("GLOSS_USER_AGENT", "")
	opts, _, err := parse([]string{"a.png"}, io.Discard)
	if err != nil || opts.UserAgent != "gloss/"+version {
		t.Fatalf("default agent %q, %v", opts.UserAgent, err)
	}
	t.Setenv("GLOSS_USER_AGENT", "env/1")
	if opts, _, err = parse([]string{"a.png"}, io.Discard); err != nil || opts.UserAgent != "env/1" {
		t.Fatalf("environment agent %q, %v", opts.UserAgent, err)
	}
	if opts, _, err = parse([]string{"--user-agent", "flag/1", "a.png"}, io.Discard); err != nil || opts.UserAgent != "flag/1" {
		t.Fatalf("flag over environment %q, %v", opts.UserAgent, err)
	}
	if opts, _, err = parse([]string{"--user-agent", "", "a.png"}, io.Discard); err != nil || opts.UserAgent != "env/1" {
		t.Fatalf("empty flag %q, %v", opts.UserAgent, err)
	}
	for _, bad := range []string{"two\nlines", "tab\there", strings.Repeat("x", 1025)} {
		if _, _, err := parse([]string{"--user-agent", bad, "a.png"}, io.Discard); err == nil || !strings.Contains(err.Error(), "--user-agent") {
			t.Fatalf("%q accepted: %v", bad, err)
		}
	}
	t.Setenv("GLOSS_USER_AGENT", "bad\x7fagent")
	if _, _, err := parse([]string{"a.png"}, io.Discard); err == nil || !strings.Contains(err.Error(), "GLOSS_USER_AGENT") {
		t.Fatalf("bad environment agent accepted: %v", err)
	}
}

func TestFetchFolderIsSafeForConcurrentUse(t *testing.T) {
	// A fetch command makes the folder on one goroutine while the model
	// asks on another whether a path is a download.
	var folder fetchFolder
	t.Cleanup(func() { folder.discard(nil) })
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(2)
		go func() { defer wg.Done(); folder.path() }()
		go func() { defer wg.Done(); folder.holds("/nowhere/x.png") }()
	}
	wg.Wait()
	if folder.dir == "" {
		t.Fatal("no folder")
	}
}

func TestDescribeReportsDownloads(t *testing.T) {
	address := "https://example.invalid/missing.pdf"
	opts := options{Options: app.Options{Files: []string{address, "../../examples/shapes.svg"}, Page: 1, DPI: 72}, JSON: true}
	opts.Failed = map[string]string{address: "example.invalid: HTTP 404"}
	opts.Origins = map[string]string{"../../examples/shapes.svg": "https://example.invalid/shapes.svg"}
	var stdout, stderr bytes.Buffer
	err := describe(opts, &stdout, &stderr)
	if err == nil {
		t.Fatal("a failed download should make the exit status 1")
	}
	var got []struct{ Path, Kind, Error string }
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("%v\n%s", err, stdout.String())
	}
	if len(got) != 2 || got[0].Path != address || got[0].Error != "example.invalid: HTTP 404" || got[1].Path != "https://example.invalid/shapes.svg" || got[1].Kind != "svg" || got[1].Error != "" {
		t.Fatalf("%+v", got)
	}
}

func TestDetachDownloadsAddressesIntoTheSession(t *testing.T) {
	srv := httptest.NewServer(http.FileServer(http.Dir("../../examples")))
	defer srv.Close()
	dir := t.TempDir()
	args, err := detachArguments(context.Background(), []string{"--serve", srv.URL + "/shapes.svg", "-"}, dir, strings.NewReader("hello\n"), "", "gloss/test")
	if err != nil {
		t.Fatal(err)
	}
	if len(args) != 3 || args[0] != "--serve" || filepath.Dir(args[1]) != dir || filepath.Base(args[1]) != "shapes.svg" || args[2] != filepath.Join(dir, "stdin") {
		t.Fatalf("args=%v", args)
	}
	for _, path := range args[1:] {
		if _, err := os.Stat(path); err != nil {
			t.Fatal(err)
		}
	}
	// A failed download fails the start before any server runs.
	if _, err := detachArguments(context.Background(), []string{srv.URL + "/missing.png"}, dir, strings.NewReader(""), "", "gloss/test"); err == nil || !strings.Contains(err.Error(), "HTTP 404") {
		t.Fatalf("err=%v", err)
	}
}

func TestParseRefusesFetchWhenDisabled(t *testing.T) {
	t.Setenv("GLOSS_NO_FETCH", "yes")
	if _, _, err := parse([]string{"--fetch", "a.png"}, io.Discard); err == nil || !strings.Contains(err.Error(), "GLOSS_NO_FETCH") {
		t.Fatalf("err=%v", err)
	}
	if _, _, err := parse([]string{"a.png"}, io.Discard); err != nil {
		t.Fatalf("without --fetch: %v", err)
	}
	t.Setenv("GLOSS_NO_FETCH", "")
	if _, _, err := parse([]string{"--fetch", "a.png"}, io.Discard); err != nil {
		t.Fatalf("empty GLOSS_NO_FETCH: %v", err)
	}
}
