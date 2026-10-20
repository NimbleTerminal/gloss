package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"nimbleterminal.dev/gloss/internal/app"
	"nimbleterminal.dev/gloss/internal/handoff"
	"nimbleterminal.dev/gloss/internal/handoff/handofftest"
)

// testToken has the form of a session token, for the flags that take one.
const testToken = "0123456789abcdef0123456789abcdef"

// The page, driven from outside, as the handoff package's tests drive it.
type (
	terminal = handofftest.Terminal
	lines    = handofftest.Lines
)

var (
	get     = handofftest.Get
	picture = handofftest.Picture
)

// upload is a file to drop, written as {name, data}.
type upload struct {
	name string
	data []byte
}

func drop(t *testing.T, target string, edit func(*http.Request), files ...upload) (int, []string) {
	t.Helper()
	out := make([]handofftest.Upload, len(files))
	for i, f := range files {
		out[i] = handofftest.Upload{Name: f.name, Data: f.data}
	}
	return handofftest.Drop(t, target, edit, out...)
}

type webPickLog chan string

func (c webPickLog) Write(p []byte) (int, error) {
	c <- string(p)
	return len(p), nil
}

func TestResumeFlags(t *testing.T) {
	opts, _, err := parse([]string{"--resume", testToken, "--timeout", "5s", "--json"}, &bytes.Buffer{})
	if err != nil || opts.Resume != testToken || opts.Timeout != 5*time.Second {
		t.Fatalf("%+v %v", opts, err)
	}
	for _, args := range [][]string{{"--resume", testToken, "a.svg"}, {"--resume", testToken, "--pick"}, {"--resume", testToken, "--text"}, {"--detached", testToken}, {"--serve", "--detached", "nope"}} {
		if _, _, err := parse(args, &bytes.Buffer{}); err == nil {
			t.Errorf("accepted %v", args)
		}
	}
}

func TestPickNetworkFlags(t *testing.T) {
	for _, args := range [][]string{
		{}, {"--listen", "192.168.1.42:0"}, {"--listen", "[::1]:8080"},
		{"--listen", "0.0.0.0:0", "--advertise-host", "laptop.example.ts.net"},
		{"--listen", "[::]:9090", "--advertise-host", "[fd7a:115c:a1e0::1]"},
	} {
		if _, _, err := parse(append([]string{"--pick-web"}, args...), io.Discard); err != nil {
			t.Errorf("%v: %v", args, err)
		}
	}
	for _, args := range [][]string{
		{"--listen", ""}, {"--listen", "localhost:0"}, {"--listen", "en0:0"},
		{"--listen", ":8080"}, {"--listen", "127.0.0.1:65536"},
		{"--listen", "0.0.0.0:0"}, {"--listen", "[::]:0"},
		{"--advertise-host", ""}, {"--advertise-host", "https://laptop.ts.net"},
		{"--advertise-host", "laptop:80"}, {"--advertise-host", "laptop/path"},
		{"--advertise-host", "0.0.0.0"}, {"--advertise-host", "::"},
		{"--advertise-host", "user@laptop"}, {"--advertise-host", "laptop?x=1"},
		{"--advertise-host", "laptop#fragment"}, {"--advertise-host", "☕.example"},
		{"--advertise-host", "*.example"}, {"--advertise-host", "bad..example"},
		{"--advertise-host", "-bad.example"}, {"--advertise-host", "bad-.example"},
		{"--advertise-host", "127.1"}, {"--advertise-host", "2130706433"},
		{"--advertise-host", "laptop\n.example"},
		{"--listen", "224.0.0.1:0"}, {"--listen", "255.255.255.255:0"},
		{"--listen", "169.254.1.2:0"}, {"--listen", "[fe80::1%en0]:0"},
		{"--listen", "[::]:0", "--advertise-host", "192.168.1.42"},
		{"--listen", "0.0.0.0:0", "--advertise-host", "::1"},
		{"--listen", "192.168.1.42:0", "--advertise-host", "192.168.1.43"},
		{"--listen", "0.0.0.0:0", "--advertise-host", "127.0.0.1"},
		{"--listen", "0.0.0.0:0", "--advertise-host", "127.0.0.2"},
		{"--listen", "0.0.0.0:0", "--advertise-host", "LOCALHOST."},
		{"--listen", "[::]:0", "--advertise-host", "::1"},
	} {
		if _, _, err := parse(append([]string{"--pick-web"}, args...), io.Discard); err == nil {
			t.Errorf("accepted %v", args)
		}
	}
	for _, mode := range [][]string{nil, {"--serve"}, {"--pick"}, {"--resume", testToken}, {"--status", testToken}, {"--cancel", testToken}} {
		for _, flags := range [][]string{{"--listen", handoff.DefaultListen}, {"--advertise-host", "localhost"}} {
			if _, _, err := parse(append(append([]string{}, mode...), flags...), io.Discard); err == nil {
				t.Errorf("accepted network flags outside web pick: %v %v", mode, flags)
			}
		}
	}
}

