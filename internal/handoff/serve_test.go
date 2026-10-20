package handoff

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"nimbleterminal.dev/gloss/internal/app"
	"nimbleterminal.dev/gloss/internal/document"
)

func serving(t *testing.T, opts app.Options) *Server {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	opts.Render, opts.Render3D, opts.Page, opts.DPI = "glyph", "software", 1, 72
	s, err := serve(ctx, opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		s.Close()
		s.Discard()
	})
	return s
}

func TestServedRequestIsAccessibleAndEscaped(t *testing.T) {
	request := "Choose <script>alert('invoice')</script> & check totals\n" + strings.Repeat("Full request. ", 80)
	s := &Server{token: "test-token", prompt: request, pick: true}
	r := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:1234/test-token/", nil)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	page := w.Body.String()
	if w.Code != http.StatusOK || !strings.Contains(page, `<h1 id="request">Choose &lt;script&gt;`) ||
		strings.Contains(page, "<script>alert") || strings.Count(page, "Full request.") != 80 ||
		!strings.Contains(page, "aria-labelledby=\"request\"") || !strings.Contains(page, "press Enter in the viewer to send") {
		t.Fatalf("request missing, truncated, or unsafe: %d\n%s", w.Code, page)
	}
	for _, asset := range []string{"pickers.mjs", "drop.mjs"} {
		w = httptest.NewRecorder()
		s.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "http://127.0.0.1:1234/test-token/"+asset, nil))
		if w.Code != http.StatusOK {
			t.Errorf("picker dependency %s: %d", asset, w.Code)
		}
	}
}

func TestServedPageNeedsItsToken(t *testing.T) {
	s := serving(t, app.Options{Pick: true})
	u, err := url.Parse(s.URL)
	if err != nil || u.Hostname() != "127.0.0.1" || len(strings.Trim(u.Path, "/")) < 32 {
		t.Fatalf("URL %q must be on this machine and hard to guess", s.URL)
	}
	root := "http://" + u.Host
	for _, path := range []string{"/", "/drop.mjs", "/ws", "/static/booba/booba.js", "/" + strings.Repeat("0", 32) + "/"} {
		if code, _ := get(t, root+path, nil); code != http.StatusNotFound {
			t.Errorf("GET %s without the token: %d", path, code)
		}
	}
	if code, _ := drop(t, root+"/drop", nil, upload{"a.png", picture(t)}); code != http.StatusNotFound {
		t.Errorf("a drop without the token: %d", code)
	}
	code, page := get(t, s.URL, nil)
	if code != 200 || !strings.Contains(page, "Drop files") || !strings.Contains(page, "serve.mjs") {
		t.Fatalf("page: %d\n%s", code, page)
	}
	for _, path := range []string{"serve.mjs", "drop.mjs", "upload.mjs", "static/booba/booba.js", "static/ghostty-web/ghostty-web.js"} {
		if code, body := get(t, s.URL+path, nil); code != 200 || len(body) < 100 {
			t.Errorf("GET %s: %d, %d bytes", path, code, len(body))
		}
	}
	// A page elsewhere that resolves its own name to this machine is refused.
	if code, _ := get(t, s.URL, func(r *http.Request) { r.Host = "evil.example:" + u.Port() }); code != http.StatusForbidden {
		t.Errorf("a foreign Host was served: %d", code)
	}
	// The viewer behind the page answers only to the page's server.
	if _, _, err := websocket.Dial(context.Background(), "ws://"+s.backend+"/ws", nil); err == nil {
		t.Error("the viewer can be reached around the token")
	}
}

