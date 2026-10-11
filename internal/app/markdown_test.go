package app

import (
	"image"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/NimbleTerminal/gloss/internal/document"
	"github.com/charmbracelet/x/ansi"
)

func TestMarkdownLayoutAndScroll(t *testing.T) {
	doc := &document.Markdown{Source: []byte("# Heading\n\nSome **bold** text.\n\n![picture](local.png)\n\n" + strings.Repeat("another paragraph\n\n", 30)), Images: []document.MarkdownImage{{Alt: "picture", Image: image.NewRGBA(image.Rect(0, 0, 40, 20))}}}
	m := newMarkdownView(doc, 500)
	m.layout(60, 12, 8, 16)
	var all strings.Builder
	rows := 0
	for _, line := range m.lines {
		all.WriteString(line.text)
		if line.image >= 0 {
			rows++
		}
	}
	plain := ansi.Strip(all.String())
	if !strings.Contains(plain, "Heading") || !strings.Contains(plain, "[image: picture]") || rows == 0 {
		t.Fatalf("missing rendered content: %s; image rows %d", plain, rows)
	}
	if !m.key("end") || m.offset != len(m.lines)-m.height {
		t.Fatal("end did not scroll to bottom")
	}
	m.key("home")
	if m.offset != 0 {
		t.Fatal("home did not reset")
	}
	m.raw = true
	m.layout(60, 12, 8, 16)
	if !strings.Contains(m.view(), "![picture](local.png)") {
		t.Fatal("source mode lost Markdown syntax")
	}
	for _, line := range m.lines {
		if line.image >= 0 {
			t.Fatal("source mode contains rendered images")
		}
	}
}

// In the source view every unit is a file line: with numbers on, a line's
// first row carries its number in a gutter and its wrapped rows a blank one,
// and the text is wrapped to the width left of the gutter.
func TestLineNumbersNumberSourceLines(t *testing.T) {
	doc := &document.Markdown{Source: []byte("first\nsecond line that is long enough to wrap twice over\nthird\n")}
	m := newMarkdownView(doc, 500)
	m.raw, m.numbers = true, true
	m.layout(24, 2, 8, 16)
	if !m.numbered() {
		t.Fatal("source view has no line numbers")
	}
	rows := strings.Split(ansi.Strip(m.view()), "\n")
	m.layout(24, 10, 8, 16)
	rows = strings.Split(ansi.Strip(m.view()), "\n")
	want := []string{"1 first", "2 second line that is lo", "  ng enough to wrap twic", "  e over", "3 third", "4 "}
	if len(rows) != len(want) {
		t.Fatalf("rows %q", rows)
	}
	for i, row := range rows {
		if strings.TrimRight(row, " ") != strings.TrimRight(want[i], " ") {
			t.Fatalf("row %d %q, want %q", i, row, want[i])
		}
	}
	for i, line := range []int{1, 2, 2, 2, 3, 4} {
		if m.lines[i].line != line {
			t.Fatalf("row %d is line %d, want %d", i, m.lines[i].line, line)
		}
	}
	// Going to a line puts its first row at the top, within the scroll range.
	m.layout(24, 2, 8, 16)
	if !m.goLine(3) || m.offset != 4 {
		t.Fatalf("go to line 3 gave offset %d", m.offset)
	}
	if !m.goLine(99) || m.offset != 4 {
		t.Fatalf("go past the end gave offset %d", m.offset)
	}
	if !m.goLine(0) || m.offset != 0 {
		t.Fatalf("go to line 0 gave offset %d", m.offset)
	}
	// Numbers off: no gutter, full width.
	m.numbers = false
	m.layout(24, 10, 8, 16)
	if rows := strings.Split(ansi.Strip(m.view()), "\n"); strings.TrimRight(rows[0], " ") != "first" || len(rows) != 6 {
		t.Fatalf("rows without numbers %q", rows)
	}
}

// Plain text, source and JSON are shown as one fenced code block: with
// numbers on, the rows of the block carry the file's line numbers, and
// Glamour's margin rows around it none. JSON records and Markdown prose
// have no file lines to number.
func TestLineNumbersNumberFencedFileLines(t *testing.T) {
	files := fstest.MapFS{
		"main.go":      {Data: []byte("package main\n\n\tfunc main() {}\n// end\n")},
		"data.json":    {Data: []byte("{\"value\": 42, \"list\": [1, 2]}")},
		"records.json": {Data: []byte("{\"a\": 1}\n{\"a\": 2}\n")},
		"notes.md":     {Data: []byte("# Title\n\nSome *prose* here.\n")},
	}
	for _, path := range []string{"main.go", "data.json", "records.json", "notes.md"} {
		t.Run(path, func(t *testing.T) {
			m := New(Options{Files: []string{path}, FilesFS: files, Render: "glyph", Page: 1})
			defer m.Close()
			m.width, m.height = 80, 40
			drainAnimation(t, m, m.load(false))
			m.lineNumbers = true
			drainAnimation(t, m, m.layoutMarkdown())
			v := m.markdown
			if path == "records.json" || path == "notes.md" {
				if v.numbered() {
					t.Fatalf("%s numbered its rows", path)
				}
				return
			}
			if !v.numbered() {
				t.Fatal("rows not numbered")
			}
			var want []string
			switch path {
			case "main.go":
				want = []string{"package main", "", "func main() {}", "// end"}
			case "data.json":
				want = []string{"{", "\"value\": 42,", "\"list\": [", "1,", "2", "]", "}"}
			}
			got := map[int]string{}
			for i, row := range strings.Split(ansi.Strip(v.view()), "\n") {
				line := v.lines[i].line
				if line == 0 || (i > 0 && v.lines[i-1].line == line) {
					if strings.TrimSpace(row[:v.gutter]) != "" {
						t.Fatalf("row %d %q has a number it should not", i, row)
					}
					continue
				}
				n, err := strconv.Atoi(strings.TrimSpace(row[:v.gutter]))
				if err != nil || n != line {
					t.Fatalf("row %d %q: gutter does not say line %d", i, row, line)
				}
				got[n] = strings.TrimSpace(row[v.gutter:])
			}
			if len(got) != len(want) {
				t.Fatalf("numbered %d lines, want %d: %v", len(got), len(want), got)
			}
			for i, text := range want {
				if got[i+1] != text {
					t.Fatalf("line %d is %q, want %q", i+1, got[i+1], text)
				}
			}
		})
	}
}
