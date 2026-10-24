package handoff

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"mime"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	tea "charm.land/bubbletea/v2"
	booba "github.com/NimbleMarkets/go-booba/serve"
	"nimbleterminal.dev/gloss/internal/app"
	"nimbleterminal.dev/gloss/internal/chart"
	"nimbleterminal.dev/gloss/internal/document"
	"nimbleterminal.dev/gloss/internal/i18n"
	"nimbleterminal.dev/gloss/web"
)

var (
	ErrCancelled = errors.New("nothing was chosen")
	ErrTimeout   = errors.New("timed out waiting for the viewer")
)

const maxDrops = 256 // Files in one drop.

var servePage = newLocalizedPage("serve.html")

// localizedPage is a page's template once per catalog, each with a t of its
// own language. They are made when first wanted: a clone must be taken
// before its original runs.
type localizedPage struct {
	name  string
	once  sync.Once
	pages map[string]*template.Template
}

func newLocalizedPage(name string) *localizedPage { return &localizedPage{name: name} }

// in returns the page in l's language.
func (p *localizedPage) in(l *i18n.Localizer) *template.Template {
	p.once.Do(func() {
		english := func(id, def string) string { return def }
		base := template.Must(template.New(p.name).Funcs(template.FuncMap{"t": english}).ParseFS(web.Page, p.name))
		p.pages = map[string]*template.Template{}
		for _, lang := range i18n.Languages() {
			loc := i18n.For(lang.Tag)
			p.pages[loc.Tag()] = template.Must(base.Clone()).Funcs(template.FuncMap{"t": loc.T})
		}
	})
	return p.pages[l.Tag()]
}

// webMessages are the strings a page's scripts translate. It is a variable
// so that tests can give it strings no catalog has.
var webMessages = func(l *i18n.Localizer) map[string]string { return l.Messages("web.") }

// pageText is what every page is told of its language: the catalog's tag,
// and the scripts' messages as JSON (which escapes <, > and &, so no
// message ends the element it is in).
type pageText struct {
	Lang     string
	Messages template.JS
}

func newPageText(l *i18n.Localizer) pageText {
	data, err := json.Marshal(webMessages(l))
	if err != nil {
		data = []byte("{}")
	}
	return pageText{Lang: l.Tag(), Messages: template.JS(data)}
}

// localizer is the language of the page r asks for: the viewer's when it
// has one, else the browser's best that a catalog serves. The browser's is
// kept, for the viewer that the page goes on to start.
func (s *Server) localizer(r *http.Request) *i18n.Localizer {
	if s.lang != "" {
		return i18n.For(s.lang)
	}
	tag := i18n.FromAcceptLanguage(r.Header.Get("Accept-Language"))
	s.mu.Lock()
	s.pageLang = tag
	s.mu.Unlock()
	return i18n.For(tag)
}

// viewerOptions are opts for a viewer session. One with no language of its
// own takes the page's, and the first makes it gloss's, so a detached viewer
// speaks as its page does. The language is gloss's, not the session's: later
// sessions keep the first one's, so a second browser cannot switch it.
func (s *Server) viewerOptions(opts app.Options) app.Options {
	if opts.Lang != "" {
		return opts
	}
	s.mu.Lock()
	if !s.langChosen {
		s.langChosen, s.viewerLang = true, s.pageLang
		if s.viewerLang != "" {
			i18n.SetLanguage(s.viewerLang)
		}
	}
	opts.Lang = s.viewerLang
	s.mu.Unlock()
	return opts
}

// maxSessionBytes is all that one session keeps from the page, over every
// drop: the token holder is trusted, but a page that loops should fill no disk.
// It is a variable so that tests can lower it.
var maxSessionBytes int64 = 1 << 30

