package app

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"nimbleterminal.dev/gloss/internal/document"
)

func BenchmarkPipedPatchLayout(b *testing.B) {
	data := []byte(strings.Repeat("diff --git a/file.go b/file.go\nindex 1234567..abcdef0 100644\n--- a/file.go\n+++ b/file.go\n@@ -1,2 +1,2 @@\n-old line\n+new line\n unchanged line\n", 12000))
	doc, err := document.ReadText("stdin", data)
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		view := newMarkdownView(&document.Markdown{Source: doc.Markdown}, nil)
		view.fenced = true
		view.layout(100, 25, 8, 16)
	}
}

func TestPipedPatchColorsAndFileLines(t *testing.T) {
	const source = "commit 0123456789abcdef\nAuthor: Example\n\n    Message\n\ndiff --git a/x b/x\nindex 123..456 100644\n--- a/x\n+++ b/x\n@@ -1 +1 @@\n-old line\n+new line with héllo and a long suffix that wraps\n context\n\tindented\n```\n"
	doc, err := document.ReadText("gloss-stdin-123", []byte(source))
	if err != nil {
		t.Fatal(err)
	}
	for _, theme := range []string{"light", "dark"} {
		view := newMarkdownView(&document.Markdown{Source: doc.Markdown}, nil)
		view.fenced, view.theme, view.numbers = true, theme, true
		view.layout(30, 50, 8, 16)
		if view.lastLine != strings.Count(source, "\n") {
			t.Fatalf("file lines = %d", view.lastLine)
		}
		var deleted, inserted string
		for _, unit := range view.content.units {
			if unit.plain == "-old line" {
				deleted = unit.styled
			}
			if strings.HasPrefix(unit.plain, "+new line") {
				inserted = unit.styled
			}
		}
		if !strings.Contains(deleted, "\x1b[") || !strings.Contains(inserted, "\x1b[") || strings.Split(deleted, "-old")[0] == strings.Split(inserted, "+new")[0] {
			t.Fatalf("%s missing distinct patch colors: %q, %q", theme, deleted, inserted)
		}
		assertTextSpans(t, view, 30)
		view.chop = true
		view.layout(30, 50, 8, 16)
		if len(view.lines) != strings.Count(source, "\n") || !strings.Contains(ansi.Strip(view.view()), "-old line") {
			t.Fatal("chopping lost file lines")
		}
		view.chop = false
		view.layout(80, 50, 8, 16)
		assertTextSpans(t, view, 80)
	}
}

func TestDirectTextKeepsTruncationNote(t *testing.T) {
	doc, err := document.ReadText("stdin", []byte(strings.Repeat("a line\n", 400000)))
	if err != nil {
		t.Fatal(err)
	}
	content := fencedFileContent(&document.Markdown{Source: doc.Markdown}, "light")
	if content == nil || content.fileLines == 0 {
		t.Fatal("no direct text content")
	}
	var tail strings.Builder
	for _, unit := range content.units[content.fileLines:] {
		tail.WriteString(unit.plain)
	}
	if !strings.Contains(tail.String(), "lines shown") {
		t.Fatalf("missing truncation note: %q", tail.String())
	}
}

func TestLongTypesetNavigation(t *testing.T) {
	m := lineModel(t, "history.diff", strings.Repeat("+ a line of patch history\n", 10000))
	cmd, handled := m.documentKey("t")
	drainAnimation(t, m, cmd)
	if !handled || m.typesetShown == nil || m.typesetShown.Chunks < 2 || m.typesetChunk != 0 {
		t.Fatalf("typeset toggle: %+v", m.typesetShown)
	}
	cmd, _ = m.documentKey("end")
	drainAnimation(t, m, cmd)
	if m.typesetChunk != m.typesetShown.Chunks-1 || m.typesetShown.Line < 9700 {
		t.Fatalf("end: %+v", m.typesetShown)
	}
	chunk := m.typesetChunk
	cmd, _ = m.documentKey("+")
	drainAnimation(t, m, cmd)
	if m.typesetChunk != chunk || m.typesetShown.Zoom != 1 {
		t.Fatalf("zoom lost chunk: %+v", m.typesetShown)
	}
	for range 3 {
		cmd, _ = m.documentKey("-")
		_ = m.View()
		drainAnimation(t, m, cmd)
		_ = m.View()
	}
	if m.typesetChunk != chunk || m.typesetShown.Zoom != -2 || !m.typesetMatches() {
		t.Fatalf("zoom-out lost chunk: %+v", m.typesetShown)
	}
	// A separate export loader must recreate and render the selected chunk.
	loader := &document.Loader{Files: m.opts.FilesFS}
	defer loader.Close()
	r := loader.Load(m.exportRequest())
	if r.Err != nil || r.Image == nil || r.TypesetView.Chunk != chunk {
		t.Fatalf("export: %v %+v", r.Err, r.TypesetView)
	}
	cmd, _ = m.documentKey("home")
	drainAnimation(t, m, cmd)
	if m.typesetChunk != 0 || m.typesetShown.Line != 1 {
		t.Fatalf("home: %+v", m.typesetShown)
	}
	cmd, _ = m.documentKey("up")
	drainAnimation(t, m, cmd)
	if m.typesetTop != 0 {
		t.Fatal("up crossed before first chunk")
	}
}
