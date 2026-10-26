package document

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestLocaleCount(t *testing.T) {
	for _, c := range []struct {
		tag  string
		n    int
		want string
	}{
		{"", 1234567, "1,234,567"},
		{"", 999, "999"},
		{"", -1234, "-1,234"},
		{"en", 1234567, "1,234,567"},
		{"es-ES", 1234567, "1.234.567"},
		{"de-DE", 1234567, "1.234.567"},
		{"zh-Hans", 1234567, "1,234,567"},
		{"not a tag!", 1234567, "1,234,567"},
	} {
		if got := (Locale{c.tag}).Count(c.n); got != c.want {
			t.Errorf("Locale{%q}.Count(%d) = %q, want %q", c.tag, c.n, got, c.want)
		}
	}
}

func TestLocaleBytes(t *testing.T) {
	if got, want := (Locale{}).Bytes(1572864), byteSize(1572864); got != want || got != "1.5 MiB (1,572,864 bytes)" {
		t.Errorf("zero Locale.Bytes = %q, byteSize = %q", got, want)
	}
	for _, n := range []int{0, 1, 2, 1023, 1024, 5000, 1 << 20, 3 << 30} {
		if got, want := (Locale{}).Bytes(n), byteSize(n); got != want {
			t.Errorf("zero Locale.Bytes(%d) = %q, want %q", n, got, want)
		}
	}
	if got, want := (Locale{"es-ES"}).Bytes(1572864), "1,5 MiB (1.572.864 bytes)"; got != want {
		t.Errorf("es-ES = %q, want %q", got, want)
	}
	if got, want := (Locale{"es-ES"}).Bytes(1), "1 byte"; got != want {
		t.Errorf("es-ES 1 = %q, want %q", got, want)
	}
}

func TestInfoUsesRequestLocale(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("a,b\n")
	for i := 0; i < 1200; i++ {
		fmt.Fprintf(&sb, "%d,x\n", i)
	}
	path := filepath.Join(t.TempDir(), "rows.csv")
	if err := os.WriteFile(path, []byte(sb.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	size := len(sb.String())
	load := func(loc Locale) []Field {
		l := &Loader{}
		defer l.Close()
		r := l.Load(Request{Path: path, Page: 1, DPI: 72, Generation: 1, Locale: loc})
		if r.Err != nil {
			t.Fatal(r.Err)
		}
		return r.Info
	}
	expect(t, load(Locale{}), map[string]string{"Rows": "1,201", "Size": byteSize(size)})
	got := load(Locale{"es-ES"})
	expect(t, got, map[string]string{"Rows": "1.201", "Size": (Locale{"es-ES"}).Bytes(size)})
	if !strings.Contains(field(got, "Size"), "KiB (") || field(got, "Size") == byteSize(size) {
		t.Errorf("Size = %q, want KiB with a decimal comma", field(got, "Size"))
	}
}

// Every count in the details is written in the request's language, and the
// zero Locale keeps the English grouping.
func TestInfoCountsUseRequestLocale(t *testing.T) {
	var text, jsonl, stl, tris, outputs strings.Builder
	for i := range 1200 {
		text.WriteString("a b\n")
		fmt.Fprintf(&jsonl, "{\"n\":%d}\n", i)
		if i > 0 {
			outputs.WriteString(",")
		}
		outputs.WriteString(`{"output_type":"stream","name":"stdout","text":"x"}`)
	}
	stl.WriteString("solid many\n")
	for range 1001 {
		stl.WriteString("facet normal 0 0 1\nouter loop\nvertex 0 0 0\nvertex 1 0 0\nvertex 0 1 0\nendloop\nendfacet\n")
		tris.WriteString(`<triangle v1="0" v2="1" v3="2"/>`)
	}
	stl.WriteString("endsolid many\n")
	array := "[" + strings.TrimSuffix(strings.Repeat("1,", 1200), ",") + "]"
	notebook := `{"nbformat":4,"nbformat_minor":5,"metadata":{},"cells":[{"cell_type":"code","source":"print()","metadata":{},"outputs":[` + outputs.String() + `]}]}`
	app := `<?xml version="1.0"?><Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties"><Pages>1234</Pages><Words>56789</Words></Properties>`
	mesh := archive3MF(t, map[string]string{"3D/3dmodel.model": model3MF(`unit="millimeter"`,
		`<resources><object id="1" type="model"><mesh><vertices><vertex x="0" y="0" z="0"/><vertex x="1" y="0" z="0"/><vertex x="0" y="1" z="0"/></vertices><triangles>`+tris.String()+`</triangles></mesh></object></resources><build><item objectid="1"/></build>`)})
	thousands := regexp.MustCompile(`(\d),(\d{3})`)
	for name, data := range map[string][]byte{
		"notes.txt":   []byte(text.String()),
		"rows.jsonl":  []byte(jsonl.String()),
		"list.json":   []byte(array),
		"many.stl":    []byte(stl.String()),
		"cells.ipynb": []byte(notebook),
		"memo.docx":   wordDocument(t, p("", "Hello"), map[string]string{"docProps/app.xml": app}),
		"many.3mf":    mesh,
		"dunes.ply":   nil,
	} {
		path := filepath.Join("..", "..", "examples", name)
		if data != nil {
			path = write(t, name, data)
		}
		load := func(loc Locale) []Field {
			l := &Loader{}
			defer l.Close()
			r := l.Load(Request{Path: path, Page: 1, DPI: 72, Generation: 1, Locale: loc, Describe: true})
			if r.Err != nil {
				t.Fatalf("%s: %v", name, r.Err)
			}
			return r.Info
		}
		english, german := load(Locale{}), load(Locale{"de-DE"})
		if len(english) != len(german) {
			t.Fatalf("%s: %d fields in English, %d in German", name, len(english), len(german))
		}
		grouped := false
		for i, f := range english {
			if f.Label == "Size" || f.Label == "Path" || f.Label == "Modified" {
				continue
			}
			grouped = grouped || thousands.MatchString(f.Value)
			if want := thousands.ReplaceAllString(f.Value, "$1.$2"); german[i].Value != want {
				t.Errorf("%s: %s = %q in German, want %q", name, f.Label, german[i].Value, want)
			}
		}
		if !grouped {
			t.Errorf("%s: no count over a thousand in %+v", name, english)
		}
	}
	if got := plural(Locale{"de-DE"}, 1200, "row"); got != "1.200 rows" {
		t.Errorf("plural = %q", got)
	}
	if got := plural(Locale{}, 1200, "row"); got != "1,200 rows" {
		t.Errorf("zero plural = %q", got)
	}
}