// Server shows the viewer on a page of its own, for as long as it is wanted.
//
// Booba runs the viewer and carries its terminal to the page. It listens on
// a port of its own, and answers only to requests bearing a secret. Those
// come from the Server in front of it, which serves the page, takes what is
// dropped there, and admits nothing without the token in the page's address.
type Server struct {
	URL     string
	backend string
	prompt  string
	lang    string // The viewer's language, or "" to follow the browser.
	pick    bool
	fetch   bool
	accept  document.AcceptFilter
	webPick *webPick // Present only for the experimental upload-only page.

	token, secret string
	cancel        context.CancelFunc
	front         *http.Server
	proxy         *httputil.ReverseProxy
	drops         chan []string
	charts        chan app.ChartPush
	started, done chan struct{}
	once, ended   sync.Once
	logged        io.Writer // Where the process logger wrote before the session.
	lifetime      *viewerLifetime
	closeOnce     sync.Once

	mu       sync.Mutex
	model    *app.Model
	pageLang string // The browser's language, from the last page served.
	// viewerLang is the first viewer session's language, which every
	// session keeps; langChosen says it has been chosen.
	viewerLang string
	langChosen bool
	shown      *chart.Document // What the viewer's chart pane drew last; GET /chart draws it.
	chartURL   string          // Where that page is, for the viewer to name.
	dir        string          // Where drops are kept; made when the first arrives.
	stored     atomic.Int64    // Bytes kept so far, against maxSessionBytes.
	uploads    atomic.Int32    // Active requests, including bytes not committed yet.

	// Exports, made by e in the viewer, wait here for the page to take
	// them as downloads: the server's working directory is not the
	// visitor's, and may be no place for them.
	exports  []export
	exported chan struct{} // Closed, and replaced, when one is added.
}

type export struct {
	name string
	data []byte
}

// exportWait is how long the page's question for new exports is held open.
// It is a variable so that tests need not wait.
var exportWait = 25 * time.Second

func random() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func serve(ctx context.Context, opts app.Options) (*Server, error) {
	ctx, cancel := context.WithCancel(ctx)
	s := &Server{fetch: opts.Fetch != nil, accept: opts.Accept, cancel: cancel, prompt: opts.Prompt, lang: opts.Lang, pick: opts.Pick, drops: make(chan []string, 16), charts: make(chan app.ChartPush, 1), started: make(chan struct{}), done: make(chan struct{}), exported: make(chan struct{})}
	s.lifetime = newViewerLifetime()
	opts.OnQuit = func() { s.lifetime.finish("closed") }
	// The browser carries the full request as accessible page text, outside
	// the terminal's size and line limits.
	opts.Prompt = ""
	var err error
	if s.token, err = random(); err == nil {
		s.secret, err = random()
	}
	if err != nil {
		cancel()
		return nil, err
	}
	// Booba opens its own listener, so it is told of a port found free.
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		cancel()
		return nil, err
	}
	s.backend = probe.Addr().String()
	probe.Close()
	config := booba.DefaultConfig()
	config.Host, config.Port, config.HTTP3Port, config.MaxConnections = "127.0.0.1", probe.Addr().(*net.TCPAddr).Port, -1, 1
	opts.Drops, opts.Save = s.drops, s.offer
	opts.Charts, opts.Charted = s.charts, s.charted
	viewer := booba.NewServer(config, booba.WithConnectMiddleware(func(next booba.ConnectHandler) booba.ConnectHandler {
		return func(r *http.Request) error {
			if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Gloss-Secret")), []byte(s.secret)) != 1 {
				return errors.New("not from the page's server")
			}
			return next(r)
		}
	}))
	// Booba reports its comings and goings through the process logger,
	// which would land in the terminal beside the viewer; it is quieted for
	// the session and given back on Close.
	s.logged = log.Writer()
	log.SetOutput(io.Discard)
	failed := make(chan error, 1)
	go func() {
		failed <- viewer.Serve(ctx, func(session booba.Session) (tea.Model, []tea.ProgramOption) {
			s.mu.Lock()
			opts.ChartURL = s.chartURL
			s.mu.Unlock()
			m := app.New(s.viewerOptions(opts))
			s.mu.Lock()
			first := s.model == nil
			if first {
				s.model = m
			}
			s.mu.Unlock()
			if first {
				s.once.Do(func() { close(s.started) })
				go func() {
					// The session ends when the viewer quits or the tab closes.
					<-session.Context().Done()
					reason := "disconnected"
					if ctx.Err() != nil {
						reason = "closed"
					}
					s.lifetime.finish(reason)
					s.ended.Do(func() { close(s.done) })
				}()
			}
			return m, nil
		})
	}()
	for start := time.Now(); ; time.Sleep(10 * time.Millisecond) {
		conn, err := net.Dial("tcp", s.backend)
		if err == nil {
			conn.Close()
			break
		}
		select {
		case err := <-failed:
			cancel()
			return nil, fmt.Errorf("viewer: %w", err)
		default:
		}
		if time.Since(start) > 5*time.Second {
			cancel()
			return nil, fmt.Errorf("viewer did not start: %w", err)
		}
	}
	target := &url.URL{Scheme: "http", Host: s.backend}
	s.proxy = &httputil.ReverseProxy{Rewrite: func(r *httputil.ProxyRequest) {
		r.SetURL(target)
		r.Out.Host = r.In.Host // Booba admits pages of its own host.
		r.Out.Header.Set("X-Gloss-Secret", s.secret)
	}, ErrorLog: log.New(io.Discard, "", 0)}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		cancel()
		return nil, err
	}
	s.URL = "http://" + listener.Addr().String() + "/" + s.token + "/"
	s.mu.Lock()
	s.chartURL = s.URL + "chart"
	s.mu.Unlock()
	s.front = &http.Server{Handler: s, ReadHeaderTimeout: 10 * time.Second, ReadTimeout: uploadIdleTimeout, IdleTimeout: 30 * time.Second, ErrorLog: log.New(io.Discard, "", 0)}
	go s.front.Serve(listener)
	go func() {
		<-ctx.Done()
		s.Close()
	}()
	return s, nil
}

