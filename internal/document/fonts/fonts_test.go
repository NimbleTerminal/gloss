package fonts_test

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/go-text/typesetting/font"
	"nimbleterminal.dev/nttypeset"

	"nimbleterminal.dev/gloss/internal/document/fonts"
)

var variants = []string{"Regular", "Bold", "Italic", "BoldItalic"}

func TestFontSubsetHashes(t *testing.T) {
	manifest, err := os.ReadFile("subsets.sha256")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(manifest)), "\n")
	if len(lines) != 28 {
		t.Fatal("expected 28 font faces")
	}
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			t.Fatal("invalid hash manifest")
		}
		data, err := os.ReadFile(fields[1])
		if err != nil {
			t.Fatal(err)
		}
		if fmt.Sprintf("%x", sha256.Sum256(data)) != fields[0] {
			t.Fatalf("subset changed: %s", fields[1])
		}
	}
}

func TestSourceFamilies(t *testing.T) {
	families := fonts.Families()
	if len(families) != 7 || !slices.IsSorted(families) {
		t.Fatalf("families %v", families)
	}
	for _, family := range families {
		for _, variant := range variants {
			data, err := fonts.Embedded.Face(family, variant)
			if err != nil {
				t.Fatal(family, variant, err)
			}
			if _, err := font.ParseTTF(bytes.NewReader(data)); err != nil {
				t.Fatal(family, variant, err)
			}
		}
	}
	if _, err := fonts.Embedded.Face("Nope", "Regular"); err == nil || !strings.Contains(err.Error(), "Nope") {
		t.Fatal("unknown family accepted", err)
	}
	if _, err := fonts.Embedded.Face("Literata", "Heavy"); err == nil || !strings.Contains(err.Error(), "Heavy") {
		t.Fatal("unknown variant accepted", err)
	}
}

func TestSubsetCoverage(t *testing.T) {
	for _, family := range fonts.Families() {
		for _, variant := range variants {
			data, err := fonts.Embedded.Face(family, variant)
			if err != nil {
				t.Fatal(err)
			}
			face, err := font.ParseTTF(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			for _, r := range "Café naïve résumé é — “quotes”…" {
				if _, ok := face.NominalGlyph(r); !ok {
					t.Fatalf("%s %s lacks %U", family, variant, r)
				}
			}
			for _, r := range "漢🙂" {
				if _, ok := face.NominalGlyph(r); ok {
					t.Fatalf("%s %s ships out-of-scope glyph %U", family, variant, r)
				}
			}
		}
	}
}

func TestLicenses(t *testing.T) {
	licenses := fonts.Licenses()
	if len(licenses) != 7 {
		t.Fatalf("%d licenses", len(licenses))
	}
	want := []string{"Literata", "Newsreader", "IBM Plex Sans", "Source Serif 4", "Iosevka", "IBM Plex Mono", "JetBrains Mono"}
	for i, l := range licenses {
		if l.Family != want[i] {
			t.Fatalf("license %d is %q, want %q", i, l.Family, want[i])
		}
		if !strings.Contains(strings.ToUpper(l.Text), "SIL OPEN FONT LICENSE") {
			t.Fatalf("%s license lacks the OFL text", l.Family)
		}
	}
}

func TestEmbeddedIsFaceSource(t *testing.T) {
	var _ nttypeset.FaceSource = fonts.Embedded
}
