package theme

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/json"
	"fmt"
	"image/color"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/styles"
	"nimbleterminal.dev/nttypeset"
)

// The catalog and preview interaction are adapted from go-thinkt. See
// LICENSE.thinkt. Snapshots are immutable: there is no process-global active theme.
//
//go:embed themes/*.json
var presets embed.FS

type Definition struct {
	Name       string `json:"name"`
	Base       string `json:"base"`
	Syntax     string `json:"syntax"`
	Background string `json:"background,omitempty"`
	Foreground string `json:"foreground,omitempty"`
	Muted      string `json:"muted,omitempty"`
	Border     string `json:"border,omitempty"`
	Code       string `json:"code,omitempty"`
	Header     string `json:"header,omitempty"`
	Accent     string `json:"accent,omitempty"`
	Selection  string `json:"selection,omitempty"`
	Hover      string `json:"hover,omitempty"`
}
type Choice struct{ Name, Ref string }
type snapshot struct {
	name, base, syntax string
	palette            Palette
}

var catalog sync.Map // immutable snapshot values, keyed by built-in name or content hash
var validName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

func init() {
	entries, _ := presets.ReadDir("themes")
	for _, entry := range entries {
		data, _ := presets.ReadFile("themes/" + entry.Name())
		s, err := decode(data)
		if err != nil {
			panic(err)
		}
		catalog.Store(s.name, s)
	}
}
func decode(data []byte) (snapshot, error) {
	var d Definition
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d); err != nil {
		return snapshot{}, err
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return snapshot{}, fmt.Errorf("theme must contain one JSON object")
	}
	if !validName.MatchString(d.Name) || d.Name == "auto" {
		return snapshot{}, fmt.Errorf("invalid theme name %q", d.Name)
	}
	if d.Base != "light" && d.Base != "dark" {
		return snapshot{}, fmt.Errorf("theme base must be light or dark")
	}
	p := Get(d.Base)
	if d.Syntax == "" {
		d.Syntax = Syntax(d.Base)
	}
	if _, ok := styles.Registry[d.Syntax]; !ok {
		return snapshot{}, fmt.Errorf("unknown syntax style %q", d.Syntax)
	}
	for _, field := range []struct {
		value string
		dst   *color.RGBA
	}{{d.Background, &p.Background}, {d.Foreground, &p.Foreground}, {d.Muted, &p.Muted}, {d.Border, &p.Border}, {d.Code, &p.Code}, {d.Header, &p.Header}, {d.Accent, &p.Accent}, {d.Selection, &p.Selection}, {d.Hover, &p.Hover}} {
		if field.value == "" {
			continue
		}
		var n uint32
		if len(field.value) != 7 || field.value[0] != '#' {
			return snapshot{}, fmt.Errorf("theme colors must be #RRGGBB")
		}
		for _, c := range field.value[1:] {
			if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
				return snapshot{}, fmt.Errorf("invalid theme color %q", field.value)
			}
		}
		if _, err := fmt.Sscanf(field.value, "#%06x", &n); err != nil {
			return snapshot{}, err
		}
		*field.dst = rgb(n)
	}
	return snapshot{d.Name, d.Base, d.Syntax, p}, nil
}

func Name(ref string) string {
	if s, ok := catalog.Load(ref); ok {
		return s.(snapshot).name
	}
	return ref
}
func Dark(ref string) bool {
	if s, ok := catalog.Load(ref); ok {
		return s.(snapshot).base == "dark"
	}
	return ref == "dark"
}
func Syntax(ref string) string {
	if s, ok := catalog.Load(ref); ok {
		return s.(snapshot).syntax
	}
	if ref == "dark" {
		return "github-dark"
	}
	return "github"
}