// Wait returns the viewer once its session is over. A timeout of zero waits
// as long as it takes.
func (s *Server) Wait(timeout time.Duration) (*app.Model, error) {
	s.lifetime.configure(timeout, 0, nil)
	for {
		s.lifetime.mu.Lock()
		deadline := s.lifetime.deadline
		s.lifetime.mu.Unlock()
		var timer *time.Timer
		var late <-chan time.Time
		if !deadline.IsZero() {
			timer = time.NewTimer(time.Until(deadline))
			late = timer.C
		}
		select {
		case <-s.done:
			if timer != nil {
				timer.Stop()
			}
			goto finished
		case <-s.lifetime.changed:
			if timer != nil {
				timer.Stop()
			}
			continue
		case <-late:
			s.lifetime.mu.Lock()
			expired := !time.Now().Before(s.lifetime.deadline)
			s.lifetime.mu.Unlock()
			if !expired {
				continue
			}
			s.lifetime.finish("timeout")
			s.Close()
			return nil, ErrTimeout
		}
	}
finished:
	s.Close()
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.model, nil
}

func (s *Server) Close() {
	s.closeOnce.Do(func() {
		if s.lifetime != nil {
			s.lifetime.finish("closed")
		}
		s.cancel()
		if s.lifetime != nil {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_ = s.front.Shutdown(ctx)
		}
		s.front.Close()
		if s.logged != nil {
			log.SetOutput(s.logged)
		}
	})
}