func TestForegroundWebPickJSONMessage(t *testing.T) {
	var stdout bytes.Buffer
	log, done := make(webPickLog, 1), make(chan error, 1)
	go func() {
		done <- served(options{Options: app.Options{Pick: true}, PickWeb: true, JSON: true, NoOpen: true, Timeout: 5 * time.Second}, &stdout, log)
	}()
	var address string
	select {
	case line := <-log:
		address = strings.TrimSpace(strings.TrimPrefix(line, "gloss: viewer at "))
	case err := <-done:
		t.Fatalf("server stopped before listening: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("server did not announce itself")
	}
	code, _ := drop(t, address+"files", nil, upload{"receipt.txt", []byte("receipt")})
	if code != 200 {
		t.Fatalf("upload: %d", code)
	}
	_, body := get(t, address+"files", nil)
	var list struct{ Files []struct{ ID string } }
	if err := json.Unmarshal([]byte(body), &list); err != nil || len(list.Files) != 1 {
		t.Fatalf("files: %s %v", body, err)
	}
	message := "The invoice is attached.\nPlease check the date."
	confirm, _ := json.Marshal(map[string]any{"ids": []string{list.Files[0].ID}, "message": message})
	response, err := http.Post(address+"confirm", "application/json", bytes.NewReader(confirm))
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, response.Body)
	response.Body.Close()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	var result handoff.Resumed
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil || result.Status != "picked" || result.Message != message || len(result.Paths) != 1 {
		t.Fatalf("foreground answer: %s %v", stdout.String(), err)
	}
	t.Cleanup(func() { os.RemoveAll(filepath.Dir(result.Paths[0])) })
	if data, err := os.ReadFile(result.Paths[0]); err != nil || string(data) != "receipt" {
		t.Fatalf("file not retained: %q %v", data, err)
	}
}

func TestWebPickFlags(t *testing.T) {
	opts, _, err := parse([]string{"--pick-web", "--no-open", "--prompt", "Invoice, please", "--accept", "pdf"}, io.Discard)
	if err != nil || !opts.Serve || !opts.Pick || !opts.PickWeb || opts.Accept != "pdf" {
		t.Fatalf("parse: %+v %v", opts, err)
	}
	if opts, _, err := parse([]string{"--pick-web", "--json"}, io.Discard); err != nil || !opts.JSON {
		t.Fatalf("JSON web pick: %+v %v", opts, err)
	}
	for _, args := range [][]string{
		{"invoice.pdf"}, {"-"}, {"--glob", "*.png"}, {"--fetch"}, {"--menu"}, {"--preview"},
		{"--info"}, {"--text"}, {"--output", "a.png"}, {"--resume", testToken}, {"--status", testToken}, {"--cancel", testToken},
	} {
		if _, _, err := parse(append([]string{"--pick-web"}, args...), io.Discard); err == nil {
			t.Errorf("accepted --pick-web %v", args)
		}
	}
}

