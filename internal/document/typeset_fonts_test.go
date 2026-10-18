package document

import (
	"context"
	"crypto/sha256"
	"fmt"
	"image"
	"os"
	"strings"
	"testing"
	"testing/fstest"
)

func TestFontLooksAndSubsets(t *testing.T) {
	for _, look := range FontLooks() {
		e, err := newEngine(look.Name, "")
		if err != nil {
			t.Fatal(err)
		}
		for _, face := range e.faces {
			for _, r := range "Café naïve résumé é — “quotes”…" {
				if _, ok := face.NominalGlyph(r); !ok {
					t.Fatalf("%s lacks %U", look.Name, r)
				}
			}
			for _, r := range "漢🙂" {
				if _, ok := face.NominalGlyph(r); ok {
					t.Fatalf("%s ships out-of-scope glyph %U", look.Name, r)
				}
			}
		}
		if _, err := ResolveFont(look.Name, "jetbrains"); err != nil {
			t.Fatal(err)
		}
	}
	for _, v := range [][2]string{{"missing", ""}, {"readable", "other"}} {
		if _, err := ResolveFont(v[0], v[1]); err == nil {
			t.Fatal("invalid font accepted")
		}
	}
}
func TestFontChangesReflowAndPreserveAssets(t *testing.T) {
	l := &Loader{Files: fstest.MapFS{"page.md": {Data: []byte(strings.Repeat("A readable paragraph with café and **bold** words.\n\n", 40))}}}
	defer l.Close()
	q := Request{Path: "page.md", Typeset: true, Font: "readable", TypesetView: &TypesetViewport{Top: 700, Height: 600}}
	r := l.Load(q)
	if r.Err != nil {
		t.Fatal(r.Err)
	}
	prep, eng, layout := l.typesetPrepared, l.typesetEngine, l.direct
	q.Font = "editorial"
	r = l.Load(q)
	if r.Err != nil {
		t.Fatal(r.Err)
	}
	if l.typesetPrepared != prep || l.typesetEngine != eng || l.direct == layout || r.TypesetView.Top == 0 {
		t.Fatal("font switch lost prepared content or did not reflow")
	}
	layout = l.direct
	q.Theme = "nord"
	r = l.Load(q)
	if r.Err != nil || l.direct != layout {
		t.Fatal("color change reshaped", r.Err)
	}
	q.Mono = "jetbrains"
	r = l.Load(q)
	if r.Err != nil || l.typesetEngine.look.Code != "JetBrains Mono" {
		t.Fatal("mono override", r.Err)
	}
	q.Font = "invalid"
	if r = l.Load(q); r.Err == nil {
		t.Fatal("invalid request accepted")
	}
}
func TestFontFallbackMissingAndLigatures(t *testing.T) {
	e, err := newEngine("technical")
	if err != nil {
		t.Fatal(err)
	}
	primary := e.faces[0]
	var candidate rune
	fallback, err := loadFace("A", 0)
	if err != nil {
		t.Fatal(err)
	}
	for r := rune(0x20); r < 0x2070; r++ {
		_, a := primary.NominalGlyph(r)
		_, b := fallback.NominalGlyph(r)
		if !a && b {
			candidate = r
			break
		}
	}
	if candidate == 0 {
		t.Fatal("fixture needs a glyph present only in fallback")
	}
	{
		m := fallbackMap{e, 0}
		if m.ResolveFace(candidate) == primary || !e.fallbackUsed["Literata"] {
			t.Fatal("fallback not used")
		}
	}
	e.shape([]span{{text: "🙂漢"}}, 25)
	if len(e.missing) != 2 || !strings.Contains(e.fontFields()[len(e.fontFields())-1].Value, "U+1F642") {
		t.Fatal("missing glyphs not reported")
	}
	s := e.shape([]span{{text: "ffi", style: 0}}, 25)
	n := 0
	for _, run := range s.runs {
		n += len(run.Glyphs)
	}
	if n != 3 {
		t.Fatal("ligature enabled", n)
	}
}
func TestFontPreviewBoundedAndCancelled(t *testing.T) {
	im, err := FontPreview(context.Background(), []byte("# Sample\n\n```go\nfunc hello() {}\n```"), "nord", "", 480, 320)
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
	if _, err := FontPreview(ctx, nil, "light", "", 480, 320); err == nil {
		t.Fatal("ignored cancellation")
	}
	if _, err := FontPreview(context.Background(), nil, "light", "", 5000, 5000); err == nil {
		t.Fatal("unbounded preview")
	}
}

func TestFontSubsetHashes(t *testing.T) {
	manifest, err := os.ReadFile("fonts/subsets.sha256")
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
		data, err := typesetFonts.ReadFile("fonts/" + fields[1])
		if err != nil {
			t.Fatal(err)
		}
		if fmt.Sprintf("%x", sha256.Sum256(data)) != fields[0] {
			t.Fatalf("subset changed: %s", fields[1])
		}
	}
}
