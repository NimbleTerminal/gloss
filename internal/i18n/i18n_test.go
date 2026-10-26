package i18n

import (
	"strings"
	"sync"
	"testing"
)

func TestTFallsBackToDefault(t *testing.T) {
	SetLanguage("es")
	t.Cleanup(func() { SetLanguage("en") })
	if got := T("test.missing", "Hello"); got != "Hello" {
		t.Fatal(got)
	}
	if got := Tf("test.missing", "Hello %s", "you"); got != "Hello you" {
		t.Fatal(got)
	}
}

func TestTnEnglishTemplate(t *testing.T) {
	SetLanguage("en")
	for n, want := range map[int]string{1: "1 file", 3: "3 files", 0: "0 files"} {
		if got := Tn("test.files", "{{.Count}} file", "{{.Count}} files", n); got != want {
			t.Errorf("Tn(%d) = %q, want %q", n, got, want)
		}
	}
	// A foreign language without the ID also falls back to the English template.
	SetLanguage("es")
	t.Cleanup(func() { SetLanguage("en") })
	if got := Tn("test.files", "{{.Count}} file", "{{.Count}} files", 3); got != "3 files" {
		t.Errorf("es Tn = %q", got)
	}
}

func TestTnWithoutLocalizer(t *testing.T) {
	var l *Localizer
	if got := l.Tn("test.files", "{{.Count}} file", "{{.Count}} files", 1); got != "1 file" {
		t.Errorf("nil Tn = %q", got)
	}
	if got := l.T("x", "def"); got != "def" {
		t.Errorf("nil T = %q", got)
	}
}

func TestTranslated(t *testing.T) {
	const def = "seed"
	if got := For("es").T("test.seed", def); got != "semilla" {
		t.Errorf("es = %q", got)
	}
	if got := For("zh-Hans").T("test.seed", def); got != "种子" {
		t.Errorf("zh-Hans = %q", got)
	}
	if got := For("en").T("test.seed", def); got != def {
		t.Errorf("en = %q", got)
	}
	if got := For("zh-CN").T("test.seed", def); got != "种子" {
		t.Errorf("zh-CN = %q", got)
	}
	if got := For("fr").Tag(); got != "en" {
		t.Errorf("fr tag = %q", got)
	}
	m := For("es").Messages("test.")
	if len(m) == 0 {
		t.Fatal("no test. messages")
	}
	for k := range m {
		if !strings.HasPrefix(k, "test.") {
			t.Errorf("key %q lacks prefix", k)
		}
	}
	if m["test.seed"] != "semilla" {
		t.Errorf("Messages seed = %q", m["test.seed"])
	}
	if got := For("es").Messages("web."); len(got) != 0 {
		t.Errorf("web. = %v", got)
	}
	if got := For("en").Messages("test."); len(got) != 0 {
		t.Errorf("en Messages = %v", got)
	}
}

func TestSetLanguage(t *testing.T) {
	t.Cleanup(func() { SetLanguage("en") })
	for in, want := range map[string]string{"": "en", "es-MX": "es", "zh-CN": "zh-Hans", "fr": "en"} {
		SetLanguage(in)
		if got := Language(); got != want {
			t.Errorf("SetLanguage(%q): Language() = %q, want %q", in, got, want)
		}
	}
	SetLanguage("zh-Hans")
	if got := T("test.seed", "seed"); got != "种子" {
		t.Errorf("T = %q", got)
	}
}

func TestLanguages(t *testing.T) {
	var tags []string
	for _, l := range Languages() {
		tags = append(tags, l.Tag)
		if l.Name == "" || l.English == "" {
			t.Errorf("incomplete %+v", l)
		}
	}
	if got := strings.Join(tags, ","); got != "en,es,zh-Hans" {
		t.Errorf("tags = %s", got)
	}
	if Languages()[1].Name != "Español" || Languages()[2].Name != "简体中文" {
		t.Errorf("names = %+v", Languages())
	}
}

func TestConcurrent(t *testing.T) {
	t.Cleanup(func() { SetLanguage("en") })
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 200 {
				if i%2 == 0 {
					SetLanguage("es")
					SetLanguage("zh-Hans")
				}
				_ = T("test.seed", "seed")
				_ = Tn("test.n", "{{.Count}} a", "{{.Count}} b", i)
			}
		}()
	}
	wg.Wait()
}