// asked runs gloss as a program would to ask for a file, and plays the user.
func asked(t *testing.T, user func(term *terminal, page string) []string) (answer string, dropped []string, err error) {
	t.Helper()
	opts, _, parseErr := parse([]string{"--serve", "--pick", "--no-open", "--render", "glyph", "--timeout", "30s"}, &bytes.Buffer{})
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	var stdout, stderr lines
	ended := make(chan error, 1)
	go func() { ended <- served(opts, &stdout, &stderr) }()
	page := ""
	for start := time.Now(); page == ""; time.Sleep(10 * time.Millisecond) {
		if _, after, ok := strings.Cut(stderr.String(), "viewer at "); ok && strings.Contains(after, "\n") {
			page = strings.TrimSpace(after)
		}
		if time.Since(start) > 10*time.Second {
			t.Fatalf("no address: %q", stderr.String())
		}
	}
	term := handofftest.Connect(t, page)
	term.Until("Drop a file here to send it")
	dropped = user(term, page)
	select {
	case err = <-ended:
	case <-time.After(10 * time.Second):
		t.Fatal("gloss did not finish")
	}
	return stdout.String(), dropped, err
}

func TestAskedForAFile(t *testing.T) {
	answer, dropped, err := asked(t, func(term *terminal, page string) []string {
		_, paths := drop(t, page+"drop", nil, upload{"holiday.png", picture(t)})
		term.Until("Enter send holiday.png")
		term.Send('0', "\r")
		return paths
	})
	if err != nil || len(dropped) != 1 || answer != dropped[0]+"\n" {
		t.Fatalf("answer %q, dropped %q, err %v", answer, dropped, err)
	}
	if got, err := os.ReadFile(dropped[0]); err != nil || !bytes.Equal(got, picture(t)) {
		t.Fatalf("the answer cannot be read: %v", err)
	}
	os.RemoveAll(filepath.Dir(dropped[0]))
}

func TestAskedAndDeclined(t *testing.T) {
	answer, dropped, err := asked(t, func(term *terminal, page string) []string {
		_, paths := drop(t, page+"drop", nil, upload{"private.png", picture(t)})
		term.Until("Enter send private.png")
		term.Send('0', "q")
		return paths
	})
	if err != handoff.ErrCancelled || answer != "" || len(dropped) != 1 {
		t.Fatalf("answer %q, dropped %q, err %v", answer, dropped, err)
	}
	// What was dropped and then withheld is not left lying about.
	if _, err := os.Stat(filepath.Dir(dropped[0])); !os.IsNotExist(err) {
		t.Fatalf("the withheld file was kept: %v", err)
	}
}

func TestAcceptFlag(t *testing.T) {
	opts, _, err := parse([]string{"--accept", "image/*,pdf", "--fetch", "--pick"}, &bytes.Buffer{})
	if err != nil || opts.Accept != "image/*,pdf" {
		t.Fatalf("filter=%q err=%v", opts.Accept, err)
	}
	if _, _, err := parse([]string{"--accept", "nonsense"}, &bytes.Buffer{}); err == nil {
		t.Fatal("unknown filter accepted")
	}
}

