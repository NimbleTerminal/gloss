package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestPrintFlags(t *testing.T) {
	for _, args := range [][]string{
		{"--print-width", "80"}, {"--print", "--print-height", "0"},
		{"--print", "--print-width", "513"}, {"--print", "--print-height", "257"},
		{"--print", "--render", "kitty"}, {"--print", "--serve"},
		{"--print", "--output", "a.png"}, {"--print", "--text"},
		{"--print", "--info"}, {"--print", "--json"}, {"--print", "--menu"},
		{"--print", "--pick"}, {"--print", "--page", "all"},
	} {
		if _, _, err := parse(args, &bytes.Buffer{}); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	f, err := os.CreateTemp(t.TempDir(), "pipe")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if w, h := printSize(options{}, f); w != 80 || h != 24 {
		t.Fatalf("non-terminal default %dx%d", w, h)
	}
}

func TestPrintFiles(t *testing.T) {
	for _, name := range []string{"landscape.png", "landscape.jxl", "shapes.svg", "field-guide.pdf", "tetrahedron.stl", "sales.csv"} {
		t.Run(name, func(t *testing.T) {
			opts, _, err := parse([]string{"--print", "../../examples/" + name}, &bytes.Buffer{})
			if err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			if err := printFiles(opts, &out, 40, 12); err != nil {
				t.Fatal(err)
			}
			plain := ansi.Strip(out.String())
			if strings.TrimSpace(plain) == "" || strings.Count(plain, "\n") > 12 {
				t.Fatalf("empty or tall output: %q", plain)
			}
			for _, line := range strings.Split(plain, "\n") {
				if ansi.StringWidth(line) > 40 {
					t.Fatalf("wide line: %q", line)
				}
			}
		})
	}
}

// Exercise run with actual pipes: no TTY, probing, UI loop or stderr output.
func TestPrintCapturedShell(t *testing.T) {
	if os.Getenv("GLOSS_TEST_PRINT_CHILD") == "1" {
		args := strings.Split(os.Getenv("GLOSS_TEST_PRINT_ARGS"), "\n")
		if err := run(args); err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		}
		os.Exit(0)
	}
	for _, args := range [][]string{
		{"--print", "--print-width", "32", "--print-height", "10", "../../examples/landscape.png"},
		{"--print", "-"},
	} {
		cmd := exec.Command(os.Args[0], "-test.run=^TestPrintCapturedShell$")
		cmd.Env = append(os.Environ(), "GLOSS_TEST_PRINT_CHILD=1", "GLOSS_TEST_PRINT_ARGS="+strings.Join(args, "\n"))
		cmd.Stdin = strings.NewReader("hello from a pipe\n")
		var out, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &stderr
		if err := cmd.Run(); err != nil || stderr.Len() != 0 {
			t.Fatalf("%v: %v stderr=%s", args, err, &stderr)
		}
		if out.Len() == 0 || strings.Contains(out.String(), "q quit") || strings.Contains(out.String(), "\x1b[?1049") {
			t.Fatalf("not static content: %q", out.String())
		}
	}
}

func TestPrintTextAndColumns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.txt")
	os.WriteFile(path, []byte("hello\nsecond\nthird\n"), 0600)
	opts, _, err := parse([]string{"--print", "--line-numbers", path}, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := printFiles(opts, &out, 40, 12); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(strings.Fields(ansi.Strip(out.String())), " "), "1 hello") {
		t.Fatalf("missing numbered text: %q", ansi.Strip(out.String()))
	}
	opts, _, err = parse([]string{"--print", "--cols", "region,revenue", "../../examples/sales.csv"}, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := printFiles(opts, &out, 80, 12); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "quarter") || !strings.Contains(out.String(), "revenue") {
		t.Fatalf("column filter ignored: %s", &out)
	}
	if err := printFiles(opts, brokenPrintWriter{}, 80, 12); err == nil {
		t.Fatal("writer error lost")
	}
}

func TestPrintTypeset(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.md")
	if err := os.WriteFile(path, []byte("# A typeset page\n\nHello world."), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"--print", "--typeset", path},
	} {
		opts, _, err := parse(args, &bytes.Buffer{})
		if err != nil {
			t.Fatal(err)
		}
		for _, width := range []int{80, 140} {
			var out bytes.Buffer
			if err := printFiles(opts, &out, width, 24); err != nil {
				t.Fatalf("%v: %v", args, err)
			}
			if out.Len() == 0 {
				t.Fatalf("empty: %v", args)
			}
		}
	}
}

type brokenPrintWriter struct{}

func (brokenPrintWriter) Write([]byte) (int, error) { return 0, errors.New("closed pipe") }
