package document

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"nimbleterminal.dev/ntvideo"
)

// A video is never read into memory, as other files are: it is played from
// its path, by libmpv, and described and drawn by ffprobe and ffmpeg, all
// through ntvideo, whose limits (2 GiB, 4096×4096) bound it instead of
// MaxFileBytes. It is known by its name or by its first bytes.

// videoExtensions name the containers gloss takes as video.
var videoExtensions = []string{".mp4", ".m4v", ".mov", ".webm", ".mkv", ".ogv", ".avi", ".mpg", ".mpeg", ".wmv", ".flv"}

// ErrVideoNative is a video held in memory where nothing can play it: the
// native gallery's embedded files.
var ErrVideoNative = errors.New("a video held in memory plays only in the browser")

// ErrVideoPreview is a video in the browser asked for a picture: its frames
// come from ffmpeg, which the browser does not have.
var ErrVideoPreview = errors.New("no picture of a video in the browser: open it to play")

// videoHeadBytes is how much of a file says whether it is a video.
const videoHeadBytes = 64

// sniffVideo says whether data begins as a video container: MP4 or
// QuickTime (an ftyp box), Matroska or WebM (an EBML header), Ogg with a
// Theora stream (not Ogg audio), AVI, MPEG program stream, ASF (WMV) or FLV.
func sniffVideo(data []byte) bool {
	switch {
	case bytes.HasPrefix(data, []byte("OggS")):
		return bytes.Contains(data, []byte("\x80theora"))
	case len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "AVI ",
		bytes.HasPrefix(data, []byte{0x00, 0x00, 0x01, 0xba}),
		bytes.HasPrefix(data, []byte{0x30, 0x26, 0xb2, 0x75, 0x8e, 0x66, 0xcf, 0x11}),
		bytes.HasPrefix(data, []byte("FLV\x01")):
		return true
	}
	if len(data) >= 12 && string(data[4:8]) == "ftyp" {
		// HEIC and AVIF pictures are ftyp files too; they are images.
		switch string(data[8:12]) {
		case "heic", "heix", "hevc", "hevx", "heim", "heis", "mif1", "msf1", "avif", "avis":
			return false
		}
		return true
	}
	return bytes.HasPrefix(data, []byte{0x1a, 0x45, 0xdf, 0xa3})
}

// isVideoName says whether path is named as a video.
func isVideoName(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	for _, e := range videoExtensions {
		if ext == e {
			return true
		}
	}
	return false
}

