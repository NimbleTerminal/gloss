package document

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"path/filepath"
	"strings"
	"time"

	xdraw "golang.org/x/image/draw"
	"nimbleterminal.dev/ntvideo"
)

// MaxFrames is how many frames of a video one request may draw: ntvideo's
// batch, made by one ffmpeg.
const MaxFrames = 16

// maxFramePixels is what ntvideo allows a batch of frames, all together.
const maxFramePixels = 16 << 20

// Frame is a moment of a video, drawn, with the subtitle shown then.
type Frame struct {
	At      time.Duration
	Image   image.Image
	Caption string // The subtitle text at that moment; empty when none.
}

// Spread gives n moments of a video spread evenly through it: the middle
// of each of n equal parts, so neither the first frame, often black, nor
// the very end.
func Spread(duration time.Duration, n int) []time.Duration {
	times := make([]time.Duration, n)
	for i := range times {
		times[i] = time.Duration((float64(i) + 0.5) / float64(n) * float64(duration))
	}
	return times
}

// loadFrames draws the frames a request asks for, by time or by count, in
// one batch, with the subtitles shown at each.
func loadFrames(ctx context.Context, q Request, media ntvideo.MediaInfo, probeErr error, subs []Subtitle) ([]Frame, error) {
	times := q.Times
	if q.FrameCount > 0 {
		if probeErr != nil {
			return nil, ffmpegHint(probeErr, "find how long it is")
		}
		if media.Duration <= 0 {
			return nil, errors.New("its length is unknown, so frames cannot be spread through it: name their times with --at")
		}
		times = Spread(media.Duration, q.FrameCount)
	}
	if len(times) > MaxFrames {
		return nil, fmt.Errorf("at most %d frames at once", MaxFrames)
	}
	for _, t := range times {
		if probeErr == nil && media.Duration > 0 && t >= media.Duration {
			return nil, fmt.Errorf("%s is past the end, at %s", Clock(t), Clock(media.Duration))
		}
	}
	// The frames come at the export's size, or less, as the batch allows.
	edge := q.MaxEdge
	if edge <= 0 {
		edge = 1568
	}
	edge = min(edge, int(math.Sqrt(float64(maxFramePixels/len(times)))))
	images, err := ntvideo.Frames(ctx, q.Path, times, ntvideo.HeadlessOptions{MaxFrameWidth: edge, MaxFrameHeight: edge})
	if err != nil {
		return nil, ffmpegHint(err, "draw its frames")
	}
	// Subtitles are a help, not a need: without them the frames still stand.
	cues, _, _ := videoCues(ctx, q.Path, media, subs)
	frames := make([]Frame, len(times))
	for i, t := range times {
		frames[i] = Frame{At: t, Image: images[i], Caption: captionAt(cues, t)}
	}
	return frames, nil
}

// captionAt is the text of the cues shown at t, one to a line.
func captionAt(cues []ntvideo.CaptionCue, t time.Duration) string {
	var lines []string
	for _, c := range cues {
		if c.Start <= t && t < c.End {
			lines = append(lines, c.Text)
		}
	}
	return strings.Join(lines, "\n")
}

// videoCues reads a video's subtitles as timed text: the first SRT or
// WebVTT file beside it, else the first of its own subtitle streams that
// is text, the one marked default first. It says where they came from.
func videoCues(ctx context.Context, path string, media ntvideo.MediaInfo, subs []Subtitle) ([]ntvideo.CaptionCue, string, error) {
	for _, sub := range subs {
		switch strings.ToLower(filepath.Ext(sub.Path)) {
		case ".srt", ".vtt":
			cues, err := ntvideo.Cues(ctx, sub.Path, ntvideo.HeadlessOptions{})
			if err != nil {
				return nil, "", fmt.Errorf("%s: %w", filepath.Base(sub.Path), err)
			}
			return cues, filepath.Base(sub.Path), nil
		}
	}
	streams := append([]ntvideo.SubtitleStream(nil), media.Subtitles...)
	for i, s := range streams {
		if s.Default && i > 0 {
			streams[0], streams[i] = streams[i], streams[0]
			break
		}
	}
	var last error
	for _, s := range streams {
		cues, err := ntvideo.Cues(ctx, path, ntvideo.HeadlessOptions{SubtitleStream: s.Index})
		if errors.Is(err, ntvideo.ErrSubtitleNotText) {
			last = errors.New("its subtitles are pictures, not text")
			continue
		}
		if err != nil {
			return nil, "", ffmpegHint(err, "read its subtitles")
		}
		return cues, streamName(s), nil
	}
	if last != nil {
		return nil, "", last
	}
	return nil, "", errors.New("no subtitles: no .srt or .vtt file named after it beside it, and no text subtitles in it")
}

