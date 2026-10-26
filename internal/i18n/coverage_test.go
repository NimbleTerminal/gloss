package i18n

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

// The tests here keep the catalogs (locales/*.toml) and the strings in the
// source in step: every ID the source asks for is translated, every
// translation is still asked for, and a translation keeps its format verbs.

// use is one call of T, Tf, Tn, {{t}} or msg() found in the source.
type use struct {
	ID   string
	Kind string   // "T", "Tf", "Tn", "tmpl" or "msg"
	Defs []string // the English default or, for Tn, one and other
	File string
}

var idPattern = regexp.MustCompile(`^[a-z][A-Za-z0-9]*(\.[A-Za-z0-9]+)+$`)

// strLit matches a double-quoted, single-quoted or raw string literal.
const strLit = "(\"(?:[^\"\\\\\\n]|\\\\.)*\"|'(?:[^'\\\\\\n]|\\\\.)*'|`[^`]*`)"

var (
	goCallT  = regexp.MustCompile(`\b(T|Tf)\(\s*` + strLit + `\s*,\s*` + strLit)
	goCallTn = regexp.MustCompile(`\bTn\(\s*` + strLit + `\s*,\s*` + strLit + `\s*,\s*` + strLit)
	tmplCall = regexp.MustCompile(`\{\{-?\s*t\s+` + strLit + `\s+` + strLit)
	jsCall   = regexp.MustCompile(`\bmsg\(\s*` + strLit + `\s*,\s*` + strLit)
)

// scanSource returns the calls in src, a file named name. The file's
// extension says which forms count: Go calls in .go, msg() in .html and .mjs,
// and templates anywhere. Comment lines are skipped.
func scanSource(name, src string) []use {
	var lines []string
	for _, line := range strings.Split(src, "\n") {
		if t := strings.TrimSpace(line); strings.HasPrefix(t, "//") || strings.HasPrefix(t, "*") || strings.HasPrefix(t, "/*") {
			continue
		}
		lines = append(lines, line)
	}
	src = strings.Join(lines, "\n")

	var out []use
	// add records a match: its groups are the ID, then the defaults.
	add := func(kind string, groups []string) {
		u := use{ID: unquote(groups[0]), Kind: kind, File: name}
		for _, d := range groups[1:] {
			u.Defs = append(u.Defs, unquote(d))
		}
		out = append(out, u)
	}
	ext := filepath.Ext(name)
	if ext == ".go" {
		for _, m := range goCallT.FindAllStringSubmatch(src, -1) {
			add(m[1], m[2:])
		}
		for _, m := range goCallTn.FindAllStringSubmatch(src, -1) {
			add("Tn", m[1:])
		}
	}
	for _, m := range tmplCall.FindAllStringSubmatch(src, -1) {
		add("tmpl", m[1:])
	}
	if ext == ".html" || ext == ".mjs" {
		for _, m := range jsCall.FindAllStringSubmatch(src, -1) {
			add("msg", m[1:])
		}
	}
	return out
}

// unquote returns the text of a string literal.
func unquote(lit string) string {
	if strings.HasPrefix(lit, "'") {
		lit = `"` + strings.ReplaceAll(strings.ReplaceAll(lit[1:len(lit)-1], `\'`, `'`), `"`, `\"`) + `"`
	}
	if s, err := strconv.Unquote(lit); err == nil {
		return s
	}
	return lit[1 : len(lit)-1]
}

