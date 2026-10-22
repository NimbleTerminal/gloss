package document

import (
	"errors"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"nimbleterminal.dev/ntvideo"
)

func TestSniffVideo(t *testing.T) {
	for name, tc := range map[string]struct {
		head string
		want bool
	}{
		"mp4":       {"\x00\x00\x00\x20ftypisom\x00\x00\x02\x00", true},
		"quicktime": {"\x00\x00\x00\x14ftypqt  \x00\x00\x00\x00", true},
		"webm":      {"\x1a\x45\xdf\xa3\x9f\x42\x86\x81", true},
		"ogv":       {"OggS\x00\x02\x00\x00\x00\x00\x00\x00\x00\x00\x6a\x1e\x09\x27\x00\x00\x00\x00\x86\xdf\x98\x05\x01\x2a\x80theora", true},
		"ogg audio": {"OggS\x00\x02\x00\x00\x00\x00\x00\x00\x00\x00\x6a\x1e\x09\x27\x00\x00\x00\x00\x86\xdf\x98\x05\x01\x1e\x01vorbis", false},
		"avi":       {"RIFF\x00\x10\x00\x00AVI LIST", true},
		"wav":       {"RIFF\x00\x10\x00\x00WAVEfmt ", false},
		"mpeg":      {"\x00\x00\x01\xba\x44\x00", true},
		"wmv":       {"\x30\x26\xb2\x75\x8e\x66\xcf\x11\xa6\xd9", true},
		"flv":       {"FLV\x01\x05\x00\x00\x00\x09", true},
		"heic":      {"\x00\x00\x00\x18ftypheic\x00\x00\x00\x00", false},
		"avif":      {"\x00\x00\x00\x1cftypavif\x00\x00\x00\x00", false},
		"png":       {"\x89PNG\r\n\x1a\n\x00\x00\x00\x0d", false},
		"short":     {"\x00\x00", false},
	} {
		if got := sniffVideo([]byte(tc.head)); got != tc.want {
			t.Errorf("%s: sniffVideo = %v, want %v", name, got, tc.want)
		}
	}
}

func TestIsVideoFile(t *testing.T) {
	dir := t.TempDir()
	write := func(name, data string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	mp4 := "\x00\x00\x00\x20ftypisom\x00\x00\x02\x00"
	png := "\x89PNG\r\n\x1a\n\x00\x00\x00\x0dIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x02\x00\x00\x00"
	for _, tc := range []struct {
		path, forced string
		want         bool
	}{
		{write("clip.mp4", mp4), "", true},
		{write("clip", mp4), "", true},                // By its bytes.
		{write("clip.mov", "not a header"), "", true}, // By its name.
		{write("still.mp4", png), "", false},          // A picture misnamed.
		{write("clip.webm", mp4), "image", false},     // --type says otherwise.
		{write("notes.txt", "hello"), "video", true},  // --type says so.
		{write("notes2.txt", "hello"), "", false},
	} {
		if got := isVideoFile(nil, tc.path, tc.forced); got != tc.want {
			t.Errorf("%s (--type %q): %v, want %v", filepath.Base(tc.path), tc.forced, got, tc.want)
		}
	}
	if isVideoFile(nil, dir, "") {
		t.Error("a folder is a video")
	}
}

// A video over the 128 MiB that other files may be is still a video, and
// --accept knows it from its first bytes.
func TestLargeVideoIsNotRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "big.mp4")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("\x00\x00\x00\x20ftypisom\x00\x00\x02\x00"); err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(MaxFileBytes + 1); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if !isVideoFile(nil, path, "") {
		t.Fatal("a large video is not known as one")
	}
	accept, _ := ParseAccept("video/*")
	if err := accept.CheckFile(nil, path); err != nil {
		t.Fatalf("--accept video/*: %v", err)
	}
	accept, _ = ParseAccept("image/*")
	if err := accept.CheckFile(nil, path); err == nil || !strings.Contains(err.Error(), "received video") {
		t.Fatalf("--accept image/*: %v", err)
	}
}

