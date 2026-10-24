package chart

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

func TestPageIsSelfContained(t *testing.T) {
	page, csp := Page("Sales", json.RawMessage(`{"series":[{"type":"bar","data":[1]}]}`))
	// The bundle is checked on its own: it must not end its own element.
	if bytes.Contains(echartsScript, []byte("</script")) {
		t.Fatal("the ECharts bundle ends a script element")
	}
	rest := strings.Replace(string(page), string(echartsScript), "", 1)
	for _, bad := range []string{"<script src", "<link ", "url(", "@import", "http://", "https://"} {
		if strings.Contains(rest, bad) {
			t.Errorf("page fetches: %q", bad)
		}
	}
	for _, good := range []string{"<title>Sales</title>", `"series"`, "Content-Security-Policy", "echarts.init", "richText", csp} {
		if !strings.Contains(rest, good) {
			t.Errorf("page lacks %q", good)
		}
	}
	if len(page) < 500_000 || len(page) > 2<<20 {
		t.Errorf("page is %d bytes", len(page))
	}
}

// Only the page's own two scripts may run: the policy names them by hash,
// so nothing an option smuggles in (an inline handler, a javascript: URL)
// can.
func TestPageScriptsAreHashed(t *testing.T) {
	page, csp := Page("Sales", json.RawMessage(`{"series":[]}`))
	if strings.Contains(csp, "'unsafe-inline'") && strings.Index(csp, "script-src") < strings.Index(csp, "'unsafe-inline'") && strings.Index(csp, "'unsafe-inline'") < strings.Index(csp, "style-src") {
		t.Fatalf("script-src allows any inline script: %s", csp)
	}
	scripts := regexp.MustCompile(`(?s)<script>(.*?)</script>`).FindAllSubmatch(page, -1)
	if len(scripts) != 2 {
		t.Fatalf("%d scripts", len(scripts))
	}
	for _, s := range scripts {
		sum := sha256.Sum256(s[1])
		if want := "'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"; !strings.Contains(csp, want) {
			t.Errorf("policy lacks the hash of a %d-byte script", len(s[1]))
		}
	}
}

func TestPageKeepsScriptEndInert(t *testing.T) {
	separator := string(rune(0x2028)) // Spelled by code point: an editor may turn an escape into the rune.
	page, _ := Page("<b>x</b></script><script>alert(1)</script>", json.RawMessage(`{"title":{"text":"</script><script>alert(2)</script>`+separator+`"}}`))
	if n := bytes.Count(page, []byte("</script>")); n != 2 {
		t.Fatalf("%d script ends", n)
	}
	for _, raw := range []string{"<script>alert(1)", "<b>x</b>", separator} {
		if bytes.Contains(page, []byte(raw)) {
			t.Errorf("%q reached the page raw", raw)
		}
	}
	if !bytes.Contains(page, []byte(`<\/script>`)) {
		t.Fatal("option's script end was not escaped")
	}
}

func TestPageSurvivesBadJSON(t *testing.T) {
	page, _ := Page("x", json.RawMessage(`{"series":`))
	if !strings.Contains(string(page), "var option = {};") {
		t.Fatal("invalid JSON reached the page")
	}
}
