package document

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nimbleterminal.dev/nttypeset"
)

func testInput(t *testing.T, source string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "input.md")
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

// The fixture's walk makes every kind of block, with its local image and a
// placeholder for the missing one.
func TestFixtureBlocks(t *testing.T) {
	doc, images, err := prepareForTest("../../examples/typeset.md")
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[nttypeset.Kind]bool{}
	for _, b := range doc.Blocks {
		kinds[b.Kind] = true
	}
	for _, kind := range []nttypeset.Kind{nttypeset.KindHeading, nttypeset.KindParagraph, nttypeset.KindQuote, nttypeset.KindCode, nttypeset.KindTable, nttypeset.KindImage} {
		if !kinds[kind] {
			t.Errorf("missing kind %d", kind)
		}
	}
	if images["landscape.png"] == nil || images["missing-typeset-image.png"] != nil {
		t.Fatal("local image/placeholder")
	}
}

// The walk gives up a document with too many blocks with an error, not a panic.
func TestBuildBlocksLimit(t *testing.T) {
	data := []byte(strings.Repeat("---\n\n", 32800))
	root, err := readMarkdownTree(data)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := buildBlocks(context.Background(), root, data, nil, nil); err == nil || err.Error() != "typeset document exceeds 32,768 blocks" {
		t.Fatalf("block limit: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := buildBlocks(ctx, root, data, nil, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled walk: %v", err)
	}
}
func TestPreparationBudgetsAndOfflineImages(t *testing.T) {
	cases := []struct{ name, source string }{
		{"source", strings.Repeat("a", (2<<20)+1)},
		{"utf8", string([]byte{0xff})},
		{"refs", strings.Repeat("![x](https://example.invalid/x.png)\n\n", 33)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := prepareForTest(testInput(t, tc.source)); err == nil {
				t.Fatal("accepted over-budget input")
			}
		})
	}
	doc, images, err := prepareForTest(testInput(t, strings.Repeat("![x](https://example.invalid/x.png)\n\n", 32)))
	if err != nil || len(images) != 0 || len(doc.Blocks) != 32 {
		t.Fatalf("remote references must be placeholders: %v", err)
	}
	path := testInput(t, "![huge](huge.png)")
	var b bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	if err = png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	data := b.Bytes()
	binary.BigEndian.PutUint32(data[16:20], 8192)
	binary.BigEndian.PutUint32(data[20:24], 8192)
	binary.BigEndian.PutUint32(data[29:33], crc32.ChecksumIEEE(data[12:29]))
	if err = os.WriteFile(filepath.Join(filepath.Dir(path), "huge.png"), data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err = prepareForTest(path); err == nil || !strings.Contains(err.Error(), "16 Mi") {
		t.Fatalf("pixel budget not rejected before decode: %v", err)
	}
}

// prepareForTest prepares and walks a Markdown file as Load does, without
// animation.
func prepareForTest(path string) (*nttypeset.Document, map[string]image.Image, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	_, root, images, _, err := prepareTypeset(Request{Path: path}, data, nil)
	if err != nil {
		return nil, nil, err
	}
	doc, err := buildBlocks(context.Background(), root, data, images, nil)
	return doc, images, err
}

// testLayout lays out a whole document at width with gloss's fonts.
func testLayout(t *testing.T, doc *nttypeset.Document, width int) *nttypeset.Layout {
	t.Helper()
	e, err := newTypesetEngine("", "")
	if err != nil {
		t.Fatal(err)
	}
	l, err := e.Layout(doc, width, nttypeset.LayoutOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return l
}