// streamName names a subtitle stream in a video, for a person.
func streamName(s ntvideo.SubtitleStream) string {
	name := fmt.Sprintf("subtitle stream %d", s.Index)
	var about []string
	// An MP4's stream is titled after its handler, by ffmpeg, unless named.
	for _, a := range []string{s.Language, strings.TrimSuffix(s.Title, "SubtitleHandler")} {
		if a != "" {
			about = append(about, a)
		}
	}
	if len(about) > 0 {
		name += " (" + strings.Join(about, ", ") + ")"
	}
	return name
}

// Transcript writes cues as lines of text, each after its times:
// [0:01.2–0:03.4] Hello. A cue of several lines is joined by " / ".
func Transcript(cues []ntvideo.CaptionCue) string {
	var b strings.Builder
	for _, c := range cues {
		text := strings.Join(strings.Fields(strings.ReplaceAll(c.Text, "\n", " / ")), " ")
		fmt.Fprintf(&b, "[%s–%s] %s\n", fineClock(c.Start), fineClock(c.End), text)
	}
	return b.String()
}

// Clock writes a time as m:ss, or h:mm:ss, to the second.
func Clock(d time.Duration) string {
	d = d.Round(time.Second)
	h, m, s := int(d.Hours()), int(d.Minutes())%60, int(d.Seconds())%60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

// fineClock writes a time as Clock does, to a tenth of a second.
func fineClock(d time.Duration) string {
	d = d.Round(time.Second / 10)
	tenths := int(d%time.Second) / int(time.Second/10)
	return fmt.Sprintf("%s.%d", Clock(d.Truncate(time.Second)), tenths)
}

// frameSheet lays frames out as one picture, as many across as down or one
// more, each at its own aspect, within maxEdge, each with its time in its
// corner. Frames are never enlarged.
func frameSheet(frames []Frame, maxEdge int) (image.Image, error) {
	cols := int(math.Ceil(math.Sqrt(float64(len(frames)))))
	rows := (len(frames) + cols - 1) / cols
	fw, fh := 0, 0
	for _, f := range frames {
		b := f.Image.Bounds()
		fw, fh = max(fw, b.Dx()), max(fh, b.Dy())
	}
	gap := max(2, maxEdge/256)
	scale := min(1, float64(maxEdge-gap*(cols-1))/float64(cols*fw), float64(maxEdge-gap*(rows-1))/float64(rows*fh))
	tw, th := int(float64(fw)*scale), int(float64(fh)*scale)
	if tw < 1 || th < 1 {
		return nil, fmt.Errorf("%d frames do not fit %d pixels", len(frames), maxEdge)
	}
	sheet := image.NewRGBA(image.Rect(0, 0, cols*tw+(cols-1)*gap, rows*th+(rows-1)*gap))
	draw.Draw(sheet, sheet.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	for i, f := range frames {
		at := image.Rect(0, 0, tw, th).Add(image.Pt(i%cols*(tw+gap), i/cols*(th+gap)))
		// A frame of another shape keeps its own, centered in the tile.
		b := f.Image.Bounds()
		s := min(float64(tw)/float64(b.Dx()), float64(th)/float64(b.Dy()))
		w, h := int(float64(b.Dx())*s), int(float64(b.Dy())*s)
		into := image.Rect(0, 0, w, h).Add(at.Min).Add(image.Pt((tw-w)/2, (th-h)/2))
		xdraw.CatmullRom.Scale(sheet, into, f.Image, b, draw.Src, nil)
		boxedLabel(sheet, at, fineClock(f.At))
	}
	return sheet, nil
}
