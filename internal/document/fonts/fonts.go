// Package fonts serves a small set of OFL-licensed font subsets embedded in
// the binary. Source implements the face-source interface of
// nimbleterminal.dev/nttypeset without importing it.
package fonts

import (
	"embed"
	"fmt"
	"sort"
)

//go:embed Gloss*.ttf
var faces embed.FS

//go:embed *.OFL.txt
var licenseFiles embed.FS

type family struct {
	name, letter, slug string
}

// table lists the families in license order.
var table = []family{
	{"Literata", "A", "literata"},
	{"Newsreader", "B", "newsreader"},
	{"IBM Plex Sans", "C", "ibmplexsans"},
	{"Source Serif 4", "D", "sourceserif4"},
	{"Iosevka", "E", "iosevka"},
	{"IBM Plex Mono", "F", "ibmplexmono"},
	{"JetBrains Mono", "G", "jetbrainsmono"},
}

var variants = map[string]bool{"Regular": true, "Bold": true, "Italic": true, "BoldItalic": true}

// Source serves the embedded font subsets.
type Source struct{}

// Embedded is the Source over the embedded subsets.
var Embedded Source

// Face returns the font bytes for a family and a variant (Regular, Bold,
// Italic or BoldItalic). It errors, naming the offender, for anything else.
func (Source) Face(family, variant string) ([]byte, error) {
	if !variants[variant] {
		return nil, fmt.Errorf("fonts: unknown variant %q", variant)
	}
	for _, f := range table {
		if f.name == family {
			return faces.ReadFile("Gloss" + f.letter + "-" + variant + ".ttf")
		}
	}
	return nil, fmt.Errorf("fonts: unknown family %q", family)
}

// Families returns the available family names, sorted.
func Families() []string {
	names := make([]string, len(table))
	for i, f := range table {
		names[i] = f.name
	}
	sort.Strings(names)
	return names
}

// License is the license text shipped for a family.
type License struct{ Family, Text string }

// Licenses returns the OFL text of every family, in the order of the table.
func Licenses() []License {
	out := make([]License, len(table))
	for i, f := range table {
		text, err := licenseFiles.ReadFile(f.slug + ".OFL.txt")
		if err != nil {
			panic(err) // embedded at build time
		}
		out[i] = License{Family: f.name, Text: string(text)}
	}
	return out
}