func TestLoadVideo(t *testing.T) {
	for _, tool := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s is not installed", tool)
		}
	}
	path := filepath.Join(t.TempDir(), "clip.mp4")
	out, err := exec.Command("ffmpeg", "-loglevel", "error", "-f", "lavfi", "-i", "testsrc2=size=64x48:rate=10", "-t", "1", "-pix_fmt", "yuv420p", path).CombinedOutput()
	if err != nil {
		t.Skipf("ffmpeg could not make a clip: %v %s", err, out)
	}
	l := &Loader{}
	defer l.Close()
	r := l.Load(Request{Path: path, Generation: 1})
	if r.Err != nil || r.Kind != "video" || r.Video == nil || r.Image == nil {
		t.Fatalf("kind=%q video=%v image=%v err=%v", r.Kind, r.Video, r.Image != nil, r.Err)
	}
	if b := r.Image.Bounds(); b.Dx() != 64 || b.Dy() != 48 {
		t.Fatalf("first frame %v", b)
	}
	if r.Video.Info.Width != 64 || r.Video.Info.Height != 48 || r.Video.Info.HasAudio {
		t.Fatalf("info %+v", r.Video.Info)
	}
	info := r.Info
	r = l.Load(Request{Path: path, Generation: 2, Describe: true})
	if r.Err != nil || r.Image != nil || len(r.Info) != len(info) {
		t.Fatalf("describe: image=%v fields=%d/%d err=%v", r.Image != nil, len(r.Info), len(info), r.Err)
	}
}

