package document

import (
	"fmt"
	"strings"

	"nimbleterminal.dev/ntfonts"
	"nimbleterminal.dev/nttypeset"
)

// FontLook pairs a body family with a code family, by name.
type FontLook = nttypeset.FontLook

// FontLooks returns the built-in looks: readable, editorial, technical, compact.
func FontLooks() []FontLook { return nttypeset.FontLooks() }

// ResolveFont returns the look called name, with gloss's flag wording for a
// look or mono override it does not know.
func ResolveFont(name, override string) (FontLook, error) {
	if override != "" && override != "jetbrains" {
		return FontLook{}, fmt.Errorf("--mono must be jetbrains")
	}
	look, err := nttypeset.ResolveFont(name, override)
	if err != nil {
		return FontLook{}, fmt.Errorf("--font must be readable, editorial, technical or compact")
	}
	return look, nil
}

// FontKey is the cache key for a look and override.
func FontKey(name, override string) string { return nttypeset.FontKey(name, override) }

// newTypesetEngine builds an engine over gloss's embedded faces.
func newTypesetEngine(font, mono string) (*nttypeset.Engine, error) {
	if _, err := ResolveFont(font, mono); err != nil {
		return nil, err
	}
	return nttypeset.NewEngine(nttypeset.WithFonts(ntfonts.Embedded), nttypeset.WithLook(font, mono))
}

// setTypesetFonts switches the engine's look, with gloss's flag wording for a
// look it does not know.
func setTypesetFonts(e *nttypeset.Engine, font, mono string) (bool, error) {
	if _, err := ResolveFont(font, mono); err != nil {
		return false, err
	}
	return e.SetFonts(font, mono)
}

// fontFields are the info fields for the engine's look: its families, the
// fallback families used and the runes no face has.
func fontFields(e *nttypeset.Engine) []Field {
	look := e.Look()
	fields := []Field{{Label: "Body font", Value: look.Body}, {Label: "Code font", Value: look.Code}}
	if used := e.Fallbacks(); len(used) > 0 {
		fields = append(fields, Field{Label: "Font fallback", Value: strings.Join(used, ", ")})
	}
	if missing := e.Missing(); len(missing) > 0 {
		names := make([]string, 0, len(missing))
		for _, r := range missing {
			names = append(names, fmt.Sprintf("U+%04X", r))
		}
		fields = append(fields, Field{Label: "Missing glyphs", Value: strings.Join(names, ", ") + " (up to 32; laid-out text)"})
	}
	return fields
}