func TestServedDropsAreKeptForTheCaller(t *testing.T) {
	s := serving(t, app.Options{Pick: true})
	code, paths := drop(t, s.URL+"drop", nil, upload{"../../etc/pass wd.png", picture(t)}, upload{`C:\Users\me\plan.svg`, []byte("<svg xmlns='http://www.w3.org/2000/svg'/>")}, upload{"pass wd.png", []byte("another")})
	if code != 200 || len(paths) != 3 {
		t.Fatalf("code=%d paths=%q", code, paths)
	}
	dir := filepath.Dir(paths[0])
	for i, want := range []string{"pass wd.png", "plan.svg", "pass wd-2.png"} {
		if filepath.Base(paths[i]) != want || filepath.Dir(paths[i]) != dir || !filepath.IsAbs(paths[i]) {
			t.Errorf("file %d kept as %q, want %q beside the others", i, paths[i], want)
		}
	}
	if got, err := os.ReadFile(paths[0]); err != nil || !bytes.Equal(got, picture(t)) {
		t.Errorf("contents: %v", err)
	}
	if info, err := os.Stat(dir); err != nil || info.Mode().Perm() != 0700 || !strings.HasPrefix(filepath.Base(dir), "gloss-") {
		t.Errorf("the folder of drops must be the user's alone: %v %v", info, err)
	}
	// They outlive the server: the caller has yet to read them.
	s.Close()
	if _, err := os.Stat(paths[0]); err != nil {
		t.Errorf("the drop went with the server: %v", err)
	}
	os.RemoveAll(dir)
}

func TestServedDropsAreBounded(t *testing.T) {
	s := serving(t, app.Options{Pick: true})
	if code, paths := drop(t, s.URL+"drop", nil, upload{"huge.png", make([]byte, document.MaxFileBytes+1)}); code != http.StatusRequestEntityTooLarge || len(paths) != 0 {
		t.Errorf("a drop over 128 MiB: %d %q", code, paths)
	}
	if code, paths := drop(t, s.URL+"drop", nil, upload{"empty.png", nil}, upload{"..", []byte("x")}); code != http.StatusBadRequest || len(paths) != 0 {
		t.Errorf("nothing usable: %d %q", code, paths)
	}
	cross := func(r *http.Request) { r.Header.Set("Origin", "https://evil.example") }
	if code, paths := drop(t, s.URL+"drop", cross, upload{"a.png", picture(t)}); code != http.StatusForbidden || len(paths) != 0 {
		t.Errorf("a drop from another site: %d %q", code, paths)
	}
	if code, _ := get(t, s.URL+"drop", nil); code != http.StatusMethodNotAllowed {
		t.Errorf("GET drop: %d", code)
	}
}

func TestServedPickFromDropToAnswer(t *testing.T) {
	s := serving(t, app.Options{Pick: true})
	term := connect(t, s)
	term.Until("Drop a file here to send it")
	code, paths := drop(t, s.URL+"drop", nil, upload{"holiday.png", picture(t)})
	if code != 200 || len(paths) != 1 {
		t.Fatalf("code=%d paths=%q", code, paths)
	}
	defer os.RemoveAll(filepath.Dir(paths[0]))
	term.Until("Enter send holiday.png")
	term.Send('0', "\r")
	m, err := s.Wait(5 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if got := m.Picked(); len(got) != 1 || got[0] != paths[0] {
		t.Fatalf("picked %q, want %q", got, paths)
	}
	if resp, err := http.Get(s.URL); err == nil {
		resp.Body.Close()
		t.Errorf("the server outlived its purpose: %d", resp.StatusCode)
	}
	if conn, err := net.Dial("tcp", s.backend); err == nil {
		conn.Close()
		t.Error("the viewer outlived its purpose")
	}
}

func TestServedPickCancelledAndUnanswered(t *testing.T) {
	s := serving(t, app.Options{Pick: true})
	term := connect(t, s)
	term.Until("Drop a file here to send it")
	term.Send('0', "q")
	if m, err := s.Wait(5 * time.Second); err != nil || m == nil || m.Picked() != nil {
		t.Fatalf("quitting: %v %v", m, err)
	}
	closed := serving(t, app.Options{Pick: true})
	term = connect(t, closed)
	term.Until("Drop a file here to send it")
	term.Close() // The tab is closed.
	if m, err := closed.Wait(5 * time.Second); err != nil || m == nil || m.Picked() != nil {
		t.Fatalf("closing the tab: %v %v", m, err)
	}
	ignored := serving(t, app.Options{Pick: true})
	if _, err := ignored.Wait(200 * time.Millisecond); err != ErrTimeout {
		t.Fatalf("nobody came: %v", err)
	}
}

func TestServedViewerShowsTheCallersFile(t *testing.T) {
	s := serving(t, app.Options{Files: []string{"../../examples/shapes.svg"}})
	term := connect(t, s)
	term.Until("shapes.svg")
	term.Send('0', "i")
	term.Until("View box")
	term.Send('0', "q")
	if m, err := s.Wait(5 * time.Second); err != nil || m.Picked() != nil {
		t.Fatalf("%v %v", m, err)
	}
}

func TestExitCodes(t *testing.T) {
	for err, want := range map[error]int{nil: 0, ErrCancelled: 2, ErrTimeout: 124, fmt.Errorf("pick: %w", ErrCancelled): 2, io.ErrUnexpectedEOF: 1} {
		if got := ExitCode(err); got != want {
			t.Errorf("exitCode(%v) = %d, want %d", err, got, want)
		}
	}
}

func TestServeFilterRejectsAndRemovesWholeDrop(t *testing.T) {
	s := serving(t, app.Options{Accept: "image/*"})
	code, paths := drop(t, s.URL+"drop", nil, upload{"ok.png", picture(t)}, upload{"lie.png", []byte("<!doctype html><html>Sign in</html>")})
	if code != http.StatusUnsupportedMediaType || len(paths) != 0 {
		t.Fatalf("status=%d paths=%v", code, paths)
	}
	s.mu.Lock()
	dir := s.dir
	s.mu.Unlock()
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 || s.stored.Load() != 0 {
		t.Fatalf("rejected files retained: %v %v, bytes=%d", entries, err, s.stored.Load())
	}
}

func TestServedURLDropsAreOptIn(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		s := &Server{token: "test-token", fetch: enabled, accept: "image/*"}
		w := httptest.NewRecorder()
		s.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "http://127.0.0.1:1234/test-token/", nil))
		if !strings.Contains(w.Body.String(), fmt.Sprintf(`data-fetch="%t"`, enabled)) || !strings.Contains(w.Body.String(), "Required formats: image/*") {
			t.Fatal(w.Body.String())
		}
		w = httptest.NewRecorder()
		s.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "http://127.0.0.1:1234/test-token/remote.mjs", nil))
		if w.Code != http.StatusOK {
			t.Fatal(w.Code)
		}
	}
}

