// Package i18n looks up gloss's translated strings.
//
// English lives in the source: every call carries its English default and a
// literal dotted ID, and a catalog (locales/NAME.toml) overrides it.
//
//	i18n.T("app.search.none", "No matches")
//	i18n.Tf("app.search.count", "%d of %d", n, total)
//	i18n.Tn("app.files", "{{.Count}} file", "{{.Count}} files", n)
package i18n

import (
	"bytes"
	"embed"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"text/template"

	"github.com/BurntSushi/toml"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

//go:embed locales/*.toml
var localeFS embed.FS

// Lang describes a language gloss can show.
type Lang struct {
	Tag     string // catalog tag: en, es, zh-Hans
	Name    string // in the language itself
	English string // in English
}

// Languages lists the catalogs, English first.
func Languages() []Lang {
	return []Lang{
		{"en", "English", "English"},
		{"es", "Español", "Spanish"},
		{"zh-Hans", "简体中文", "Chinese (Simplified)"},
	}
}

var (
	loadOnce sync.Once
	bundle   *goi18n.Bundle
	// own holds each catalog's own "other" strings by ID.
	own map[string]map[string]string
)

func load() {
	loadOnce.Do(func() {
		bundle = goi18n.NewBundle(language.English)
		bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)
		own = map[string]map[string]string{}
		entries, err := localeFS.ReadDir("locales")
		if err != nil {
			return
		}
		for _, e := range entries {
			mf, err := bundle.LoadMessageFileFS(localeFS, "locales/"+e.Name())
			if err != nil {
				continue // a broken catalog leaves that language in English
			}
			m := map[string]string{}
			for _, msg := range mf.Messages {
				m[msg.ID] = msg.Other
			}
			own[mf.Tag.String()] = m
		}
	})
}

// Localizer translates into one catalog. The zero value and a nil *Localizer
// are English.
type Localizer struct {
	tag string
	loc *goi18n.Localizer // nil for English
}

// For returns the localizer for tag, matched to a catalog as Match does.
func For(tag string) *Localizer {
	tag = Match(tag)
	if tag == "en" {
		return &Localizer{tag: "en"}
	}
	load()
	return &Localizer{tag: tag, loc: goi18n.NewLocalizer(bundle, tag, "en")}
}

// Tag returns the catalog tag: en, es or zh-Hans.
func (l *Localizer) Tag() string {
	if l == nil || l.tag == "" {
		return "en"
	}
	return l.tag
}

// T returns the string for id, or def when the catalog has none.
func (l *Localizer) T(id, def string) string {
	if l == nil || l.loc == nil {
		return def
	}
	s, err := l.loc.Localize(&goi18n.LocalizeConfig{
		DefaultMessage: &goi18n.Message{ID: id, Other: def},
	})
	if err != nil {
		return def
	}
	return s
}

// Tf is T followed by fmt.Sprintf.
func (l *Localizer) Tf(id, def string, args ...any) string {
	return fmt.Sprintf(l.T(id, def), args...)
}

// Tn picks the plural form for n and executes it as a template with Count.
// one and other are the English forms; without a catalog entry they are used
// as they are.
func (l *Localizer) Tn(id, one, other string, n int) string {
	if l != nil && l.loc != nil {
		s, err := l.loc.Localize(&goi18n.LocalizeConfig{
			DefaultMessage: &goi18n.Message{ID: id, One: one, Other: other},
			PluralCount:    n,
			TemplateData:   map[string]int{"Count": n},
		})
		if err == nil {
			return s
		}
	}
	tmpl := other
	if n == 1 {
		tmpl = one
	}
	return countTemplate(tmpl, n)
}

// countTemplate executes tmpl with {{.Count}}; a template that does not parse
// is returned as written.
func countTemplate(tmpl string, n int) string {
	t, err := template.New("").Parse(tmpl)
	if err != nil {
		return tmpl
	}
	var b bytes.Buffer
	if err := t.Execute(&b, map[string]int{"Count": n}); err != nil {
		return tmpl
	}
	return b.String()
}

// Messages returns the catalog's own strings whose IDs start with prefix
// (plural messages give their "other" form). English has none.
func (l *Localizer) Messages(prefix string) map[string]string {
	out := map[string]string{}
	if l == nil || l.loc == nil {
		return out
	}
	load()
	for id, s := range own[l.tag] {
		if strings.HasPrefix(id, prefix) {
			out[id] = s
		}
	}
	return out
}

var active atomic.Pointer[Localizer]

// SetLanguage makes tag, matched to a catalog, the process-wide language.
// The empty tag is English. It is safe to call while others use T.
func SetLanguage(tag string) {
	active.Store(For(tag))
}

// Language returns the active catalog tag.
func Language() string { return active.Load().Tag() }

// T is Localizer.T on the active language.
func T(id, def string) string { return active.Load().T(id, def) }

// Tf is Localizer.Tf on the active language.
func Tf(id, def string, args ...any) string { return active.Load().Tf(id, def, args...) }

// Tn is Localizer.Tn on the active language.
func Tn(id, one, other string, n int) string { return active.Load().Tn(id, one, other, n) }