func TestFetchedFilesAreRemovedUnlessPicked(t *testing.T) {
	dir := t.TempDir()
	keep, drop := filepath.Join(dir, "keep.png"), filepath.Join(dir, "drop.png")
	for _, p := range []string{keep, drop} {
		if err := os.WriteFile(p, []byte("x"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	discardFetched(dir, []string{keep})
	if _, err := os.Stat(keep); err != nil {
		t.Error("the picked file was removed")
	}
	if _, err := os.Stat(drop); !os.IsNotExist(err) {
		t.Error("the unpicked file was kept")
	}
	if _, err := os.Stat(dir); err != nil {
		t.Error("the folder went while a picked file was still in it")
	}
	discardFetched(dir, nil)
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Error("an emptied folder was kept")
	}
}

func TestFetchFlag(t *testing.T) {
	opts, _, err := parse([]string{"--fetch", "a.csv"}, &bytes.Buffer{})
	if err != nil || !opts.FetchAllowed {
		t.Fatalf("%+v %v", opts, err)
	}
	if opts, _, _ := parse([]string{"a.csv"}, &bytes.Buffer{}); opts.FetchAllowed || opts.Fetch != nil {
		t.Fatal("fetching is on without being asked for")
	}
	if _, _, err := parse([]string{"--fetch", "-o", "out.png", "a.csv"}, &bytes.Buffer{}); err == nil {
		t.Fatal("--fetch accepted with an export, which fetches nothing")
	}
}

func TestPromptFlags(t *testing.T) {
	opts, _, err := parse([]string{"--pick", "--prompt", "Choose the March invoice"}, &bytes.Buffer{})
	if err != nil || opts.Prompt != "Choose the March invoice" || opts.PromptTop {
		t.Fatalf("%+v %v", opts, err)
	}
	opts, _, err = parse([]string{"--prompt", "Choose", "--prompt-loc", "top", "a.png"}, &bytes.Buffer{})
	if err != nil || !opts.PromptTop {
		t.Fatalf("top: %+v %v", opts, err)
	}
	for _, args := range [][]string{{"--prompt-loc", "left", "--prompt", "x"}, {"--prompt-loc", "top"}, {"--prompt", "x", "-o", "out.png", "a.png"}} {
		if _, _, err := parse(args, &bytes.Buffer{}); err == nil {
			t.Errorf("accepted %v", args)
		}
	}
}

func TestServeFlags(t *testing.T) {
	opts, _, err := parse([]string{"--serve", "--pick", "--no-open", "--timeout", "90s", "a.svg"}, &bytes.Buffer{})
	if err != nil || !opts.Pick || !opts.Serve || !opts.NoOpen || opts.Timeout != 90*time.Second || len(opts.Files) != 1 {
		t.Fatalf("%+v %v", opts, err)
	}
	if opts, _, err = parse([]string{"a.svg"}, &bytes.Buffer{}); err != nil || opts.Pick || opts.Serve || opts.NoOpen || opts.Timeout != 0 {
		t.Fatalf("defaults: %+v %v", opts, err)
	}
	for _, args := range [][]string{
		{"--serve", "-o", "out.png", "a.svg"}, {"--pick", "-O", "out", "a.svg"}, {"--serve", "-X"},
		{"--timeout", "-5s", "--pick"}, {"--timeout", "soon", "--pick"}, {"--no-open"}, {"--timeout", "5s", "a.svg"},
	} {
		if _, _, err := parse(args, &bytes.Buffer{}); err == nil {
			t.Errorf("accepted %v", args)
		}
	}
}

func TestStandardInputIsReadOnlyWhenItIsTheDocument(t *testing.T) {
	// Whatever starts gloss to ask for a file has no terminal to give it:
	// its standard input is a pipe, or nothing, and holds no document.
	for _, tt := range []struct {
		name     string
		args     []string
		terminal bool
		want     []string
	}{
		{"piped to the viewer", nil, false, []string{"-"}},
		{"at a terminal", nil, true, nil},
		{"asked for by name", []string{"--pick", "-"}, false, []string{"-"}},
		{"a pick started by a program", []string{"--pick"}, false, nil},
		{"a page started by a program", []string{"--serve"}, false, nil},
		{"a page for a file", []string{"--serve", "a.svg"}, false, []string{"a.svg"}},
		{"an export of a pipe", []string{"-o", "out.png"}, false, []string{"-"}},
	} {
		opts, _, err := parse(tt.args, &bytes.Buffer{})
		if err != nil {
			t.Fatal(err)
		}
		if got := arguments(opts, tt.terminal); strings.Join(got, " ") != strings.Join(tt.want, " ") {
			t.Errorf("%s: %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestStatusFlags(t *testing.T) {
	for _, args := range [][]string{
		{"--status", testToken, "a.svg"}, {"--status", testToken, "--pick"}, {"--status", testToken, "--serve"},
		{"--status", testToken, "--json"}, {"--status", testToken, "--timeout", "1s"}, {"--status", testToken, "--resume", testToken},
		{"--status", testToken, "--info"}, {"--status", testToken, "-o", "x.png"}, {"--status", testToken, "--prompt", "hi"},
	} {
		if _, _, err := parse(args, &bytes.Buffer{}); err == nil {
			t.Errorf("%v was accepted", args)
		}
	}
	if opts, done, err := parse([]string{"--status", testToken}, &bytes.Buffer{}); err != nil || done || opts.Status != testToken {
		t.Errorf("--status alone: %+v %v %v", opts, done, err)
	}
}
