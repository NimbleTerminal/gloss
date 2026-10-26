package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
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
	spanish := func(name string) string {
		if name == "LANG" {
			return "es_ES"
		}
		return ""
	}
	for _, tt := range []struct {
		name string
		args []string
		tty  bool
		want string
	}{
		{"info ignores LANG", []string{"--info", "x.png"}, true, ""},
		{"info takes --lang", []string{"--info", "--lang", "es", "x.png"}, true, "es"},
		{"viewer reads LANG", []string{"x.png"}, true, "es-ES"},
		{"serve without a terminal is detached", []string{"--serve", "x.png"}, false, ""},
		{"serve from a terminal reads LANG", []string{"--serve", "x.png"}, true, "es-ES"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			opts, _, err := parse(tt.args, &bytes.Buffer{})
			if err != nil {
				t.Fatal(err)
			}
			if got := language(opts, tt.tty, spanish); got != tt.want {
				t.Fatalf("language = %q, want %q", got, tt.want)
			}
		})
	}
	var opts options
	opts.Serve, opts.Detached = true, "token"
	if got := language(opts, true, spanish); got != "" {
		t.Fatalf("detached server: %q", got)
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
	runs := func(locale []string, dir string) []string {
		var outs []string
		for _, args := range [][]string{
			{"--info", "--json", "../../examples/sales.csv", "../../examples/landscape.png"},
			{"--text", "--json", "../../examples/readme.md"},
			{"--output-dir", dir, "--json", "../../examples/landscape.png"},
		} {
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
			outs = append(outs, strings.ReplaceAll(out.String(), dir, "DIR"))
		}
		return outs
	}
	spanish := runs([]string{"LANG=es_ES.UTF-8", "LC_ALL="}, filepath.Join(t.TempDir(), "es"))
	plain := runs([]string{"LANG=C"}, filepath.Join(t.TempDir(), "c"))
	for i := range spanish {
		if spanish[i] != plain[i] {
			t.Errorf("run %d differs:\nes_ES: %s\nC:     %s", i, spanish[i], plain[i])
		}
	}
	if !regexp.MustCompile(`"DIR/`).MatchString(spanish[2]) {
		t.Errorf("export manifest names no file in its folder: %s", spanish[2])
	}
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
