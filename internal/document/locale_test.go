package document

import (
	"fmt"
	"os"
	"path/filepath"
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
