package app

import (
	"strings"
	"testing"
	"testing/fstest"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func status(t *testing.T, v tea.View) tea.ProgramStatus {
	t.Helper()
	if v.ProgramStatus == nil {
		t.Fatal("the view reports no status: a nil status clears every record")
	}
	if v.ProgramStatus.App != "gloss" || v.ProgramStatus.ID != "" {
		t.Fatalf("status %+v is not gloss's root record", *v.ProgramStatus)
	}
	return *v.ProgramStatus
}

func wantStatus(t *testing.T, v tea.View, state tea.ProgramState, message string) {
	t.Helper()
	if s := status(t, v); s.State != state || s.Message != message {
		t.Fatalf("status = %s %q; want %s %q", s.State, s.Message, state, message)
	}
}

// Loading works, a document at rest is idle, one that failed is an error,
// and quitting leaves idle behind.
func TestViewerStatus(t *testing.T) {
	files := fstest.MapFS{"notes.txt": {Data: []byte("notes\n")}, "broken.json": {Data: []byte("{")}}
	m := New(Options{Files: []string{"notes.txt", "broken.json"}, FilesFS: files, Render: "glyph", Page: 1})
	t.Cleanup(func() { _ = m.Close() })
	m.width, m.height = 80, 10
	load := m.load(false)
	wantStatus(t, m.View(), tea.ProgramStateWorking, "Loading")
	drainAnimation(t, m, load)
	wantStatus(t, m.View(), tea.ProgramStateIdle, "")
	if first, again := status(t, m.View()), status(t, m.View()); first != again {
		t.Fatal("redrawing changed the status")
	}
	drainAnimation(t, m, m.switchFile(1))
	if m.err == nil {
		t.Fatal("broken JSON loaded")
	}
	wantStatus(t, m.View(), tea.ProgramStateError, "Could not open the file")
	m.quit()
	wantStatus(t, m.View(), tea.ProgramStateIdle, "")
}

// Picking blocks on the person until Enter sends the files: then done.
func TestPickStatus(t *testing.T) {
	m := picking(t, samples+"landscape.png")
	wantStatus(t, m.View(), tea.ProgramStateBlocked, "Waiting for files")
	send(m, enter)
	if m.Picked() == nil {
		t.Fatal("nothing picked")
	}
	wantStatus(t, m.View(), tea.ProgramStateDone, "Files chosen")

	cancelled := picking(t, samples+"landscape.png")
	send(cancelled, press("q"))
	wantStatus(t, cancelled.View(), tea.ProgramStateIdle, "")
}

func TestHandoffStatus(t *testing.T) {
	for _, tc := range []struct {
		end     HandoffEnd
		state   tea.ProgramState
		message string
	}{
		{HandoffReceived, tea.ProgramStateDone, "Files received"},
		{HandoffDeclined, tea.ProgramStateIdle, ""},
		{HandoffTimedOut, tea.ProgramStateError, "Timed out waiting for files"},
		{HandoffFailed, tea.ProgramStateError, "Could not receive files"},
	} {
		m, err := NewHandoff(handoffURL, "Send a photo", "glyph", make(chan HandoffEnd), func() {})
		if err != nil {
			t.Fatal(err)
		}
		m.Update(tea.WindowSizeMsg{Width: 80, Height: 40})
		wantStatus(t, m.View(), tea.ProgramStateBlocked, "Waiting for files")
		m.Update(handoffFinished{tc.end})
		wantStatus(t, m.View(), tc.state, tc.message)
		m.Close()
	}
	m, err := NewHandoff(handoffURL, "", "glyph", make(chan HandoffEnd), func() {})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	wantStatus(t, m.View(), tea.ProgramStateIdle, "")
}

// Reports carry no address, token, path or prompt: only gloss's own words.
func TestStatusEncodingIsGeneric(t *testing.T) {
	m, err := NewHandoff(handoffURL, "Send a photo", "glyph", make(chan HandoffEnd), func() {})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	seq := ansi.SetProgramStatus(*m.View().ProgramStatus)
	if !strings.HasPrefix(seq, "\x1b]7501;") || !strings.Contains(seq, "app=gloss") {
		t.Fatalf("report %q", seq)
	}
	for _, secret := range []string{handoffURL, "photo"} {
		if strings.Contains(seq, secret) {
			t.Fatalf("report %q carries %q", seq, secret)
		}
	}
}
