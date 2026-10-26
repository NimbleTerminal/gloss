package document

import (
	"fmt"

	textlang "golang.org/x/text/language"
	"golang.org/x/text/message"
)

// Locale is the language numbers in a document's details are written in,
// resolved by the caller from the user's language: the loader does not read
// the environment. The zero Locale writes them as they always were.
type Locale struct {
	Tag string // A BCP 47 tag, as es-MX; empty is English.
}

// printer is the Locale's number formatter; false is the English default,
// for an empty tag or one that does not parse.
func (l Locale) printer() (*message.Printer, bool) {
	if l.Tag == "" {
		return nil, false
	}
	tag, err := textlang.Parse(l.Tag)
	if err != nil {
		return nil, false
	}
	return message.NewPrinter(tag), true
}

// Count is n with its digits grouped as the language groups them.
func (l Locale) Count(n int) string {
	p, ok := l.printer()
	if !ok {
		return grouped(n)
	}
	return p.Sprintf("%d", n)
}

// Bytes is byteSize with localized numbers. The units and the word bytes
// stay English.
func (l Locale) Bytes(n int) string {
	p, ok := l.printer()
	if !ok {
		return byteSize(n)
	}
	if n == 1 {
		return "1 byte"
	}
	exact := p.Sprintf("%d", n) + " bytes"
	for _, unit := range []struct {
		name string
		size float64
	}{{"GiB", 1 << 30}, {"MiB", 1 << 20}, {"KiB", 1 << 10}} {
		if float64(n) >= unit.size {
			return fmt.Sprintf("%s %s (%s)", p.Sprintf("%.1f", float64(n)/unit.size), unit.name, exact)
		}
	}
	return exact
}
