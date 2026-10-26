package app

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	ntopenwith "nimbleterminal.dev/ntopenwith"

	"nimbleterminal.dev/gloss/internal/document"
	"nimbleterminal.dev/gloss/internal/openwith"
)

// The notes the viewer writes speak the language in use; an error's own
// text, a file's name and a count stay as they were.
func TestNotesSpeakTheLanguage(t *testing.T) {
	for _, c := range []struct {
		tag                                   string
		lines, theme, one, many, copied, fail string
		noEditor, fetch                       string
	}{
		{"es", "los números de línea necesitan la vista de código: pulse s", "error al guardar el tema: disk full",
			"1 carácter copiado", "5 caracteres copiados", "Imagen copiada", "error al copiar la imagen: no pasteboard",
			"Ninguna app puede editar notes.txt", "error al descargar: 404 Not Found"},
		{"zh-Hans", "行号需要源码视图：按 s", "保存主题失败：disk full",
			"已复制 1 个字符", "已复制 5 个字符", "图片已复制", "复制图片失败：no pasteboard",
			"没有应用可以编辑 notes.txt", "下载失败：404 Not Found"},
	} {
		t.Run(c.tag, func(t *testing.T) {
			speaking(t, c.tag)
			m := themedModel(t, "dark", false)
			// A plain note: rendered Markdown has no file lines.
			send(m, press("#"))
			if m.note != c.lines {
				t.Errorf("line numbers note = %q, want %q", m.note, c.lines)
			}
			// A prefix gloss adds to an error.
			send(m, themeSaved{err: errors.New("disk full")})
			if m.note != c.theme {
				t.Errorf("theme note = %q, want %q", m.note, c.theme)
			}
			// A count, in its plural form.
			for text, want := range map[string]string{"x": c.one, "héllo": c.many} {
				m.typesetSelected = text
				m.typesetSelectKey("y")
				if m.note != want {
					t.Errorf("copied %q: note = %q, want %q", text, m.note, want)
				}
			}
			m.typesetSelected = ""
			// A picture action's confirmation and its failure.
			send(m, imageCopied{})
			if m.note != c.copied {
				t.Errorf("copied image note = %q, want %q", m.note, c.copied)
			}
			send(m, imageCopied{err: errors.New("no pasteboard")})
			if m.note != c.fail {
				t.Errorf("copy failure note = %q, want %q", m.note, c.fail)
			}
			// Open with: nothing can edit the file; an app's own error stays.
			m.opts.OpenWith = &fakeOpenHost{}
			m.openWithPicker = &openWithPicker{operation: 3, generation: m.generation, verb: openwith.Edit}
			send(m, openWithFinished{operation: 3, generation: m.generation, target: openwith.Target{Path: "/tmp/notes.txt"}, err: ntopenwith.ErrNoEditor})
			if m.note != c.noEditor {
				t.Errorf("no editor note = %q, want %q", m.note, c.noEditor)
			}
			m.openWithPicker = &openWithPicker{operation: 4, generation: m.generation, verb: openwith.Open}
			m.layer = layerOpenWith
			send(m, openWithFinished{operation: 4, generation: m.generation, err: errors.New("exit status 7")})
			if !strings.HasSuffix(m.note, "exit status 7") {
				t.Errorf("open-with failure lost its error: %q", m.note)
			}
			m.closeOpenWith()
			// A download that failed.
			m.fetchedFile(fetchResult{address: "https://example.com/x", err: errors.New("404 Not Found")})
			if m.note != c.fetch {
				t.Errorf("fetch note = %q, want %q", m.note, c.fetch)
			}
		})
	}
}

// A long note in Chinese still leaves the status bar one row at 40 columns.
func TestNoteFitsTheStatusBarInChinese(t *testing.T) {
	speaking(t, "zh-Hans")
	m := viewing(t, Options{Files: []string{"sales.csv"}}, "")
	m.Update(document.Result{Generation: m.generation, Kind: "csv", Page: 1, Pages: 1, Sheet: &document.Sheet{Name: "sales", Rows: [][]string{{"a", "b"}, {"1", "2"}}, Columns: 2, Delimiter: "comma"}})
	m.Update(tea.WindowSizeMsg{Width: 40, Height: 20})
	m.saveLink("https://example.com/report.pdf") // No --fetch.
	if want := "需以 --fetch 运行才能保存链接的文件"; m.note != want {
		t.Fatalf("note = %q, want %q", m.note, want)
	}
	view := ansi.Strip(m.View().Content)
	lines := strings.Split(view, "\n")
	if len(lines) != 20 {
		t.Fatalf("%d rows, want 20:\n%s", len(lines), view)
	}
	fits(t, view, 40)
	if bar := lines[len(lines)-2]; !strings.Contains(bar, "需以") {
		t.Errorf("status bar lacks the note: %s", bar)
	}
}

// Other words the viewer draws itself: a picture on its way, the find
// prompt, the mesh's key row and the GIF scrubber.
func TestViewerPlaceholdersSpeakTheLanguage(t *testing.T) {
	speaking(t, "es")
	f := newFinder(".", 80)
	if view := ansi.Strip(f.view(80, 5)); !strings.Contains(view, "Buscar:") || !strings.Contains(view, "Escriba qué buscar y pulse Enter") {
		t.Errorf("finder:\n%s", view)
	}
	f.running = true
	if view := ansi.Strip(f.view(80, 5)); !strings.Contains(view, "buscando…") {
		t.Errorf("finder while searching:\n%s", view)
	}
	if got := kindWord("ipynb"); got != "cuaderno" {
		t.Errorf("notebook tile = %q", got)
	}
	speaking(t, "zh-Hans")
	m := New(Options{Render: "glyph"})
	t.Cleanup(func() { m.Close() })
	m.width = 60
	if _, cells, suffix := m.scrubberGeometry(); !strings.Contains(suffix, "逐帧") || cells != 60-4-ansi.StringWidth(suffix) {
		t.Errorf("scrubber: %d cells, suffix %q", cells, suffix)
	}
}
