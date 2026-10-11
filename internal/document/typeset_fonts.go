package document

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/font/opentype"
	"github.com/go-text/typesetting/shaping"
)

type FontLook struct {
	Name, Body, Code string
	body, code       string
}

var fontLooks = []FontLook{
	{"readable", "Literata", "Iosevka", "A", "E"},
	{"editorial", "Newsreader", "Iosevka", "B", "E"},
	{"technical", "IBM Plex Sans", "IBM Plex Mono", "C", "F"},
	{"compact", "Source Serif 4", "JetBrains Mono", "D", "G"},
}

func FontLooks() []FontLook { return append([]FontLook(nil), fontLooks...) }
func ResolveFont(name, override string) (FontLook, error) {
	if name == "" {
		name = "readable"
	}
	if override != "" && override != "jetbrains" {
		return FontLook{}, fmt.Errorf("--mono must be jetbrains")
	}
	for _, look := range fontLooks {
		if look.Name == name {
			if override == "jetbrains" {
				look.Code, look.code = "JetBrains Mono", "G"
			}
			return look, nil
		}
	}
	return FontLook{}, fmt.Errorf("--font must be readable, editorial, technical or compact")
}
func FontKey(name, override string) string {
	if name == "" {
		name = "readable"
	}
	return name + "/" + override
}

var noLigatures = []shaping.FontFeature{
	{Tag: opentype.MustNewTag("liga"), Value: 0},
	{Tag: opentype.MustNewTag("clig"), Value: 0},
	{Tag: opentype.MustNewTag("dlig"), Value: 0},
	{Tag: opentype.MustNewTag("calt"), Value: 0},
}

func loadFace(family string, style int) (*font.Face, error) {
	variant := []string{"Regular", "Bold", "Italic", "BoldItalic"}[style&3]
	data, err := typesetFonts.ReadFile("fonts/Gloss" + family + "-" + variant + ".ttf")
	if err != nil {
		return nil, err
	}
	return font.ParseTTF(bytes.NewReader(data))
}
func (e *engine) setFonts(name, override string) (bool, error) {
	key := FontKey(name, override)
	if e.fontKey == key {
		return false, nil
	}
	look, err := ResolveFont(name, override)
	if err != nil {
		return false, err
	}
	var faces [8]*font.Face
	for i := range faces {
		family := look.body
		if i&mono != 0 {
			family = look.code
		}
		faces[i], err = loadFace(family, i)
		if err != nil {
			return false, err
		}
	}
	e.faces = faces
	e.fontKey = key
	e.look = look
	e.fallback = [8]*font.Face{}
	e.missing = map[rune]bool{}
	e.fallbackUsed = map[string]bool{}
	e.cache = map[string]*shaped{}
	e.cacheBytes = 0
	e.glyphs = map[glyphKey]*glyphMask{}
	e.glyphBytes = 0
	return true, nil
}

type fallbackMap struct {
	e     *engine
	style int
}

func (m fallbackMap) ResolveFace(r rune) *font.Face {
	i := m.style & 7
	primary := m.e.faces[i]
	if _, ok := primary.NominalGlyph(r); ok {
		return primary
	}
	if unicode.IsControl(r) || unicode.IsSpace(r) {
		return primary
	}
	fallback := m.e.fallback[i]
	family, label := "A", "Literata"
	if i&mono != 0 {
		family, label = "E", "Iosevka"
	}
	if fallback == nil {
		var err error
		fallback, err = loadFace(family, i)
		if err != nil {
			panic(err)
		}
		m.e.fallback[i] = fallback
	}
	if _, ok := fallback.NominalGlyph(r); ok {
		m.e.fallbackUsed[label] = true
		return fallback
	}
	if len(m.e.missing) < 32 {
		m.e.missing[r] = true
	}
	return primary
}
func (e *engine) fontFields() []Field {
	fields := []Field{{Label: "Body font", Value: e.look.Body}, {Label: "Code font", Value: e.look.Code}}
	if len(e.fallbackUsed) > 0 {
		var names []string
		for name := range e.fallbackUsed {
			names = append(names, name)
		}
		sort.Strings(names)
		fields = append(fields, Field{Label: "Font fallback", Value: strings.Join(names, ", ")})
	}
	if len(e.missing) > 0 {
		var runes []int
		for r := range e.missing {
			runes = append(runes, int(r))
		}
		sort.Ints(runes)
		var names []string
		for _, r := range runes {
			names = append(names, fmt.Sprintf("U+%04X", r))
		}
		fields = append(fields, Field{Label: "Missing glyphs", Value: strings.Join(names, ", ") + " (up to 32; laid-out text)"})
	}
	return fields
}