// Discard removes what was dropped on the page. Drops are kept only when
// they are the answer to a pick; otherwise they were for looking at.
func (s *Server) Discard() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dir != "" {
		os.RemoveAll(s.dir)
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// A page elsewhere can point a name of its own at this machine.
	host, _, err := net.SplitHostPort(r.Host)
	allowed := err == nil && (host == "127.0.0.1" || host == "localhost")
	if s.webPick != nil {
		allowed = s.webPick.access.allows(r.Host)
	}
	if !allowed {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	token, rest, ok := strings.Cut(strings.TrimPrefix(r.URL.Path, "/"), "/")
	if !ok || subtle.ConstantTimeCompare([]byte(token), []byte(s.token)) != 1 {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Cache-Control", "no-store")
	if s.webPick != nil {
		s.serveWebPick(w, r, rest)
		return
	}
	switch {
	case rest == "session" || rest == "activity" || rest == "finish":
		s.viewerSession(w, r, rest)
	case rest == "":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		l := s.localizer(r)
		servePage.in(l).Execute(w, struct {
			pageText
			Prompt string
			Pick   bool
			Fetch  bool
			Accept document.AcceptFilter
		}{newPageText(l), s.prompt, s.pick, s.fetch, s.accept})
	case rest == "serve.mjs" || rest == "i18n.mjs" || rest == "viewer-session.mjs" || rest == "drop.mjs" || rest == "upload.mjs" || rest == "pickers.mjs" || rest == "remote.mjs":
		data, err := fs.ReadFile(web.Page, rest)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		kind := "text/javascript; charset=utf-8"
		w.Header().Set("Content-Type", kind)
		w.Write(data)
	case rest == "drop":
		s.receive(w, r)
	case rest == "chart":
		s.pushChart(w, r)
	case rest == "exports":
		s.listExports(w, r)
	case strings.HasPrefix(rest, "export/"):
		s.sendExport(w, r, strings.TrimPrefix(rest, "export/"))
	case rest == "ws" || strings.HasPrefix(rest, "static/"):
		r.URL.Path, r.URL.RawPath = "/"+rest, ""
		s.proxy.ServeHTTP(w, r)
	default:
		http.NotFound(w, r)
	}
}

// receive keeps the files of a drop and tells the viewer where. They are
// kept past the server's end: whoever started gloss has yet to read them.
func (s *Server) receive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST a drop", http.StatusMethodNotAllowed)
		return
	}
	if !samePickOrigin(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if s.uploads.Add(1) > 4 {
		s.uploads.Add(-1)
		r.Close = true
		w.Header().Set("Connection", "close")
		http.Error(w, "too many concurrent uploads", http.StatusServiceUnavailable)
		return
	}
	defer s.uploads.Add(-1)
	r.Body = http.MaxBytesReader(w, r.Body, maxSessionBytes+(1<<20))
	succeeded := false
	if s.webPick == nil {
		controller := http.NewResponseController(w)
		defer func() {
			if succeeded {
				_ = controller.SetReadDeadline(time.Time{})
			}
		}()
		r.Body = &uploadBody{ReadCloser: r.Body, controller: controller}
	}
	form, err := r.MultipartReader()
	if err != nil {
		http.Error(w, "not a drop", http.StatusBadRequest)
		return
	}
	var paths []string
	var taken int64 // What this drop has kept, to give back if it fails.
	fail := func(code int, reason string) {
		r.Close = true
		w.Header().Set("Connection", "close")
		for _, path := range paths {
			os.Remove(path)
		}
		s.stored.Add(-taken)
		http.Error(w, reason, code)
	}
	for parts := 0; ; parts++ {
		part, err := form.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			fail(uploadErrorStatus(err, http.StatusBadRequest), "could not read upload")
			return
		}
		if parts >= maxDrops {
			fail(http.StatusRequestEntityTooLarge, "too many multipart fields")
			return
		}
		// Only the last element of the name is the page's to choose.
		name := path.Base(strings.ReplaceAll(part.FileName(), `\`, "/"))
		if part.FileName() == "" || name == "." || name == ".." || name == "/" || !usableName(name) {
			continue
		}
		if len(paths) == maxDrops || (s.webPick != nil && len(s.webPick.files)+len(paths) >= maxWebPickFiles) {
			fail(http.StatusRequestEntityTooLarge, "too many files")
			return
		}
		if len(name) > 255 {
			fail(http.StatusBadRequest, "filename exceeds 255 bytes")
			return
		}
		// One file may be 128 MiB, and the session has only so much to give.
		charged := &uploadQuotaReader{s: s, reader: io.LimitReader(part, document.MaxFileBytes+1)}
		kept, size, err := s.keep(name, charged)
		if err != nil || size > document.MaxFileBytes {
			s.stored.Add(-charged.charged)
		}
		switch {
		case errors.Is(err, syscall.ENAMETOOLONG):
			fail(http.StatusBadRequest, "filename is too long (including any duplicate-name suffix)")
			return
		case err != nil:
			code := uploadErrorStatus(err, http.StatusInternalServerError)
			fail(code, "the drop could not be kept")
			return
		case size > document.MaxFileBytes:
			os.Remove(kept)
			fail(http.StatusRequestEntityTooLarge, document.ErrTooLarge.Error())
			return
		case size == 0:
			os.Remove(kept)
			continue
		}
		taken += size
		paths = append(paths, kept)
		if err := s.accept.CheckFile(nil, kept); err != nil {
			fail(http.StatusUnsupportedMediaType, err.Error())
			return
		}
	}
	if len(paths) == 0 {
		fail(http.StatusBadRequest, "nothing in the drop could be used")
		return
	}
	if s.webPick != nil {
		if err := s.webPick.add(w, paths); err != nil {
			fail(http.StatusInternalServerError, "could not register uploads")
		}
		return
	}
	select {
	case s.drops <- paths:
	default:
		fail(http.StatusServiceUnavailable, "the viewer is busy")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string][]string{"paths": paths})
	succeeded = true
}

// offer keeps an export for the page to download, within what the session
// may keep.
func (s *Server) offer(name string, data []byte) (string, error) {
	if s.stored.Add(int64(len(data))) > maxSessionBytes {
		s.stored.Add(-int64(len(data)))
		return "", errors.New("this session has kept all the files it will")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.exports = append(s.exports, export{name: name, data: data})
	close(s.exported)
	s.exported = make(chan struct{})
	return name + " (downloaded by the browser)", nil
}

// listExports answers the page with the exports from number after on, as
// soon as there are any or, with none, after a while.
func (s *Server) listExports(w http.ResponseWriter, r *http.Request) {
	after, err := strconv.Atoi(r.URL.Query().Get("after"))
	if err != nil || after < 0 {
		http.Error(w, "after what?", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	more := s.exported
	s.mu.Unlock()
	if s.exportsFrom(after) == nil {
		select {
		case <-more:
		case <-r.Context().Done():
			return
		case <-time.After(exportWait):
		}
	}
	type listed struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	list := []listed{}
	for i, e := range s.exportsFrom(after) {
		list = append(list, listed{after + i, e.name})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}

func (s *Server) exportsFrom(after int) []export {
	s.mu.Lock()
	defer s.mu.Unlock()
	if after >= len(s.exports) {
		return nil
	}
	return s.exports[after:]
}

// sendExport gives the page an export, to be saved where the browser saves.
func (s *Server) sendExport(w http.ResponseWriter, r *http.Request, id string) {
	i, err := strconv.Atoi(id)
	s.mu.Lock()
	ok := err == nil && i >= 0 && i < len(s.exports)
	var e export
	if ok {
		e = s.exports[i]
	}
	s.mu.Unlock()
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": e.name}))
	w.Write(e.data)
}

// keep writes a dropped file under its own name, or the nearest one free.
func (s *Server) keep(name string, from io.Reader) (string, int64, error) {
	dir, err := s.folder()
	if err != nil {
		return "", 0, err
	}
	// The name is claimed before a byte is read, so no lock is held while the
	// page, possibly slow, sends them.
	return document.WriteNewFrom(filepath.Join(dir, name), from, 0o600)
}

// useFolder keeps drops in dir, made by whoever started the session, instead
// of a folder of its own.
func (s *Server) useFolder(dir string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dir = dir
}

// folder is where drops are kept, made when the first arrives.
func (s *Server) folder() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dir == "" {
		dir, err := os.MkdirTemp("", "gloss-")
		if err != nil {
			return "", err
		}
		s.dir = dir
	}
	return s.dir, nil
}

// usableName says whether a dropped file's name is safe to keep it under, on
// any system: no control characters, and on Windows none of the characters
// that name a stream, a device, or a pattern, nor the device names themselves.
func usableName(name string) bool {
	if strings.ContainsFunc(name, func(r rune) bool { return r < ' ' || r == 0x7f }) {
		return false
	}
	if runtime.GOOS != "windows" {
		return true
	}
	if strings.ContainsAny(name, `:<>"|?*`) || strings.HasSuffix(name, ".") || strings.HasSuffix(name, " ") {
		return false
	}
	stem, _, _ := strings.Cut(name, ".")
	switch strings.ToUpper(strings.TrimSpace(stem)) {
	case "CON", "PRN", "AUX", "NUL", "COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9", "LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9":
		return false
	}
	return true
}

func ExitCode(err error) int {
	switch {
	case err == nil:
		return 0
	case errors.Is(err, ErrCancelled):
		return 2
	case errors.Is(err, ErrTimeout):
		return 124
	}
	return 1
}
