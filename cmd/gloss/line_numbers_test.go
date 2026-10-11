package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --line-numbers reaches the typeset export: a numbered page is a different
// picture from the plain one.
func TestLineNumbersExportCLI(t *testing.T) {
	path := filepath.Join(t.TempDir(), "main.go")
	if err := os.WriteFile(path, []byte("package main\n\nfunc main() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	render := func(args ...string) []byte {
		t.Helper()
		var out, errs bytes.Buffer
		opts, _, err := parse(append(args, "--typeset", "--output", "-", "--max-edge", "256", path), &out)
		if err != nil {
			t.Fatal(err)
		}
		if err := exportFiles(opts, &out, &errs); err != nil {
			t.Fatal(err, errs.String())
		}
		return out.Bytes()
	}
	plain, numbered := render(), render("--line-numbers")
	if bytes.Equal(plain, numbered) {
		t.Fatal("--line-numbers did not change the export")
	}
	opts, _, err := parse([]string{"--line-numbers", path}, &bytes.Buffer{})
	if err != nil || !opts.LineNumbers || !exportRequest(opts.Options, path, 1).TypesetLineNumbers {
		t.Fatalf("flag not carried: %v %+v", err, opts.LineNumbers)
	}
	// -N is its short form, as in less.
	if opts, _, err := parse([]string{"-N", path}, &bytes.Buffer{}); err != nil || !opts.LineNumbers {
		t.Fatalf("-N: %v %+v", err, opts.LineNumbers)
	}
}

// --chop-long-lines, -S as in less, reaches the viewer's options and the
// typeset export, where a chopped page is a different picture.
func TestChopLongLinesCLI(t *testing.T) {
	path := filepath.Join(t.TempDir(), "long.txt")
	line := []byte("short\n" + strings.Repeat("a long line of words that will wrap at the measure ", 8) + "\n")
	if err := os.WriteFile(path, line, 0600); err != nil {
		t.Fatal(err)
	}
	render := func(args ...string) []byte {
		t.Helper()
		var out, errs bytes.Buffer
		opts, _, err := parse(append(args, "--typeset", "--output", "-", "--max-edge", "256", path), &out)
		if err != nil {
			t.Fatal(err)
		}
		if err := exportFiles(opts, &out, &errs); err != nil {
			t.Fatal(err, errs.String())
		}
		return out.Bytes()
	}
	if bytes.Equal(render(), render("-S")) {
		t.Fatal("-S did not change the export")
	}
	for _, flag := range []string{"--chop-long-lines", "-S"} {
		opts, _, err := parse([]string{flag, path}, &bytes.Buffer{})
		if err != nil || !opts.ChopLines || !exportRequest(opts.Options, path, 1).TypesetChop {
			t.Fatalf("%s not carried: %v %+v", flag, err, opts.ChopLines)
		}
	}
}
