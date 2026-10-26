package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"

	"nimbleterminal.dev/gloss/internal/theme"
)

func TestLangValidation(t *testing.T) {
	_, _, err := parse([]string{"--lang", "garbage!!", "x.png"}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "not a language tag") {
		t.Fatalf("garbage: %v", err)
	}
	opts, _, err := parse([]string{"--lang", "zh_CN.UTF-8", "x.png"}, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if opts.Lang != "zh-CN" {
		t.Fatalf("Lang = %q, want zh-CN", opts.Lang)
	}
}

func TestLanguageResolution(t *testing.T) {
	env := func(vars ...string) func(string) string {
		return func(name string) string {
			for i := 0; i+1 < len(vars); i += 2 {
				if vars[i] == name {
					return vars[i+1]
				}
			}
			return ""
		}
	}
	spanish := env("LANG", "es_ES")
	saved := func(tag string) func() string {
		return func() string { return tag }
	}
	for _, tt := range []struct {
		name   string
		args   []string
		tty    bool
		getenv func(string) string
		saved  func() string
		want   string
	}{
		{"info ignores LANG", []string{"--info", "x.png"}, true, spanish, nil, ""},
		{"info takes --lang", []string{"--info", "--lang", "es", "x.png"}, true, spanish, nil, "es"},
		{"viewer reads LANG", []string{"x.png"}, true, spanish, nil, "es-ES"},
		{"serve without a terminal is detached", []string{"--serve", "x.png"}, false, spanish, nil, ""},
		{"serve from a terminal reads LANG", []string{"--serve", "x.png"}, true, spanish, nil, "es-ES"},
		{"saved beats LANG", []string{"x.png"}, true, spanish, saved("zh-Hans"), "zh-Hans"},
		{"saved beats LC_ALL", []string{"x.png"}, true, env("LC_ALL", "es_ES"), saved("zh-Hans"), "zh-Hans"},
		{"GLOSS_LANG beats saved", []string{"x.png"}, true, env("GLOSS_LANG", "es", "LANG", "en_US"), saved("zh-Hans"), "es"},
		{"--lang beats saved", []string{"--lang", "es", "x.png"}, true, spanish, saved("zh-Hans"), "es"},
		{"no saved choice is LANG", []string{"x.png"}, true, spanish, saved(""), "es-ES"},
		{"pick in a terminal reads saved", []string{"--pick", "x.png"}, true, spanish, saved("zh-Hans"), "zh-Hans"},
		{"headless ignores saved", []string{"--info", "x.png"}, true, spanish, saved("zh-Hans"), ""},
		{"export ignores saved", []string{"--output", "-", "x.png"}, true, spanish, saved("zh-Hans"), ""},
		{"print ignores saved", []string{"--print", "x.png"}, true, spanish, saved("zh-Hans"), ""},
		{"detached start ignores saved", []string{"--pick", "x.png"}, false, spanish, saved("zh-Hans"), ""},
		{"serve page ignores saved", []string{"--serve", "x.png"}, true, spanish, saved("zh-Hans"), "es-ES"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			opts, _, err := parse(tt.args, &bytes.Buffer{})
			if err != nil {
				t.Fatal(err)
			}
			if got := language(opts, tt.tty, tt.getenv, tt.saved); got != tt.want {
				t.Fatalf("language = %q, want %q", got, tt.want)
			}
		})
	}
	var opts options
	opts.Serve, opts.Detached = true, "token"
	if got := language(opts, true, spanish, saved("zh-Hans")); got != "" {
		t.Fatalf("detached server: %q", got)
	}
	// The saved file is not even read where it does not count.
	read := func() string { t.Fatal("saved choice read"); return "" }
	for _, args := range [][]string{{"--info", "x.png"}, {"--text", "x.md"}, {"--serve", "x.png"}, {"--pick-web"}} {
		opts, _, err := parse(args, &bytes.Buffer{})
		if err != nil {
			t.Fatal(err)
		}
		language(opts, true, spanish, read)
	}
	language(opts, true, spanish, read)
}

func TestLangPreferences(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "gloss")
	if err := writeLangChoice(dir, "zh-Hans"); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(dir); err != nil || info.Mode().Perm() != 0700 {
		t.Fatalf("folder: %v %v", info, err)
	}
	if got := readLangChoice(dir); got != "zh-Hans" {
		t.Fatalf("read %q", got)
	}
	if err := writeLangChoice(dir, "es"); err != nil {
		t.Fatal(err)
	}
	if got := readLangChoice(dir); got != "es" {
		t.Fatalf("read %q", got)
	}
	// System removes the file, and removing nothing is no error.
	for range 2 {
		if err := writeLangChoice(dir, ""); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(dir, "lang")); !os.IsNotExist(err) {
			t.Fatalf("System left the file: %v", err)
		}
	}
	if got := readLangChoice(dir); got != "" {
		t.Fatalf("no file read %q", got)
	}
	if err := writeLangChoice(dir, "garbage!!"); err == nil {
		t.Fatal("garbage tag saved")
	}
	// A corrupt file is no choice.
	for _, bad := range []string{"", "{", `{"lang":"garbage!!"}`, `{"lang":"es","x":1}`, `{"lang":"es"}{}`, strings.Repeat(" ", 5000) + `{"lang":"es"}`} {
		if err := os.WriteFile(filepath.Join(dir, "lang"), []byte(bad), 0600); err != nil {
			t.Fatal(err)
		}
		if got := readLangChoice(dir); got != "" {
			t.Fatalf("%q read as %q", bad, got)
		}
	}
}

