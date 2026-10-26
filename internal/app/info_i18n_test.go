package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"nimbleterminal.dev/gloss/internal/document"
)

// The details box draws document's English labels in the language in use;
// the values, and the labels document hands --info, stay as they were.
func TestInfoBoxSpeaksTheLanguage(t *testing.T) {
	loader := &document.Loader{}
	t.Cleanup(func() { _ = loader.Close() })
	r := loader.Load(document.Request{Path: "../../examples/sales.csv", Describe: true, Generation: 1})
	if r.Err != nil {
		t.Fatal(r.Err)
	}
	english := map[string]bool{}
	for _, f := range r.Info {
		english[f.Label] = true
	}
	for _, label := range []string{"File", "Size", "Table", "Rows", "Columns"} {
		if !english[label] {
			t.Fatalf("document gave no %q: %v", label, r.Info)
		}
	}
	for _, c := range []struct {
		tag  string
		want []string
	}{
		{"es", []string{"Archivo", "Tamaño", "Tabla", "Filas", "Columnas", "Terminal", "Tema del texto", "celdas"}},
		{"zh-Hans", []string{"文件", "大小", "表格", "行数", "列数", "终端", "文本主题", "单元格"}},
	} {
		t.Run(c.tag, func(t *testing.T) {
			speaking(t, c.tag)
			m := loaded(t, "sales.csv")
			m.fields = r.Info
			send(m, tea.WindowSizeMsg{Width: 100, Height: 40}, press("i"))
			box := ansi.Strip(m.infoBox(100, 40))
			for _, want := range c.want {
				if !strings.Contains(box, want) {
					t.Errorf("the box lacks %q:\n%s", want, box)
				}
			}
			for _, f := range r.Info {
				if f.Value != "" && !strings.Contains(box, f.Value) {
					t.Errorf("the box lacks the value %q:\n%s", f.Value, box)
				}
			}
			fits(t, box, 100)
			if infoLabel("Not a label gloss knows") != "Not a label gloss knows" {
				t.Error("an unknown label is not shown as it came")
			}
		})
	}
}