// Without ffmpeg a video is still described, and says what to install;
// a headless load, which needs its first frame, fails saying so.
func TestVideoWithoutFFmpeg(t *testing.T) {
	path := filepath.Join(t.TempDir(), "clip.mp4")
	if err := os.WriteFile(path, []byte("\x00\x00\x00\x20ftypisom\x00\x00\x02\x00"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", t.TempDir())
	l := &Loader{}
	defer l.Close()
	r := l.Load(Request{Path: path, Generation: 1, Describe: true})
	if r.Err != nil || r.Kind != "video" || !strings.Contains(fieldsText(r.Info), "install ffmpeg to read its details") {
		t.Fatalf("describe: err=%v info=%v", r.Err, r.Info)
	}
	r = l.Load(Request{Path: path, Generation: 2})
	if r.Err == nil || !strings.Contains(r.Err.Error(), "install ffmpeg to draw its frames") {
		t.Fatalf("headless: %v", r.Err)
	}
	r = l.Load(Request{Path: path, Generation: 3, Animate: true})
	if r.Err != nil || r.Video == nil {
		t.Fatalf("viewer: the video should still play: %v", r.Err)
	}
}

func fieldsText(fields []Field) string {
	var b strings.Builder
	for _, f := range fields {
		b.WriteString(f.Label + " " + f.Value + "\n")
	}
	return b.String()
}

// A video held in memory, as the browser's files are, is described without
// ffmpeg and has no picture; natively it cannot be played from memory.
func TestMemoryVideo(t *testing.T) {
	files := fstest.MapFS{"clip.mp4": {Data: []byte("\x00\x00\x00\x20ftypisom\x00\x00\x02\x00")}}
	l := &Loader{Files: files}
	defer l.Close()
	r := l.Load(Request{Path: "clip.mp4", Generation: 1, Describe: true})
	if r.Err != nil || r.Kind != "video" || !strings.Contains(fieldsText(r.Info), "Format MP4") {
		t.Fatalf("describe: err=%v info=%v", r.Err, r.Info)
	}
	if r = l.Load(Request{Path: "clip.mp4", Generation: 2, Preview: true}); !errors.Is(r.Err, ErrVideoPreview) {
		t.Fatalf("preview: %v", r.Err)
	}
	if r = l.Load(Request{Path: "clip.mp4", Generation: 3, Animate: true}); !errors.Is(r.Err, ErrVideoNative) || r.Video != nil {
		t.Fatalf("viewer: err=%v video=%v", r.Err, r.Video)
	}
}

func TestSpread(t *testing.T) {
	got := Spread(12*time.Second, 4)
	want := []time.Duration{1500 * time.Millisecond, 4500 * time.Millisecond, 7500 * time.Millisecond, 10500 * time.Millisecond}
	if !slices.Equal(got, want) {
		t.Fatalf("Spread = %v, want %v", got, want)
	}
}

func TestTranscript(t *testing.T) {
	cues := []ntvideo.CaptionCue{
		{Start: time.Second, End: 4 * time.Second, Text: "Hello there.\nSecond line"},
		{Start: 3723*time.Second + 450*time.Millisecond, End: 3725 * time.Second, Text: "Later."},
	}
	want := "[0:01.0–0:04.0] Hello there. / Second line\n[1:02:03.5–1:02:05.0] Later.\n"
	if got := Transcript(cues); got != want {
		t.Fatalf("Transcript =\n%s\nwant\n%s", got, want)
	}
	if got := captionAt(cues, 2*time.Second); got != "Hello there.\nSecond line" {
		t.Fatalf("captionAt(2s) = %q", got)
	}
	if got := captionAt(cues, 4*time.Second); got != "" {
		t.Fatalf("captionAt(4s), as the cue ends = %q", got)
	}
}

// Frames of several shapes share a sheet, each at its own aspect, and none
// is enlarged past its size.
func TestFrameSheet(t *testing.T) {
	wide := image.NewRGBA(image.Rect(0, 0, 160, 90))
	tall := image.NewRGBA(image.Rect(0, 0, 90, 160))
	sheet, err := frameSheet([]Frame{{At: 0, Image: wide}, {At: time.Second, Image: tall}, {At: 2 * time.Second, Image: wide}}, 4096)
	if err != nil {
		t.Fatal(err)
	}
	if b := sheet.Bounds(); b.Dx() > 2*160+16 || b.Dy() > 2*160+16 {
		t.Fatalf("sheet %v: frames were enlarged", b)
	}
	sheet, err = frameSheet([]Frame{{Image: wide}, {Image: wide}, {Image: wide}, {Image: wide}}, 100)
	if err != nil {
		t.Fatal(err)
	}
	if b := sheet.Bounds(); b.Dx() > 100 || b.Dy() > 100 {
		t.Fatalf("sheet %v is over its edge", b)
	}
}

// With ffmpeg, a video gives its frames at the times asked, or spread through
// it, with the subtitles shown then, and its subtitles as text.
func TestVideoFramesAndSubtitles(t *testing.T) {
	for _, tool := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s is not installed", tool)
		}
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "clip.mp4")
	out, err := exec.Command("ffmpeg", "-loglevel", "error", "-f", "lavfi", "-i", "testsrc2=size=64x48:rate=10", "-t", "4", "-pix_fmt", "yuv420p", path).CombinedOutput()
	if err != nil {
		t.Skipf("ffmpeg could not make a clip: %v %s", err, out)
	}
	srt := "1\n00:00:01,000 --> 00:00:02,000\nHello.\n"
	if err := os.WriteFile(filepath.Join(dir, "clip.en.srt"), []byte(srt), 0o600); err != nil {
		t.Fatal(err)
	}
	l := &Loader{}
	defer l.Close()
	r := l.Load(Request{Path: path, Generation: 1, Times: []time.Duration{1500 * time.Millisecond, 3 * time.Second}})
	if r.Err != nil || len(r.Frames) != 2 || r.Image != r.Frames[0].Image {
		t.Fatalf("--at: frames=%d err=%v", len(r.Frames), r.Err)
	}
	if r.Frames[0].Caption != "Hello." || r.Frames[1].Caption != "" {
		t.Fatalf("captions %q, %q", r.Frames[0].Caption, r.Frames[1].Caption)
	}
	r = l.Load(Request{Path: path, Generation: 2, FrameCount: 4})
	if r.Err != nil || len(r.Frames) != 4 || r.Frames[0].At != 500*time.Millisecond {
		t.Fatalf("--frames: %d frames, first at %v, err=%v", len(r.Frames), r.Frames, r.Err)
	}
	if r = l.Load(Request{Path: path, Generation: 3, Times: []time.Duration{9 * time.Second}}); r.Err == nil || !strings.Contains(r.Err.Error(), "past the end") {
		t.Fatalf("past the end: %v", r.Err)
	}
	r = l.Load(Request{Path: path, Generation: 4, TextOnly: true})
	if r.Err != nil || r.Text != "[0:01.0–0:02.0] Hello.\n" || r.Video.CueSource != "clip.en.srt" {
		t.Fatalf("text %q from %q: %v", r.Text, r.Video.CueSource, r.Err)
	}
	os.Remove(filepath.Join(dir, "clip.en.srt"))
	if r = l.Load(Request{Path: path, Generation: 5, TextOnly: true}); r.Err == nil || !strings.Contains(r.Err.Error(), "no subtitles") {
		t.Fatalf("no subtitles: %v", r.Err)
	}
}
