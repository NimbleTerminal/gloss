package chart

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var sales = [][]string{
	{"region", "quarter", "units", "revenue", "booked"},
	{"North", "Q1", "120", "48.5", "2026-03-31"},
	{"South", "Q1", "95", "39.9", "2026-03-31"},
	{"North", "Q2", "131", "52", "2026-06-30"},
	{"South", "Q2", "102", "41.7", "2026-06-30"},
}

func visible(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = i
	}
	return out
}

// golden compares a document to testdata/<name>.json, writing it when
// UPDATE_GOLDEN is set.
func golden(t *testing.T, name string, doc Document) {
	t.Helper()
	path := filepath.Join("testdata", name+".json")
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, doc.Raw, "", "  "); err != nil {
		t.Fatal(err)
	}
	pretty.WriteByte('\n')
	if os.Getenv("UPDATE_GOLDEN") != "" {
		if err := os.WriteFile(path, pretty.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(want, pretty.Bytes()) {
		t.Fatalf("%s differs:\n%s", path, pretty.String())
	}
}

func TestInferSales(t *testing.T) {
	s := Analyze(sales[0], sales[1:], visible(5))
	if s.X == nil || s.X.Name != "booked" || len(s.Measures) != 2 || s.Rows != 4 {
		t.Fatalf("%+v", s)
	}
	if got := s.Templates(); got[0] != TemplateLine || got[1] != TemplateArea {
		t.Fatalf("templates %v", got)
	}
	doc, err := s.Document(TemplateLine)
	if err != nil || doc.Kind != KindFlint || doc.Unbound {
		t.Fatalf("%+v %v", doc, err)
	}
	golden(t, "sales-line", doc)
	// Hiding the date leaves region as x: a stacked bar of two measures.
	s = Analyze(sales[0], sales[1:], []int{0, 1, 2, 3})
	if got := s.Templates(); got[0] != TemplateStackedBar {
		t.Fatalf("templates %v", got)
	}
	doc, _ = s.Document(TemplateStackedBar)
	golden(t, "sales-stacked", doc)
	// One measure and a repeating category sums it.
	s = Analyze(sales[0], sales[1:], []int{0, 3})
	if got := s.Templates(); got[0] != TemplateBar || got[1] != TemplateLollipop {
		t.Fatalf("templates %v", got)
	}
	doc, _ = s.Document(TemplateBar)
	golden(t, "sales-bar", doc)
	if !strings.Contains(string(doc.Raw), `"aggregate":"sum"`) {
		t.Fatal("repeated categories are not summed")
	}
}

func TestInferShapes(t *testing.T) {
	for _, tc := range []struct {
		name   string
		header []string
		rows   [][]string
		want   []string
	}{
		{"heatmap", []string{"region", "quarter", "revenue"}, [][]string{{"N", "Q1", "1"}, {"S", "Q1", "2"}}, []string{TemplateHeatmap}},
		{"scatter", []string{"units", "revenue"}, [][]string{{"1", "2"}, {"3", "4"}}, []string{TemplateScatter, TemplateConnectedScatter}},
		{"histogram", []string{"revenue"}, [][]string{{"1"}, {"2"}, {"2"}}, []string{TemplateHistogram, TemplateECDF, TemplateSparkline}},
		{"candlestick", []string{"day", "open", "high", "low", "close"}, [][]string{{"2026-01-02", "1", "3", "0.5", "2"}}, []string{TemplateCandlestick}},
		{"nothing", []string{"a", "b"}, [][]string{{"x", "y"}}, nil},
		{"empty", []string{"a"}, nil, nil},
	} {
		s := Analyze(tc.header, tc.rows, visible(len(tc.header)))
		got := s.Templates()
		if strings.Join(got, ",") != strings.Join(tc.want, ",") {
			t.Errorf("%s: %v, want %v", tc.name, got, tc.want)
			continue
		}
		for _, template := range got {
			if _, err := s.Document(template); err != nil {
				t.Errorf("%s %s: %v", tc.name, template, err)
			}
		}
	}
	if _, err := Analyze([]string{"a"}, nil, visible(1)).Document(TemplateBar); err == nil {
		t.Fatal("documented an unchartable shape")
	}
}

func TestInferCapsRows(t *testing.T) {
	rows := make([][]string, MaxRows+5)
	for i := range rows {
		rows[i] = []string{"2026-01-01", "1"}
	}
	s := Analyze([]string{"day", "v"}, rows, visible(2))
	if s.Rows != MaxRows || len(s.Notes) != 1 || !strings.Contains(string(s.Notes[0]), "5 rows") {
		t.Fatalf("rows=%d notes=%v", s.Rows, s.Notes)
	}
	header := make([]string, MaxSeries+3)
	row := make([]string, MaxSeries+3)
	header[0], row[0] = "k", "a"
	for i := 1; i < len(header); i++ {
		header[i], row[i] = "m"+strings.Repeat("x", i), "1"
	}
	s = Analyze(header, [][]string{row}, visible(len(header)))
	if len(s.Measures) != MaxSeries || len(s.Notes) != 1 {
		t.Fatalf("measures=%d notes=%v", len(s.Measures), s.Notes)
	}
}

// Long form multiplies rows by measures, and the compiler takes at most
// 65,535 values: wide sheets chart fewer rows and say so.
func TestInferBoundsLongFormValues(t *testing.T) {
	header := []string{"day"}
	for i := 0; i < MaxSeries; i++ {
		header = append(header, "m"+strings.Repeat("x", i))
	}
	rows := make([][]string, MaxRows)
	for r := range rows {
		rows[r] = make([]string, len(header))
		rows[r][0] = "2026-01-01"
		for c := 1; c < len(header); c++ {
			rows[r][c] = "1"
		}
	}
	s := Analyze(header, rows, visible(len(header)))
	if s.Rows != MaxValues/MaxSeries || len(s.Notes) != 1 || !strings.Contains(string(s.Notes[0]), "rows charted") {
		t.Fatalf("rows=%d notes=%v", s.Rows, s.Notes)
	}
	doc, err := s.Document(TemplateLine)
	if err != nil {
		t.Fatal(err)
	}
	// One "series":"…" per long-form row, plus its semantic type.
	if n := strings.Count(string(doc.Raw), `"series":"`); n > MaxValues+1 {
		t.Fatalf("%d long-form rows", n)
	}
}

// Field names never collide, whatever the headers say.
func TestInferNamesStayDistinct(t *testing.T) {
	s := Analyze([]string{"a", "a", "a 2"}, [][]string{{"x", "1", "2"}}, []int{0, 2, 1})
	doc, err := s.Document(TemplateStackedBar)
	if err != nil {
		t.Fatal(err)
	}
	// Long form names each measure in its rows: both "a" columns stay apart.
	for _, want := range []string{`"series":"a 2"`, `"series":"a 3"`} {
		if !strings.Contains(string(doc.Raw), want) {
			t.Fatalf("missing %s in %s", want, doc.Raw)
		}
	}
	// Long form's own fields step aside for columns of the same name.
	s = Analyze([]string{"value", "series", "a", "b"}, [][]string{{"k", "s", "1", "2"}}, visible(4))
	doc, err = s.Document(TemplateStackedBar)
	if err != nil {
		t.Fatal(err)
	}
	var enc struct {
		Spec struct {
			Encodings map[string]struct{ Field string } `json:"encodings"`
		} `json:"chart_spec"`
	}
	if err := json.Unmarshal(doc.Raw, &enc); err != nil {
		t.Fatal(err)
	}
	x, y, color := enc.Spec.Encodings["x"].Field, enc.Spec.Encodings["y"].Field, enc.Spec.Encodings["color"].Field
	if x != "value" || y == "value" || color == "series" || y == color {
		t.Fatalf("x=%s y=%s color=%s", x, y, color)
	}
}

// Header text is cleaned before it names a field: the escape goes, so the
// terminal reads what is left as text.
func TestInferCleansHeaders(t *testing.T) {
	s := Analyze([]string{"re\x1b[2Jgion", "n\x07"}, [][]string{{"N", "1"}}, visible(2))
	if s.X.Name != "re[2Jgion" || s.Measures[0].Name != "n" {
		t.Fatalf("%q %q", s.X.Name, s.Measures[0].Name)
	}
}

func TestInferSkipsAndNames(t *testing.T) {
	s := Analyze([]string{"", "region", "region", "active"}, [][]string{{"1", "N", "S", "TRUE"}}, visible(4))
	// column 1 is a measure, region is x, region 3 is a second category: a heatmap.
	if got := s.Templates(); len(got) != 1 || got[0] != TemplateHeatmap {
		t.Fatalf("templates %v", got)
	}
	doc, err := s.Document(TemplateHeatmap)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"column 1"`, `"region"`, `"region 2"`} {
		if !strings.Contains(string(doc.Raw), want) {
			t.Fatalf("missing %s in %s", want, doc.Raw)
		}
	}
	if len(s.Notes) == 0 || !strings.Contains(string(s.Notes[0]), "active") {
		t.Fatalf("boolean column not reported as skipped: %v", s.Notes)
	}
}
