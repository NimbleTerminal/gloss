package document

import (
	"image"
	"strings"
	"testing"
	"testing/fstest"

	"nimbleterminal.dev/gloss/examples"
)

func exampleWebP(t *testing.T) []byte {
	t.Helper()
	data, err := examples.Files.ReadFile("motion.webp")
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestAnimatedWebPLoaderModes(t *testing.T) {
	loader := &Loader{Files: fstest.MapFS{"motion.webp": {Data: exampleWebP(t)}}}
	defer loader.Close()
	for _, q := range []Request{{}, {Animate: true}, {Animate: true, Preview: true}} {
		q.Path = "motion.webp"
		r := loader.Load(q)
		if r.Err != nil {
			t.Fatalf("%+v: %v", q, r.Err)
		}
		if (r.Animation != nil) != (q.Animate && !q.Preview) {
			t.Fatalf("wrong playback mode for %+v", q)
		}
		if r.Image.Bounds() != image.Rect(0, 0, 192, 112) {
			t.Fatalf("canvas %v", r.Image.Bounds())
		}
		if r.Animation != nil {
			if r.Animation.Frames() != 32 {
				t.Fatalf("%d frames", r.Animation.Frames())
			}
			var frames string
			for _, f := range r.Info {
				if f.Label == "Frames" {
					frames = f.Value
				}
			}
			if frames != "32" {
				t.Fatalf("info frames %q", frames)
			}
		}
	}
}

func TestMarkdownAnimatedWebP(t *testing.T) {
	landscape, err := examples.Files.ReadFile("landscape.png")
	if err != nil {
		t.Fatal(err)
	}
	files := fstest.MapFS{
		"doc.md":        {Data: []byte("![motion](motion.webp)\n\n![still](landscape.png)\n")},
		"motion.webp":   {Data: exampleWebP(t)},
		"landscape.png": {Data: landscape},
	}
	for _, q := range []Request{{Animate: true}, {}} {
		l := &Loader{Files: files}
		q.Path = "doc.md"
		r := l.Load(q)
		l.Close()
		if r.Err != nil {
			t.Fatal(r.Err)
		}
		md := r.Markdown
		if md.Animations != 1 || md.Animated != 1 || md.AnimationOverBudget != 0 {
			t.Fatalf("counts: %+v", md)
		}
		if (md.Images[0].Animation != nil) != q.Animate {
			t.Fatal("playback option ignored")
		}
		if md.Images[1].Animation != nil || md.Images[1].Image == nil {
			t.Fatal("still image mistaken for an animation")
		}
		var animated string
		for _, f := range r.Info {
			if f.Label == "Animated" {
				animated = f.Value
			}
		}
		if !strings.Contains(animated, "1 of 1 animations") {
			t.Fatalf("info animated %q", animated)
		}
	}
}
