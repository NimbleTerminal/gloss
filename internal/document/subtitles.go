package document

import (
	"bytes"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

// Subtitle files beside a video, named after it: clip.srt, clip.en.srt,
// clip.vtt, clip.fr.ass. mpv reads them natively; the browser reads WebVTT,
// so an SRT is converted for it and ASS/SSA are left out there.
const (
	maxSubtitles     = 16
	maxSubtitleBytes = 2 << 20
)

var subtitleExtensions = []string{".srt", ".vtt", ".ass", ".ssa"}

// Subtitle is a subtitle file found for a video.
type Subtitle struct {
	Path     string // The file, beside the video.
	URL      string // In the browser, a blob: URL of it as WebVTT.
	Language string // From the name, clip.en.srt: en; empty when it says none.
}

// Label names a subtitle track for a person: its language, or its format.
func (s Subtitle) Label() string {
	if s.Language != "" {
		return s.Language
	}
	return strings.ToUpper(strings.TrimPrefix(filepath.Ext(s.Path), "."))
}

// findSubtitles lists the subtitle files beside a video, named after it,
// within bounds: the one with no language first, then by name.
func findSubtitles(files fs.FS, video string) []Subtitle {
	var dir, base string
	var entries []fs.DirEntry
	var err error
	if files == nil {
		dir, base = filepath.Dir(video), filepath.Base(video)
		entries, err = os.ReadDir(dir)
	} else {
		dir, base = path.Dir(video), path.Base(video)
		entries, err = fs.ReadDir(files, dir)
	}
	if err != nil {
		return nil
	}
	stem := strings.TrimSuffix(base, filepath.Ext(base)) + "."
	var out []Subtitle
	for _, e := range entries {
		name := e.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if !e.Type().IsRegular() || !strings.HasPrefix(name, stem) || !slices.Contains(subtitleExtensions, ext) {
			continue
		}
		if info, err := e.Info(); err != nil || info.Size() > maxSubtitleBytes {
			continue
		}
		// clip.en.srt names English; clip.srt no language.
		language := strings.TrimPrefix(name[:len(name)-len(ext)]+".", stem)
		language = strings.TrimSuffix(language, ".")
		full := filepath.Join(dir, name)
		if files != nil {
			full = path.Join(dir, name)
		}
		out = append(out, Subtitle{Path: full, Language: language})
	}
	slices.SortFunc(out, func(a, b Subtitle) int {
		if (a.Language == "") != (b.Language == "") {
			if a.Language == "" {
				return -1
			}
			return 1
		}
		return strings.Compare(a.Path, b.Path)
	})
	return out[:min(len(out), maxSubtitles)]
}

// srtToVTT turns SubRip into WebVTT, which is SubRip with a header and full
// stops for commas in its times: what a browser's <track> can read.
func srtToVTT(srt []byte) []byte {
	srt = bytes.TrimPrefix(srt, []byte("\xef\xbb\xbf"))
	srt = bytes.ReplaceAll(srt, []byte("\r\n"), []byte("\n"))
	lines := bytes.Split(srt, []byte("\n"))
	for i, line := range lines {
		if bytes.Contains(line, []byte("-->")) {
			lines[i] = bytes.ReplaceAll(line, []byte(","), []byte("."))
		}
	}
	return append([]byte("WEBVTT\n\n"), bytes.Join(lines, []byte("\n"))...)
}

// subtitleFields says which subtitle files a video has, for the info box.
func subtitleFields(subs []Subtitle) []Field {
	if len(subs) == 0 {
		return nil
	}
	names := make([]string, len(subs))
	for i, s := range subs {
		names[i] = filepath.Base(s.Path)
	}
	return []Field{{"Subtitles", strings.Join(names, ", ")}}
}
