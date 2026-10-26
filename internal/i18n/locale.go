package i18n

import (
	"errors"
	"strings"

	"golang.org/x/text/language"
)

// maxAcceptLanguage bounds the header FromAcceptLanguage will parse.
const maxAcceptLanguage = 4 << 10

// catalogs are the languages gloss has a catalog for, English first.
var catalogs = []language.Tag{language.English, language.Spanish, language.SimplifiedChinese}

var matcher = language.NewMatcher(catalogs)

// Parse reads a BCP 47 tag (es-MX, zh-Hans) or a POSIX locale name
// (zh_CN.UTF-8, es_ES@euro). The codeset and modifier are dropped and
// underscores become hyphens; C and POSIX are English. An empty or
// unparseable name is an error.
func Parse(s string) (language.Tag, error) {
	name := strings.TrimSpace(s)
	if name == "" {
		return language.Und, errors.New("empty locale")
	}
	if at := strings.IndexByte(name, '@'); at >= 0 {
		if !alnum(name[at+1:]) {
			return language.Und, errors.New("bad locale modifier")
		}
		name = name[:at]
	}
	if dot := strings.IndexByte(name, '.'); dot >= 0 {
		name = name[:dot]
	}
	switch name {
	case "C", "POSIX":
		return language.English, nil
	}
	return language.Parse(strings.ReplaceAll(name, "_", "-"))
}

func alnum(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z') {
			return false
		}
	}
	return true
}

// FromEnv returns the first of GLOSS_LANG, LC_ALL, LC_MESSAGES and LANG that
// Parses, as a tag string, or "en" when none does.
func FromEnv(getenv func(string) string) string {
	for _, name := range []string{"GLOSS_LANG", "LC_ALL", "LC_MESSAGES", "LANG"} {
		if tag, err := Parse(getenv(name)); err == nil {
			return tag.String()
		}
	}
	return "en"
}

// FromAcceptLanguage returns the best tag in an Accept-Language header that
// matches a catalog with High confidence (so "es-MX", not "es"), or "en".
// Headers over 4 KiB are ignored.
func FromAcceptLanguage(header string) string {
	if len(header) > maxAcceptLanguage {
		return "en"
	}
	tags, _, err := language.ParseAcceptLanguage(header)
	if err != nil {
		return "en"
	}
	for _, tag := range tags {
		if _, _, conf := matcher.Match(tag); conf >= language.High {
			return tag.String()
		}
	}
	return "en"
}

// Match returns the catalog that serves tag: "en", "es" or "zh-Hans". Tags
// the catalogs do not cover with High confidence (zh-TW, fr) are "en".
func Match(tag string) string {
	t, err := Parse(tag)
	if err != nil {
		return "en"
	}
	_, i, conf := matcher.Match(t)
	if conf < language.High {
		return "en"
	}
	switch catalogs[i] {
	case language.Spanish:
		return "es"
	case language.SimplifiedChinese:
		return "zh-Hans"
	}
	return "en"
}
