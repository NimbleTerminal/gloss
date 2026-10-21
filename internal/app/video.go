package app

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	ntpicture "nimbleterminal.dev/ntpicture"
	"nimbleterminal.dev/ntvideo"

	"nimbleterminal.dev/gloss/internal/document"
)

// videoSeek is how far ← and → move in a video.
const videoSeek = 5 * time.Second

// showVideo plays a loaded video in the body, through ntvideo: paused on its
// first frame until Space, with a scrub bar on its last row. Its picture
// shares the pager's terminal session and image IDs.
func (m *Model) showVideo(v document.Result, cleanup tea.Cmd) tea.Cmd {
	m.source = nil
	m.videoInfo = v.Video
	source := v.Video.Path
	if v.Video.URL != "" {
		source = v.Video.URL // In the browser: the file the page holds.
	}
	m.video = ntvideo.New(
		ntvideo.WithSource(source),
		// In the browser, the page's host lays a <video> over the cells;
		// gloss keeps the keys and draws the scrub bar, so it has no
		// controls of its own. Natively these change nothing.
		ntvideo.WithHost("terminal"),
		ntvideo.WithNativeControls(false),
		ntvideo.WithPoster(v.Image),
		ntvideo.WithAutoplay(false),
		ntvideo.WithScrubber(true),
		ntvideo.WithTerminalSession(m.terminal),
		ntvideo.WithImageIDAllocator(ntpicture.NextImageID),
		ntvideo.WithKittyMedium(ntpicture.KittyMediumSharedMemory),
		ntvideo.WithWidth(m.width), ntvideo.WithHeight(m.bodyHeight()),
	)
	m.keepLayer()
	if at := m.savedVideoAt; at > 0 {
		m.video.Seek(at) // Where the menu, or a reload, left it.
	}
	m.savedVideoAt = 0
	return tea.Sequence(cleanup, m.video.Init(), m.layoutVideo())
}

// layoutVideo puts the video over the body, under any prompt above it.
func (m *Model) layoutVideo() tea.Cmd {
	if m.video == nil {
		return nil
	}
	top := 0
	if m.opts.PromptTop {
		top = m.promptHeight()
	}
	return m.video.SetBounds(ntvideo.CellRect{X: 0, Y: top, Cols: m.width, Rows: m.bodyHeight()})
}

// clearVideo stops the video and takes its picture down, then frees the
// browser's URL for it, which must outlive the video.
func (m *Model) clearVideo() tea.Cmd {
	if m.video == nil {
		return nil
	}
	video, info := m.video, m.videoInfo
	m.video, m.videoInfo = nil, nil
	closed := video.Close()
	if info == nil || info.Revoke == nil {
		return closed
	}
	return tea.Sequence(closed, func() tea.Msg { info.Revoke(); return nil })
}

// updateVideo passes a message to the video, and shows or hides it as the
// screen does: help, the menu and the app chooser cover it, and pause it.
func (m *Model) updateVideo(msg tea.Msg) tea.Cmd {
	if m.video == nil {
		return nil
	}
	visible := m.screen == screenDocument && !m.help && !m.suspended && m.openWithPicker == nil
	cmd := m.video.SetVisible(visible)
	if _, mouse := msg.(tea.MouseMsg); mouse {
		// Its bounds, and so its scrub bar, are in the terminal's cells.
		if !visible || m.layer != layerNone {
			return cmd
		}
	}
	return tea.Batch(cmd, m.video.Update(msg))
}

// videoKey gives the video its keys: Space plays and pauses, ← and → seek,
// Home goes back to the start.
func (m *Model) videoKey(k string) (tea.Cmd, bool) {
	if m.video == nil {
		return nil, false
	}
	s := m.video.Status()
	switch k {
	case "space":
		switch s.State {
		case ntvideo.Playing, ntvideo.Buffering:
			m.video.Pause()
		default:
			// At the end (mpv keeps the last frame, paused), start again.
			if s.State == ntvideo.Ended || s.Duration > 0 && s.CurrentTime >= s.Duration-time.Second/10 {
				m.video.Seek(0)
			}
			m.video.Play()
		}
		return nil, true
	case "left", "h":
		m.video.Seek(max(0, s.CurrentTime-videoSeek))
		return nil, true
	case "right", "l":
		at := s.CurrentTime + videoSeek
		if s.Duration > 0 {
			at = min(at, s.Duration)
		}
		m.video.Seek(at)
		return nil, true
	case "home":
		m.video.Seek(0)
		return nil, true
	}
	return nil, false
}

// videoDetail is the status line's account of the video.
func (m *Model) videoDetail() string {
	s := m.video.Status()
	detail := fmt.Sprintf("VIDEO · %s", clock(s.CurrentTime))
	if s.Duration > 0 {
		detail += "/" + clock(s.Duration)
	}
	state := string(s.State)
	if s.Err != nil {
		state = s.Err.Error()
	}
	return detail + " · " + state
}

// clock writes a time as m:ss, or h:mm:ss.
func clock(d time.Duration) string {
	d = d.Round(time.Second)
	h, mm, ss := int(d.Hours()), int(d.Minutes())%60, int(d.Seconds())%60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, mm, ss)
	}
	return fmt.Sprintf("%d:%02d", mm, ss)
}
