package document

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func TestFindSubtitles(t *testing.T) {
	dir := t.TempDir()
	for name, size := range map[string]int{
		"clip.mp4": 16, "clip.srt": 10, "clip.fr.srt": 10, "clip.en.vtt": 10, "clip.de.ass": 10,
		"clip.txt": 10, "clips.srt": 10, "other.srt": 10, "clip.big.srt": maxSubtitleBytes + 1,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), make([]byte, size), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "clip.dir.srt"), 0o700); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, s := range findSubtitles(nil, filepath.Join(dir, "clip.mp4")) {
		got = append(got, filepath.Base(s.Path)+"="+s.Label())
	}
	// The one with no language first; not another video's, nor too large,
	// nor a folder, nor other kinds of file.
	if want := "clip.srt=SRT clip.de.ass=de clip.en.vtt=en clip.fr.srt=fr"; strings.Join(got, " ") != want {
		t.Fatalf("got %q, want %q", strings.Join(got, " "), want)
	}
	files := fstest.MapFS{"clip.mp4": {Data: []byte{0}}, "clip.en.srt": {Data: []byte("1")}}
	if subs := findSubtitles(files, "clip.mp4"); len(subs) != 1 || subs[0].Path != "clip.en.srt" || subs[0].Language != "en" {
		t.Fatalf("in memory: %+v", subs)
	}
}

func TestSRTToVTT(t *testing.T) {
	srt := "\xef\xbb\xbf1\r\n00:00:01,000 --> 00:00:02,500\r\nHello, world\r\n\r\n2\r\n00:00:03,000 --> 00:00:04,000\r\nAgain\r\n"
	want := "WEBVTT\n\n1\n00:00:01.000 --> 00:00:02.500\nHello, world\n\n2\n00:00:03.000 --> 00:00:04.000\nAgain\n"
	if got := string(srtToVTT([]byte(srt))); got != want {
		t.Fatalf("got %q", got)
	}
}

func TestVideoInfoNamesSubtitles(t *testing.T) {
	files := fstest.MapFS{"clip.mp4": {Data: []byte("\x00\x00\x00\x20ftypisom\x00\x00\x02\x00")}, "clip.en.srt": {Data: []byte("1")}, "clip.vtt": {Data: []byte("WEBVTT")}}
	l := &Loader{Files: files}
	defer l.Close()
	r := l.Load(Request{Path: "clip.mp4", Generation: 1, Describe: true})
	if r.Err != nil || !strings.Contains(fieldsText(r.Info), "Subtitles clip.vtt, clip.en.srt") {
		t.Fatalf("err=%v info=%v", r.Err, r.Info)
	}
}
