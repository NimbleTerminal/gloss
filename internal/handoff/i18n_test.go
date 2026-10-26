package handoff

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nimbleterminal.dev/gloss/internal/app"
	"nimbleterminal.dev/gloss/internal/i18n"
)

// page returns the page a GET of s's root answers, with the header given.
func page(t *testing.T, s *Server, acceptLanguage string) string {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:1234/"+s.token+"/", nil)
	if s.webPick != nil {
		r = httptest.NewRequest(http.MethodGet, s.URL, nil)
	}
	if acceptLanguage != "" {
		r.Header.Set("Accept-Language", acceptLanguage)
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("Accept-Language %.40q: %d", acceptLanguage, w.Code)
	}
	return w.Body.String()
}

// pages are a --serve page and a --pick-web page, with the label that shows
// each is in Spanish.
func pages(t *testing.T, lang string) map[string]*Server {
	return map[string]*Server{
		"serve":    {token: "test-token", lang: lang},
		"pick-web": webPicking(t, app.Options{Lang: lang}),
	}
}

var spanish = map[string]string{
	"serve":    i18n.For("es").T("web.serve.done", "Done viewing"),
	"pick-web": i18n.For("es").T("web.pick.send", "Send files"),
}

func TestPageLanguageFromHeader(t *testing.T) {
	for name, s := range pages(t, "") {
		for header, want := range map[string]string{
			"es-MX,es;q=0.9":            "es",
			"zh-CN;q=0.1, es;q=0.9":     "es",
			"*":                         "en",
			"xx;q=abc":                  "en",
			"":                          "en",
			strings.Repeat("a,", 5<<10): "en",
		} {
			body := page(t, s, header)
			if !strings.Contains(body, `<html lang="`+want+`">`) {
				t.Errorf("%s, %.40q: want lang=%q", name, header, want)
			}
			if len(header) > 3 && strings.Contains(body, header) {
				t.Errorf("%s: the raw header %.40q is in the page", name, header)
			}
			if want == "es" && !strings.Contains(body, spanish[name]) {
				t.Errorf("%s, %q: no %q", name, header, spanish[name])
			}
		}
	}
	if spanish["serve"] == "Done viewing" || spanish["pick-web"] == "Send files" {
		t.Fatal("the Spanish catalog lacks the pages' labels")
	}
}

func TestPageLanguageFromConfig(t *testing.T) {
	for name, s := range pages(t, "zh-CN") {
		if body := page(t, s, "es"); !strings.Contains(body, `<html lang="zh-Hans">`) {
			t.Errorf("%s: --lang zh-CN gave no zh-Hans page", name)
		}
	}
}

func TestEnglishPageHasNoMessages(t *testing.T) {
	for name, s := range pages(t, "") {
		if body := page(t, s, "en-US"); !strings.Contains(body, `<script type="application/json" id="gloss-messages">{}</script>`) {
			t.Errorf("%s: English page carries messages", name)
		}
	}
}

func TestMessagesAreEscaped(t *testing.T) {
	was := webMessages
	t.Cleanup(func() { webMessages = was })
	webMessages = func(*i18n.Localizer) map[string]string {
		return map[string]string{"web.serve.done": `</script><script>alert(1)</script>`}
	}
	for name, s := range pages(t, "es") {
		body := page(t, s, "")
		start := strings.Index(body, `id="gloss-messages">`)
		if start < 0 {
			t.Fatalf("%s: no messages element", name)
		}
		if rest := body[start:]; strings.Index(rest, "</script>") < strings.Index(rest, `alert(1)`) {
			t.Errorf("%s: a message ended the script element: %s", name, rest)
		}
	}
}

func TestViewerFollowsThePage(t *testing.T) {
	t.Cleanup(func() { i18n.SetLanguage("en") })
	s := &Server{token: "test-token"}
	page(t, s, "es-MX")
	opts := s.viewerOptions(app.Options{})
	if opts.Lang != "es-MX" || i18n.Language() != "es" {
		t.Errorf("viewer Lang %q, language %q; want es-MX, es", opts.Lang, i18n.Language())
	}
	// A second browser cannot switch the running viewer's language.
	page(t, s, "zh-CN")
	if opts := s.viewerOptions(app.Options{}); opts.Lang != "es-MX" || i18n.Language() != "es" {
		t.Errorf("second viewer Lang %q, language %q; want es-MX, es", opts.Lang, i18n.Language())
	}
	s = &Server{token: "test-token", lang: "zh-CN"}
	page(t, s, "es")
	if opts := s.viewerOptions(app.Options{Lang: "zh-CN"}); opts.Lang != "zh-CN" {
		t.Errorf("--lang zh-CN viewer Lang %q", opts.Lang)
	}
}
