package document

import (
	"bytes"
	"context"
	"image"
	"strings"
	"testing"
	"testing/fstest"
	"unicode"

	"github.com/go-text/typesetting/font"
	"nimbleterminal.dev/ntfonts"
	"nimbleterminal.dev/nttypeset"
)

// The looks resolve, with and without the JetBrains override; anything else
// is refused in gloss's flag wording. Subset coverage is tested in ntfonts.
func TestFontLooksAndSubsets(t *testing.T) {
	for _, look := range FontLooks() {
		if _, err := ResolveFont(look.Name, "jetbrains"); err != nil {
			t.Fatal(err)
		}
		if _, err := newTypesetEngine(look.Name, ""); err != nil {
			t.Fatal(err)
		}
	}
	for _, v := range [][3]string{{"missing", "", "--font must be readable, editorial, technical or compact"}, {"readable", "other", "--mono must be jetbrains"}} {
		if _, err := ResolveFont(v[0], v[1]); err == nil || err.Error() != v[2] {
			t.Fatalf("ResolveFont(%q, %q) = %v, want %q", v[0], v[1], err, v[2])
		}
		if _, err := newTypesetEngine(v[0], v[1]); err == nil || err.Error() != v[2] {
			t.Fatalf("newTypesetEngine(%q, %q) = %v, want %q", v[0], v[1], err, v[2])
		}
	}
}
func TestFontChangesReflowAndPreserveAssets(t *testing.T) {
	l := &Loader{Files: fstest.MapFS{"page.md": {Data: []byte(strings.Repeat("A readable paragraph with café and **bold** words.\n\n", 40))}}}
	defer l.Close()
	q := Request{Theme: testTheme("light"), Path: "page.md", Typeset: true, Font: "readable", TypesetView: &TypesetViewport{Top: 700, Height: 600}}
	r := l.Load(q)
	if r.Err != nil {
		t.Fatal(r.Err)
	}
	doc, eng, layout := l.typesetDocument, l.typesetEngine, l.direct
	q.Font = "editorial"
	r = l.Load(q)
	if r.Err != nil {
		t.Fatal(r.Err)
	}
	if l.typesetDocument != doc || l.typesetEngine != eng || l.direct == layout || r.TypesetView.Top == 0 {
		t.Fatal("font switch lost prepared content or did not reflow")
	}
	layout = l.direct
	q.Theme = testTheme("nord")
	r = l.Load(q)
	if r.Err != nil || l.direct != layout {
		t.Fatal("color change reshaped", r.Err)
	}
	q.Mono = "jetbrains"
	r = l.Load(q)
	if r.Err != nil || l.typesetEngine.Look().Code != "JetBrains Mono" {
		t.Fatal("mono override", r.Err)
	}
	q.Font = "invalid"
	if r = l.Load(q); r.Err == nil || r.Err.Error() != "--font must be readable, editorial, technical or compact" {
		t.Fatal("invalid request accepted", r.Err)
	}
}

// A rune IBM Plex Sans lacks but Literata has falls back to Literata, and the
// info says so; a rune no face has is reported missing. Only the real
// subsets have such a rune, so this is tested here, not in nttypeset.
func TestFontFallbackMissingAndLigatures(t *testing.T) {
	parse := func(family string) *font.Face {
		t.Helper()
		data, err := ntfonts.Embedded.Face(family, "Regular")
		if err != nil {
			t.Fatal(err)
		}
		face, err := font.ParseTTF(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		return face
	}
	primary, fallback := parse("IBM Plex Sans"), parse("Literata")
	var candidate rune
	for r := rune(0x20); r < 0x2070; r++ {
		_, a := primary.NominalGlyph(r)
		_, b := fallback.NominalGlyph(r)
		if !a && b && !unicode.IsControl(r) && !unicode.IsSpace(r) {
			candidate = r
			break
		}
	}
	if candidate == 0 {
		t.Fatal("fixture needs a glyph present only in fallback")
	}
	source := "Fallback " + string(candidate) + " and missing 🙂漢.\n"
	l := &Loader{Files: fstest.MapFS{"page.md": {Data: []byte(source)}}}
	defer l.Close()
	r := l.Load(Request{Theme: testTheme("light"), Path: "page.md", Typeset: true, Font: "technical", TypesetView: &TypesetViewport{Height: 300}})
	if r.Err != nil {
		t.Fatal(r.Err)
	}
	fields := map[string]string{}
	for _, f := range r.Info {
		fields[f.Label] = f.Value
	}
	if !strings.Contains(fields["Font fallback"], "Literata") {
		t.Fatalf("fallback for %U not reported: %v", candidate, r.Info)
	}
	if got := fields["Missing glyphs"]; !strings.Contains(got, "U+1F642") || !strings.Contains(got, "U+6F22") || !strings.HasSuffix(got, " (up to 32; laid-out text)") {
		t.Fatalf("missing glyphs %q", got)
	}
}
func TestFontPreviewBoundedAndCancelled(t *testing.T) {
	im, err := FontPreview(context.Background(), []byte("# Sample\n\n```go\nfunc hello() {}\n```"), testTheme("nord").Palette, "", 480, 320)
	if err != nil || im.Bounds() != image.Rect(0, 0, 480, 320) {
		t.Fatal("preview", err)
	}
	background := im.At(0, 0)
	rowHeight := im.Bounds().Dy() / 4
	for row := 0; row < 4; row++ {
		inked := false
		for y := row * rowHeight; y < (row+1)*rowHeight && !inked; y++ {
			for x := 0; x < im.Bounds().Dx(); x++ {
				if im.At(x, y) != background {
					inked = true
					break
				}
			}
		}
		if !inked {
			t.Fatalf("preview row %d has no ink", row)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := FontPreview(ctx, nil, testTheme("light").Palette, "", 480, 320); err == nil {
		t.Fatal("ignored cancellation")
	}
	if _, err := FontPreview(context.Background(), nil, testTheme("light").Palette, "", 5000, 5000); err == nil {
		t.Fatal("unbounded preview")
	}
}

// ntfonts.Embedded is the FaceSource the typesetter takes; ntfonts cannot
// assert it without importing nttypeset.
func TestEmbeddedIsFaceSource(t *testing.T) {
	var _ nttypeset.FaceSource = ntfonts.Embedded
}