// Agent output is the same whatever the machine's locale: real runs of
// gloss, through pipes, under a Spanish LANG and under C.
func TestHeadlessIgnoresLANG(t *testing.T) {
	if os.Getenv("GLOSS_TEST_LANG_CHILD") == "1" {
		args := strings.Split(os.Getenv("GLOSS_TEST_LANG_ARGS"), "\n")
		if err := run(args); err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		}
		os.Exit(0)
	}
	// child runs gloss with args under the locale variables in locale.
	child := func(locale, args []string) string {
		t.Helper()
		cmd := exec.Command(os.Args[0], "-test.run=^TestHeadlessIgnoresLANG$")
		cmd.Env = append(withoutLocale(os.Environ()), "GLOSS_TEST_LANG_CHILD=1", "GLOSS_TEST_LANG_ARGS="+strings.Join(args, "\n"))
		cmd.Env = append(cmd.Env, locale...)
		var out, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("%v %v: %v stderr=%s", locale, args, err, &stderr)
		}
		if out.Len() == 0 {
			t.Fatalf("%v %v: no output", locale, args)
		}
		return out.String()
	}
	info := []string{"--info", "--json", "../../examples/sales.csv", "../../examples/landscape.png"}
	runs := func(locale []string, dir string) []string {
		var outs []string
		for _, args := range [][]string{
			info,
			{"--text", "--json", "../../examples/readme.md"},
			{"--output-dir", dir, "--json", "../../examples/landscape.png"},
		} {
			outs = append(outs, strings.ReplaceAll(child(locale, args), dir, "DIR"))
		}
		return outs
	}
	// A language saved in the viewer is no more read than LANG: the Spanish
	// runs also have Chinese saved in their configuration folder.
	home := t.TempDir()
	saved := []string{"HOME=" + home, "XDG_CONFIG_HOME=" + filepath.Join(home, ".config"), "AppData=" + filepath.Join(home, "AppData")}
	for _, kv := range saved {
		name, value, _ := strings.Cut(kv, "=")
		t.Setenv(name, value)
	}
	dir, err := theme.Dir()
	if err != nil {
		t.Fatal(err)
	}
	if err := writeLangChoice(dir, "zh-Hans"); err != nil {
		t.Fatal(err)
	}
	if readLangChoice(dir) != "zh-Hans" {
		t.Fatal("saved choice not written")
	}
	spanish := runs(append([]string{"LANG=es_ES.UTF-8", "LC_ALL="}, saved...), filepath.Join(t.TempDir(), "es"))
	plain := runs([]string{"LANG=C"}, filepath.Join(t.TempDir(), "c"))
	for i := range spanish {
		if spanish[i] != plain[i] {
			t.Errorf("run %d differs:\nes_ES: %s\nC:     %s", i, spanish[i], plain[i])
		}
	}
	if !regexp.MustCompile(`"DIR/`).MatchString(spanish[2]) {
		t.Errorf("export manifest names no file in its folder: %s", spanish[2])
	}

	// An explicit --lang changes values, never keys.
	explicit := child([]string{"LANG=C"}, append([]string{"--lang", "es-ES"}, info...))
	var en, es any
	if err := json.Unmarshal([]byte(plain[0]), &en); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(explicit), &es); err != nil {
		t.Fatal(err)
	}
	if got, want := jsonKeys(es, ""), jsonKeys(en, ""); !slices.Equal(got, want) {
		t.Errorf("--lang es-ES keys differ:\nes: %v\nen: %v", got, want)
	}
	size := func(v any) string {
		return v.([]any)[1].(map[string]any)["details"].(map[string]any)["File"].(map[string]any)["Size"].(string)
	}
	if size(es) == size(en) {
		t.Errorf("--lang es-ES did not change the Size value: %q", size(es))
	}
}

// jsonKeys returns the paths of every object key in v, sorted.
func jsonKeys(v any, prefix string) []string {
	var keys []string
	switch v := v.(type) {
	case map[string]any:
		for k, e := range v {
			keys = append(keys, prefix+"."+k)
			keys = append(keys, jsonKeys(e, prefix+"."+k)...)
		}
	case []any:
		for i, e := range v {
			keys = append(keys, jsonKeys(e, prefix+"["+strconv.Itoa(i)+"]")...)
		}
	}
	sort.Strings(keys)
	return keys
}

// withoutLocale drops the variables a language is read from.
func withoutLocale(env []string) []string {
	var kept []string
	for _, kv := range env {
		name, _, _ := strings.Cut(kv, "=")
		if name == "GLOSS_LANG" || name == "LANG" || strings.HasPrefix(name, "LC_") {
			continue
		}
		kept = append(kept, kv)
	}
	return kept
}