// Typeset is the theme's colors and syntax style for typeset pages.
func Typeset(ref string) nttypeset.Palette {
	p := Get(ref)
	return nttypeset.Palette{Background: p.Background, Foreground: p.Foreground, Muted: p.Muted, Border: p.Border, Code: p.Code, Header: p.Header, Accent: p.Accent, Selection: p.Selection, Mark: p.Hover, Syntax: styles.Get(Syntax(ref))}
}

func Ink(ref string, token chroma.TokenType) color.RGBA {
	c := styles.Get(Syntax(ref)).Get(token).Colour
	if !c.IsSet() {
		return Get(ref).Foreground
	}
	return color.RGBA{R: c.Red(), G: c.Green(), B: c.Blue(), A: 255}
}
func Builtins() []Choice {
	result := []Choice{{"light", "light"}, {"dark", "dark"}}
	catalog.Range(func(k, v any) bool {
		s := v.(snapshot)
		if k.(string) == s.name {
			result = append(result, Choice{s.name, k.(string)})
		}
		return true
	})
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}
func Dir() (string, error) { p, err := os.UserConfigDir(); return filepath.Join(p, "gloss"), err }

// Load resolves an explicit name or JSON path once. Renderers only read the
// resulting immutable reference; they never access the filesystem.
func Load(name string) (string, error) {
	if name == "auto" || name == "light" || name == "dark" {
		return name, nil
	}
	if _, ok := catalog.Load(name); ok {
		return name, nil
	}
	path := name
	if validName.MatchString(name) {
		dir, err := Dir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(dir, "themes", name+".json")
	}
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("load theme %q: %w", name, err)
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 64*1024+1))
	if err != nil {
		return "", err
	}
	if len(data) > 64*1024 {
		return "", fmt.Errorf("theme exceeds 64 KiB")
	}
	s, err := decode(data)
	if err != nil {
		return "", err
	}
	ref := fmt.Sprintf("theme:%x", sha256.Sum256(data))
	catalog.LoadOrStore(ref, s)
	return ref, nil
}

// Available lists built-ins and at most 64 user files. Malformed files are
// reported when explicitly selected via --theme, and omitted from the picker.
func Available() []Choice {
	result := Builtins()
	dir, err := Dir()
	if err != nil {
		return result
	}
	entries, err := os.ReadDir(filepath.Join(dir, "themes"))
	if err != nil {
		return result
	}
	count := 0
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		if count == 64 {
			break
		}
		count++
		ref, err := Load(filepath.Join(dir, "themes", e.Name()))
		if err == nil {
			result = append(result, Choice{Name(ref), ref})
		}
	}
	return result
}

// SaveChoice writes a self-contained preference so custom file changes cannot
// change an already accepted choice. Only native interactive hosts call this.
func SaveChoice(ref string) error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	return saveChoice(dir, ref)
}
func saveChoice(dir, ref string) error {
	var err error
	if err = os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data := []byte(ref)
	if s, ok := catalog.Load(ref); ok && strings.HasPrefix(ref, "theme:") {
		p := s.(snapshot)
		hex := func(c color.RGBA) string { return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B) }
		hover := "" // Left out, the hover tint stays a blend.
		if p.palette.Hover != (color.RGBA{}) {
			hover = hex(p.palette.Hover)
		}
		data, err = json.Marshal(Definition{p.name, p.base, p.syntax, hex(p.palette.Background), hex(p.palette.Foreground), hex(p.palette.Muted), hex(p.palette.Border), hex(p.palette.Code), hex(p.palette.Header), hex(p.palette.Accent), hex(p.palette.Selection), hover})
		if err != nil {
			return err
		}
	}
	f, err := os.CreateTemp(dir, ".theme-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), filepath.Join(dir, "theme"))
}
func SavedChoice() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, "theme")
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, 64*1024+1))
	if err != nil {
		return "", err
	}
	switch string(b) {
	case "auto", "light", "dark":
		return string(b), nil
	}
	if _, ok := catalog.Load(string(b)); ok {
		return string(b), nil
	}
	return Load(path)
}
