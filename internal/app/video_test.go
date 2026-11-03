//go:build !js

package app

import (
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"nimbleterminal.dev/gloss/internal/document"
	"nimbleterminal.dev/ntmediabox"
	"nimbleterminal.dev/ntvideo"
)

func stepCmd(m *Model, cmd tea.Cmd) tea.Cmd {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if msg == nil {
		return nil
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		var next []tea.Cmd
		for _, c := range batch {
			if follow := stepCmd(m, c); follow != nil {
				next = append(next, follow)
			}
		}
		return tea.Batch(next...)
	}
	v := reflect.ValueOf(msg)
	if v.Kind() == reflect.Slice && v.Type().Elem() == reflect.TypeOf(tea.Cmd(nil)) {
		var next []tea.Cmd
		for i := 0; i < v.Len(); i++ {
			if follow := stepCmd(m, v.Index(i).Interface().(tea.Cmd)); follow != nil {
				next = append(next, follow)
			}
		}
		return tea.Batch(next...)
	}
	_, follow := m.Update(msg)
	return follow
}

func TestVideoSandboxPlayback(t *testing.T) {
	if !ntmediabox.Available() {
		t.Skipf("OS sandbox unavailable: %s", ntmediabox.Probe().Detail)
	}
	probePlayer := ntvideo.NewPlayer(ntvideo.PlayerOptions{
		DisableAudio:            true,
		DisableHardwareDecoding: true,
	})
	probeErr := probePlayer.Load("nonexistent.mp4")
	_ = probePlayer.Close()
	if probeErr != nil && (strings.Contains(probeErr.Error(), "not available") || strings.Contains(probeErr.Error(), "libmpv") || strings.Contains(probeErr.Error(), "dlopen")) {
		t.Skipf("libmpv not available: %v", probeErr)
	}

	if runtime.GOOS == "linux" {
		stageBin := filepath.Join(t.TempDir(), "ntconfine-stage")
		cmd := exec.Command("go", "build", "-o", stageBin, "nimbleterminal.dev/ntconfine/cmd/ntconfine-stage")
		cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("build ntconfine-stage: %v\n%s", err, out)
		}
		prev := videoStagePath
		videoStagePath = stageBin
		t.Cleanup(func() { videoStagePath = prev })
	}

	clipPath, err := filepath.Abs("../../examples/recording.mp4")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	localClip := filepath.Join(dir, "recording.mp4")
	data, err := os.ReadFile(clipPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(localClip, data, 0o600); err != nil {
		t.Fatal(err)
	}
	subPath := filepath.Join(dir, "recording.en.srt")
	const srt = "1\n00:00:00,000 --> 00:00:10,000\nSandboxed subtitle line\n"
	if err := os.WriteFile(subPath, []byte(srt), 0o600); err != nil {
		t.Fatal(err)
	}

	m := New(Options{Files: []string{localClip}, Render: "glyph", VideoSandbox: true})
	defer m.Close()
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m.kind = "video"
	next := m.loaded(document.Result{
		Generation: m.generation,
		Kind:       "video",
		Page:       1,
		Pages:      1,
		Video: &document.Video{
			Path:      localClip,
			Subtitles: []document.Subtitle{{Path: subPath, Language: "en"}},
		},
		Image: image.NewRGBA(image.Rect(0, 0, 16, 9)),
	})
	if m.video == nil {
		t.Fatal("expected m.video to be created")
	}

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		next = stepCmd(m, next)
		st := m.video.Status()
		if st.State == ntvideo.Failed {
			t.Fatalf("sandboxed video failed: %v", st.Err)
		}
		view := m.View().Content
		if st.Duration > 0 && len(st.CaptionTracks) > 0 && strings.Contains(view, "Sandboxed subtitle line") {
			return
		}
	}
	st := m.video.Status()
	t.Fatalf("timed out waiting for sandboxed video playback and subtitle; status=%+v\nview:\n%s", st, m.View().Content)
}