// An export made in a served viewer goes to the page, to be downloaded: the
// server's working directory is not the visitor's.
func TestServedExportsAreDownloadedByThePage(t *testing.T) {
	wait := exportWait
	exportWait = 50 * time.Millisecond
	t.Cleanup(func() { exportWait = wait })
	s := &Server{token: "test-token", exported: make(chan struct{})}
	get := func(rest string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		s.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "http://127.0.0.1:1234/test-token/"+rest, nil))
		return w
	}
	if w := get("exports?after=0"); w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("none yet: %d %q", w.Code, w.Body.String())
	}
	// The page's question is answered as soon as there is one.
	exportWait = time.Minute
	answered := make(chan string, 1)
	go func() { answered <- get("exports?after=0").Body.String() }()
	time.Sleep(10 * time.Millisecond)
	name, err := s.offer("page-2.png", []byte("\x89PNG one"))
	if err != nil || !strings.Contains(name, "page-2.png") {
		t.Fatalf("offer: %q %v", name, err)
	}
	select {
	case list := <-answered:
		if !strings.Contains(list, `{"id":0,"name":"page-2.png"}`) {
			t.Fatalf("list: %s", list)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the page was not told")
	}
	s.offer("page-3.png", []byte("\x89PNG two"))
	if list := get("exports?after=1").Body.String(); strings.Contains(list, "page-2") || !strings.Contains(list, `"id":1`) {
		t.Fatalf("after 1: %s", list)
	}
	w := get("export/1")
	if w.Code != http.StatusOK || w.Body.String() != "\x89PNG two" || w.Header().Get("Content-Type") != "image/png" || !strings.Contains(w.Header().Get("Content-Disposition"), `attachment; filename=page-3.png`) {
		t.Fatalf("export: %d %v %q", w.Code, w.Header(), w.Body.String())
	}
	for _, bad := range []string{"export/2", "export/-1", "export/x"} {
		if w := get(bad); w.Code != http.StatusNotFound {
			t.Errorf("%s: %d", bad, w.Code)
		}
	}
	// Without the token there is nothing.
	w = httptest.NewRecorder()
	s.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "http://127.0.0.1:1234/other-token/export/0", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("no token: %d", w.Code)
	}
}