// readHead reads the start of a file, as readFileFrom checks it, and no more.
func readHead(files fs.FS, path string, n int) ([]byte, error) {
	var f fs.File
	var err error
	if files == nil {
		pre, statErr := os.Stat(path)
		if statErr != nil {
			return nil, statErr
		}
		if pre.IsDir() {
			return nil, ErrDirectory
		}
		if !pre.Mode().IsRegular() {
			return nil, ErrNotRegular
		}
		f, err = os.Open(path)
	} else {
		f, err = files.Open(path)
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	head := make([]byte, n)
	got, err := io.ReadFull(f, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return nil, err
	}
	return head[:got], nil
}

// isVideoFile says whether the file at path is a video, by --type, its name,
// or its first bytes; the bytes decide when the name says nothing.
func isVideoFile(files fs.FS, path, forced string) bool {
	if forced != "" {
		return forced == "video"
	}
	head, err := readHead(files, path, videoHeadBytes)
	if err != nil {
		return false
	}
	if sniffVideo(head) {
		return true
	}
	// A name is believed unless the bytes plainly say it is a picture.
	return isVideoName(path) && !pictureMagic(head)
}

// pictureMagic says whether data begins as a picture gloss opens.
func pictureMagic(data []byte) bool {
	for _, magic := range []string{"\x89PNG", "\xff\xd8\xff", "GIF8", "BM", "II*\x00", "MM\x00*", "\xff\x0a", "\x00\x00\x00\x0cJXL "} {
		if bytes.HasPrefix(data, []byte(magic)) {
			return true
		}
	}
	return len(data) >= 12 && (string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP" || string(data[4:8]) == "ftyp")
}

// loadVideo describes a video and draws its first frame, without reading it.
// The first frame stands for it in previews, thumbnails, exports and --print,
// and before playback; the viewer plays the file itself.
func (l *Loader) loadVideo(q Request) (out Result) {
	out.Generation, out.Kind, out.Page, out.Pages = q.Generation, "video", 1, 1
	if l.Files != nil {
		return l.loadMemoryVideo(q)
	}
	info, statErr := os.Stat(q.Path)
	if statErr != nil {
		out.Err = statErr
		return out
	}
	file := fileFields(nil, q.Path, int(min(info.Size(), int64(^uint(0)>>1))), q.Stdin)
	// ntvideo gives each tool 15 seconds of its own.
	ctx := q.Context
	if ctx == nil {
		ctx = context.Background()
	}
	media, probeErr := ntvideo.Probe(ctx, q.Path, ntvideo.HeadlessOptions{})
	subs := findSubtitles(nil, q.Path)
	out.Video = &Video{Path: q.Path, Info: media, ProbeErr: probeErr, Subtitles: subs}
	out.Info = append(file, describe(func() []Field { return append(videoFields(q.Path, media, probeErr), subtitleFields(subs)...) })...)
	if q.Describe {
		return out
	}
	frame, err := ntvideo.FrameAt(ctx, q.Path, max(0, q.At), ntvideo.HeadlessOptions{})
	if err != nil {
		out.Video.FrameErr = err
		// The viewer can still play it; anything else has only the picture.
		if !q.Animate || q.Preview {
			out.Err = fmt.Errorf("video: %w", ffmpegHint(err, "draw its frames"))
		}
		return out
	}
	out.Image = frame
	return out
}

// loadMemoryVideo is a video the page holds, in the browser: the browser
// plays it from a blob: URL, and nothing describes it or draws its frames.
func (l *Loader) loadMemoryVideo(q Request) (out Result) {
	out.Generation, out.Kind, out.Page, out.Pages = q.Generation, "video", 1, 1
	size := 0
	if info, err := fs.Stat(l.Files, q.Path); err == nil {
		size = int(min(info.Size(), int64(^uint(0)>>1)))
	}
	subs := findSubtitles(l.Files, q.Path)
	out.Info = append(fileFields(l.Files, q.Path, size, q.Stdin), Section("Video", append([]Field{{"Format", videoFormat(q.Path)}}, subtitleFields(subs)...)...)...)
	if q.Describe {
		return out
	}
	if !q.Animate || q.Preview {
		out.Err = ErrVideoPreview
		return out
	}
	url, revoke, err := videoURL(l.Files, q.Path)
	if err != nil {
		out.Err = err
		return out
	}
	out.Video = &Video{Path: q.Path, URL: url, Revoke: revoke}
	// The browser reads WebVTT: an SRT is converted, and ASS/SSA left out.
	for _, sub := range subs {
		data, err := readFileFrom(l.Files, sub.Path)
		switch ext := strings.ToLower(filepath.Ext(sub.Path)); {
		case err != nil || ext == ".ass" || ext == ".ssa":
			continue
		case ext == ".srt":
			data = srtToVTT(data)
		}
		url, free := blobURL(data, "text/vtt")
		sub.URL = url
		out.Video.Subtitles = append(out.Video.Subtitles, sub)
		video := out.Video.Revoke
		out.Video.Revoke = func() { video(); free() }
	}
	return out
}

// Video is a video file, played from Path, or in the browser from URL.
type Video struct {
	Path   string
	URL    string // A blob: URL the browser plays; empty natively.
	Revoke func() // Frees URL, and the subtitles', once the video is closed; nil natively.
	// The subtitle files beside it, the first shown first.
	Subtitles []Subtitle
	Info      ntvideo.MediaInfo
	ProbeErr  error // Why there are no details: usually no ffprobe.
	FrameErr  error // Why there is no first frame: usually no ffmpeg.
}

// ffmpegHint says what to install, to do what, when ffmpeg or ffprobe was
// not found; any other failure is told as it is.
func ffmpegHint(err error, what string) error {
	if errors.Is(err, exec.ErrNotFound) {
		return fmt.Errorf("install ffmpeg to %s", what)
	}
	return err
}

// videoFormat names a video's container by its name.
func videoFormat(path string) string {
	if format := strings.ToUpper(strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")); format != "" {
		return format
	}
	return "video"
}

func videoFields(path string, info ntvideo.MediaInfo, probeErr error) []Field {
	format := videoFormat(path)
	if probeErr != nil {
		return Section("Video", Field{"Format", format}, Field{"Details", ffmpegHint(probeErr, "read its details").Error()})
	}
	fields := []Field{{"Format", format}}
	if info.Duration > 0 {
		fields = append(fields, Field{"Duration", info.Duration.Round(time.Second / 10).String()})
	}
	if info.Width > 0 && info.Height > 0 {
		fields = append(fields, Field{"Size", fmt.Sprintf("%d × %d", info.Width, info.Height)})
	}
	if info.VideoCodec != "" {
		fields = append(fields, Field{"Video", info.VideoCodec})
	}
	audio := "none"
	if info.HasAudio {
		audio = info.AudioCodec
	}
	fields = append(fields, Field{"Audio", audio})
	return Section("Video", fields...)
}
