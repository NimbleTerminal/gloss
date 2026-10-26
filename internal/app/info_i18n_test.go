package app

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
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

// Every heading and label document writes, as a literal, has its case in
// infoLabel: a new one cannot reach the box in English unnoticed.
func TestInfoLabelKnowsEveryDocumentLabel(t *testing.T) {
	literal := func(e ast.Expr) (string, bool) {
		if bl, ok := e.(*ast.BasicLit); ok && bl.Kind == token.STRING {
			s, err := strconv.Unquote(bl.Value)
			return s, err == nil
		}
		return "", false
	}
	isField := func(e ast.Expr) bool {
		switch e := e.(type) {
		case *ast.Ident:
			return e.Name == "Field"
		case *ast.SelectorExpr:
			return e.Sel.Name == "Field"
		}
		return false
	}
	parse := func(pattern string) []*ast.File {
		files, err := filepath.Glob(pattern)
		if err != nil || len(files) == 0 {
			t.Fatalf("%s: %v", pattern, err)
		}
		var out []*ast.File
		for _, f := range files {
			if strings.HasSuffix(f, "_test.go") {
				continue
			}
			file, err := parser.ParseFile(token.NewFileSet(), f, nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			out = append(out, file)
		}
		return out
	}
	// The cases infoLabel has.
	known := map[string]bool{}
	for _, file := range parse("info_labels.go") {
		ast.Inspect(file, func(n ast.Node) bool {
			if cc, ok := n.(*ast.CaseClause); ok {
				for _, e := range cc.List {
					if s, ok := literal(e); ok {
						known[s] = true
					}
				}
			}
			return true
		})
	}
	// The labels document and the box's terminal section write.
	labels := map[string]bool{}
	field := func(cl *ast.CompositeLit) {
		for i, el := range cl.Elts {
			if kv, ok := el.(*ast.KeyValueExpr); ok {
				if id, ok := kv.Key.(*ast.Ident); ok && id.Name == "Label" {
					if s, ok := literal(kv.Value); ok {
						labels[s] = true
					}
				}
			} else if i == 0 {
				if s, ok := literal(el); ok {
					labels[s] = true
				}
			}
		}
	}
	for _, file := range append(append(parse("../document/*.go"), parse("info.go")...), parse("chart_pane.go")...) {
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.CallExpr:
				name := ""
				switch fn := n.Fun.(type) {
				case *ast.Ident:
					name = fn.Name
				case *ast.SelectorExpr:
					name = fn.Sel.Name
				}
				if name == "Section" && len(n.Args) > 0 {
					if s, ok := literal(n.Args[0]); ok {
						labels[s] = true
					}
				}
			case *ast.CompositeLit:
				if isField(n.Type) {
					field(n)
				}
				if at, ok := n.Type.(*ast.ArrayType); ok && isField(at.Elt) {
					for _, el := range n.Elts {
						if cl, ok := el.(*ast.CompositeLit); ok && cl.Type == nil {
							field(cl)
						}
					}
				}
			}
			return true
		})
	}
	if len(labels) < 100 {
		t.Fatalf("found only %d labels: the scan is broken", len(labels))
	}
	for label := range labels {
		if !known[label] {
			t.Errorf("infoLabel has no case for %q", label)
		}
	}
}
