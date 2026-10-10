package document

import (
	"context"
	"fmt"
	"image"
	"testing"
	"testing/fstest"

	"github.com/NimbleTerminal/gloss/internal/animate"
)

func TestMarkdownAnimationModesBudgetAndDuplicates(t *testing.T) {
	files := fstest.MapFS{}
	source := ""
	for i := range 9 {
		name := fmt.Sprintf("%d.gif", i)
		files[name] = &fstest.MapFile{Data: gifTestData(t, 0)}
		source += fmt.Sprintf("![gif](%s)\n\n", name)
	}
	source += "![duplicate](0.gif)\n"
	files["test.md"] = &fstest.MapFile{Data: []byte(source)}
	for _, mode := range []string{"text", "typeset"} {
		for _, q := range []Request{{Animate: true}, {}, {Animate: true, Preview: true}} {
			l := &Loader{Files: files}
			q.Path = "test.md"
			q.MarkdownMode = mode
			r := l.Load(q)
			l.Close()
			if r.Err != nil {
				t.Fatal(r.Err)
			}
			md := r.Markdown
			if mode == "typeset" {
				md = r.Typeset
			}
			if md.GIFs != 9 || md.Animated != 8 || md.AnimationOverBudget != 1 {
				t.Fatalf("%s counts: %+v", mode, md)
			}
			if mode == "text" {
				if (md.Images[0].Animation != nil) != (q.Animate && !q.Preview) {
					t.Fatal("playback option ignored")
				}
				if md.Images[8].Animation != nil || md.Images[8].Image == nil {
					t.Fatal("over-budget GIF not a still")
				}
				if md.Images[9].Animation != md.Images[0].Animation {
					t.Fatal("duplicate decoded separately")
				}
			} else if (r.Playback != nil) != (q.Animate && !q.Preview) {
				t.Fatal("typeset playback option ignored")
			}
		}
	}
}
func TestMarkdownAnimationAggregateBytesAndCancellation(t *testing.T) {
	data := gifTestData(t, 0)
	stats, err := animate.PreflightGIF(data)
	if err != nil {
		t.Fatal(err)
	}
	b := markdownAnimations{bytes: MaxMarkdownAnimationBytes - stats.Bytes}
	_, p, err := b.decode(context.Background(), data, true)
	if err != nil || p == nil {
		t.Fatal("exact byte boundary rejected", err)
	}
	img, p, err := b.decode(context.Background(), data, true)
	if err != nil || p != nil || img == nil || b.over != 1 {
		t.Fatal("aggregate byte limit failed", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := b.decode(ctx, data, true); err != context.Canceled {
		t.Fatal("cancelled decode", err)
	}
}
func TestTypesetAnimatedRegionsAndReflow(t *testing.T) {
	l := &Loader{Files: fstest.MapFS{"doc.md": {Data: []byte("# Motion\n\n![gif](motion.gif)\n\nAfter.")}, "motion.gif": {Data: gifTestData(t, 0)}}}
	defer l.Close()
	q := Request{Path: "doc.md", Typeset: true, Animate: true, TypesetView: &TypesetViewport{Height: 500}}
	r := l.Load(q)
	if r.Err != nil || len(r.Animated) != 1 || r.Playback == nil {
		t.Fatalf("missing region: %v, %d", r.Err, len(r.Animated))
	}
	rect := r.Animated[0].Rect
	if !rect.Overlaps(r.Image.Bounds()) {
		t.Fatal("region outside viewport")
	}
	before := r.Playback.Image()
	next := r.Playback.Clone().(*animate.CompositeSource)
	after, _, more := next.Next()
	if !more {
		t.Fatal("no composite frame")
	}
	// The second frame changes the center pixel from red to green.
	at := image.Pt(rect.Min.X+rect.Dx()/2, rect.Min.Y+rect.Dy()/2)
	if before.At(at.X, at.Y) == after.At(at.X, at.Y) {
		t.Fatal("region did not animate")
	}
	if before.At(0, 0) != after.At(0, 0) {
		t.Fatal("outside pixels changed")
	}
	q.Playback = next
	q.TypesetWidth = 300
	resized := l.Load(q)
	if resized.Err != nil || len(resized.Animated) != 1 {
		t.Fatal(resized.Err)
	}
	rr := resized.Animated[0].Rect
	at = image.Pt(rr.Min.X+rr.Dx()/2, rr.Min.Y+rr.Dy()/2)
	red, green, _, _ := resized.Image.At(at.X, at.Y).RGBA()
	if green <= red {
		t.Fatal("reflow restarted at red first frame")
	}
	q.Animate = false
	q.Playback = nil
	still := l.Load(q)
	if still.Err != nil || still.Playback != nil || len(still.Animated) != 0 {
		t.Fatal("headless path animated", still.Err)
	}
}