// scanTree returns every call under cmd, internal and web, relative to the
// module root. A call with a malformed ID is an error.
func scanTree(t *testing.T) []use {
	t.Helper()
	root := filepath.Join("..", "..")
	var out []use
	for _, dir := range []string{"cmd", "internal", "web"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			base := d.Name()
			if d.IsDir() {
				if base == "dist" || base == "node_modules" {
					return fs.SkipDir
				}
				return nil
			}
			switch ext := filepath.Ext(base); {
			case ext == ".go":
				if strings.HasSuffix(base, "_test.go") {
					return nil
				}
			case ext == ".html", ext == ".mjs":
				if strings.HasSuffix(base, ".test.mjs") {
					return nil
				}
			default:
				return nil
			}
			src, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(root, path)
			out = append(out, scanSource(filepath.ToSlash(rel), string(src))...)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, u := range out {
		if !idPattern.MatchString(u.ID) {
			t.Errorf("%s: %s(%q): the ID must be dotted, like app.search.none", u.File, u.Kind, u.ID)
		}
	}
	return out
}

// catalog holds one language's messages: ID to plural form to text. A
// message with no forms of its own has the one form "other".
type catalog map[string]map[string]string

var pluralForms = map[string]bool{"zero": true, "one": true, "two": true, "few": true, "many": true, "other": true}

// readCatalog reads locales/tag.toml as written, without go-i18n.
func readCatalog(t *testing.T, tag string) catalog {
	t.Helper()
	var raw map[string]any
	if _, err := toml.DecodeFile(filepath.Join("locales", tag+".toml"), &raw); err != nil {
		t.Fatalf("%s.toml: %v", tag, err)
	}
	c := catalog{}
	flattenCatalog(c, "", raw)
	return c
}

// flattenCatalog collects the messages in tables, which TOML nests by the
// dots of an ID ([app.help.title] is app, help, title).
func flattenCatalog(c catalog, prefix string, table map[string]any) {
	forms := map[string]string{}
	for k, v := range table {
		if s, ok := v.(string); ok && pluralForms[k] && prefix != "" {
			forms[k] = s
		}
	}
	if len(forms) > 0 {
		c[prefix] = forms
		return
	}
	for k, v := range table {
		id := k
		if prefix != "" {
			id = prefix + "." + k
		}
		switch v := v.(type) {
		case string:
			c[id] = map[string]string{"other": v}
		case map[string]any:
			flattenCatalog(c, id, v)
		}
	}
}

func catalogTags() []string {
	var tags []string
	for _, l := range Languages() {
		if l.Tag != "en" {
			tags = append(tags, l.Tag)
		}
	}
	return tags
}

func isTestID(id string) bool { return strings.HasPrefix(id, "test.") }

func TestCatalogsLoad(t *testing.T) {
	entries, err := os.ReadDir("locales")
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	b := goi18n.NewBundle(language.English)
	b.RegisterUnmarshalFunc("toml", toml.Unmarshal)
	for _, e := range entries {
		name := e.Name()
		if filepath.Ext(name) != ".toml" {
			t.Errorf("locales/%s is not a .toml catalog", name)
			continue
		}
		tag := strings.TrimSuffix(name, ".toml")
		files = append(files, tag)
		readCatalog(t, tag)
		if _, err := b.LoadMessageFileFS(localeFS, "locales/"+name); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	sort.Strings(files)
	want := catalogTags()
	sort.Strings(want)
	if fmt.Sprint(files) != fmt.Sprint(want) {
		t.Errorf("locales has %v, Languages() has %v", files, want)
	}
	// load swallows a broken catalog; it must have kept every one.
	load()
	for _, tag := range want {
		if own[tag] == nil {
			t.Errorf("%s did not load into the bundle", tag)
		}
	}
}

func TestCatalogCoverage(t *testing.T) {
	uses := scanTree(t)
	for _, tag := range catalogTags() {
		c := readCatalog(t, tag)
		missing := map[string]bool{}
		for _, u := range uses {
			if isTestID(u.ID) {
				continue
			}
			if _, ok := c[u.ID]; !ok {
				missing[u.ID] = true
			}
		}
		if len(missing) > 0 {
			t.Errorf("%s.toml is missing %d IDs:\n  %s", tag, len(missing), strings.Join(sortedKeys(missing), "\n  "))
		}
	}
}

func TestCatalogNoStale(t *testing.T) {
	used := map[string]bool{}
	for _, u := range scanTree(t) {
		used[u.ID] = true
	}
	for _, tag := range catalogTags() {
		stale := map[string]bool{}
		for id := range readCatalog(t, tag) {
			if !isTestID(id) && !used[id] {
				stale[id] = true
			}
		}
		if len(stale) > 0 {
			t.Errorf("%s.toml has %d IDs no source uses:\n  %s", tag, len(stale), strings.Join(sortedKeys(stale), "\n  "))
		}
	}
}

var (
	verbPattern  = regexp.MustCompile("%[-+# 0]*[0-9]*(\\.[0-9]+)?[a-zA-Z%]")
	countPattern = regexp.MustCompile(`\{\{\s*\.Count\s*\}\}`)
)

func verbs(s string) string {
	v := verbPattern.FindAllString(s, -1)
	sort.Strings(v)
	return strings.Join(v, " ")
}

func TestCatalogVerbsMatch(t *testing.T) {
	uses := scanTree(t)
	for _, tag := range catalogTags() {
		c := readCatalog(t, tag)
		for _, u := range uses {
			forms, ok := c[u.ID]
			if !ok {
				continue // TestCatalogCoverage reports it
			}
			switch u.Kind {
			case "Tf":
				for form, s := range forms {
					if got, want := verbs(s), verbs(u.Defs[0]); got != want {
						t.Errorf("%s.toml %s (%s): verbs [%s], source has [%s]", tag, u.ID, form, got, want)
					}
				}
			case "Tn":
				for form, s := range forms {
					def := u.Defs[1] // other
					if form == "one" {
						def = u.Defs[0]
					}
					if got, want := countPattern.MatchString(s), countPattern.MatchString(def); got != want {
						t.Errorf("%s.toml %s (%s): has {{.Count}} = %v, source has %v", tag, u.ID, form, got, want)
					}
				}
			}
		}
	}
}

func TestCatalogPlurals(t *testing.T) {
	need := map[string][]string{"es": {"one", "other"}, "zh-Hans": {"other"}}
	uses := scanTree(t)
	for _, tag := range catalogTags() {
		c := readCatalog(t, tag)
		for _, u := range uses {
			forms, ok := c[u.ID]
			if u.Kind != "Tn" || !ok {
				continue
			}
			for _, f := range need[tag] {
				if _, ok := forms[f]; !ok {
					t.Errorf("%s.toml %s: no %q form", tag, u.ID, f)
				}
			}
		}
	}
}

func TestScannerFindsAllForms(t *testing.T) {
	const goSrc = `package x

// i18n.T("comment.ignored", "x")
func f() {
	a := i18n.T("a.plain", "Plain")
	b := T("a.bare", "Bare")
	c := l.Tf("a.fmt", "%d of %d", 1, 2)
	d := i18n.Tf(
		"a.wrapped",
		"Wrapped %s",
		x)
	e := i18n.Tn("a.count", "{{.Count}} file", "{{.Count}} files", n)
	g := m.Tn("a.recv", "one", "other", n)
	h := T("a.raw", ` + "`" + `Raw "quoted"` + "`" + `)
	i := Format("a.not", "this")
	j := ReallyT("a.not", "this")
	k := T(id, "dynamic")
}
`
	got := scanSource("x.go", goSrc)
	want := []string{
		"T a.plain", "T a.bare", "Tf a.fmt", "Tf a.wrapped", "T a.raw",
		"Tn a.count", "Tn a.recv",
	}
	if g := kinds(got); fmt.Sprint(g) != fmt.Sprint(want) {
		t.Errorf("go: got %v, want %v", g, want)
	}
	for _, u := range got {
		switch u.ID {
		case "a.wrapped":
			if u.Defs[0] != "Wrapped %s" {
				t.Errorf("wrapped default %q", u.Defs[0])
			}
		case "a.count":
			if len(u.Defs) != 2 || u.Defs[0] != "{{.Count}} file" || u.Defs[1] != "{{.Count}} files" {
				t.Errorf("count defaults %q", u.Defs)
			}
		case "a.raw":
			if u.Defs[0] != `Raw "quoted"` {
				t.Errorf("raw default %q", u.Defs[0])
			}
		}
	}

	const page = `<h1>{{t "web.title" "Hello"}}</h1>
{{- t "web.trimmed" "Trim" -}}
<script>
  // msg("comment.ignored", "x")
  el.textContent = msg("web.js", "From script");
  el.title = msg('web.single', 'Single \'quoted\'');
  other.msg_x("web.not", "this");
</script>`
	got = scanSource("page.html", page)
	want = []string{"tmpl web.title", "tmpl web.trimmed", "msg web.js", "msg web.single"}
	if g := kinds(got); fmt.Sprint(g) != fmt.Sprint(want) {
		t.Errorf("html: got %v, want %v", g, want)
	}
	if got[3].Defs[0] != "Single 'quoted'" {
		t.Errorf("single-quoted default %q", got[3].Defs[0])
	}

	got = scanSource("app.mjs", `export const s = msg("web.mjs", "From module");`)
	if g := kinds(got); fmt.Sprint(g) != "[msg web.mjs]" {
		t.Errorf("mjs: got %v", g)
	}
	// msg() is JavaScript only, and T() is Go only.
	if got := scanSource("a.go", `msg("web.js", "x")`); len(got) != 0 {
		t.Errorf("msg in .go: %v", got)
	}
	if got := scanSource("a.mjs", `T("a.b", "x")`); len(got) != 0 {
		t.Errorf("T in .mjs: %v", got)
	}

	for id, ok := range map[string]bool{
		"app.help.title": true, "cmd.flag2.x": true, "web.a1": true,
		"title": false, "App.title": false, "app.": false, "app..x": false, "app.a-b": false,
	} {
		if idPattern.MatchString(id) != ok {
			t.Errorf("idPattern(%q) = %v", id, !ok)
		}
	}
}

func kinds(us []use) []string {
	var out []string
	for _, u := range us {
		out = append(out, u.Kind+" "+u.ID)
	}
	return out
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
