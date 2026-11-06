package app

import (
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"nimbleterminal.dev/gloss/internal/remote"
)

// ask delivers a request as the socket server does, and gives its answer.
func ask(t *testing.T, m *Model, req remote.Request) remote.Response {
	t.Helper()
	reply := make(chan remote.Response, 1)
	deliver(m, RemoteMsg{Request: req, Reply: reply})
	select {
	case r := <-reply:
		return r
	default:
		t.Fatal("no answer")
		return remote.Response{}
	}
}

func abs(t *testing.T, path string) string {
	t.Helper()
	full, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	return full
}

func TestRemoteOpenPutsOverlaysAway(t *testing.T) {
	m := New(Options{Files: []string{samples + "shapes.svg"}, Render: "glyph", Page: 1, DPI: 72})
	defer m.Close()
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	m.kind, m.zoom, m.help = "svg", 2, true
	m.editSearch()
	m.layer = layerInfo
	picture := abs(t, samples+"landscape.png")
	r := ask(t, m, remote.Request{Action: "open", Paths: []string{picture}})
	if !r.OK || r.Added != 1 || len(r.Skipped) != 0 || r.File != picture || r.Files != 2 {
		t.Fatalf("answer %+v", r)
	}
	if len(m.opts.Files) != 2 || m.index != 1 || m.zoom != 0 || !m.loading {
		t.Fatalf("not opened: files=%q index=%d zoom=%d loading=%v", m.opts.Files, m.index, m.zoom, m.loading)
	}
	if m.help || m.search != nil || m.layer != layerNone {
		t.Fatalf("overlays stayed: help=%v search=%v layer=%v", m.help, m.search != nil, m.layer)
	}
}

func TestRemoteOpenStartsOnThePageAsked(t *testing.T) {
	m := New(Options{Files: []string{samples + "shapes.svg"}, Render: "glyph", Page: 1, DPI: 72})
	defer m.Close()
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	m.kind = "svg"
	// The file on screen, asked for again on another page, starts over there.
	r := ask(t, m, remote.Request{Action: "open", Paths: []string{abs(t, samples+"shapes.svg")}, Page: 3})
	if !r.OK || r.Added != 0 || r.Page != 3 || r.Files != 1 {
		t.Fatalf("answer %+v", r)
	}
	if m.index != 0 || m.page != 3 || !m.loading {
		t.Fatalf("index=%d page=%d loading=%v", m.index, m.page, m.loading)
	}
}

func TestRemoteOpenLeavesTheMenu(t *testing.T) {
	m := New(Options{Files: []string{samples + "shapes.svg", samples + "readme.md"}, Render: "glyph", Page: 1, DPI: 72})
	defer m.Close()
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	m.kind = "svg"
	_ = m.openMenu(0)
	if !m.listing() {
		t.Fatal("test needs the menu open")
	}
	r := ask(t, m, remote.Request{Action: "open", Paths: []string{abs(t, samples+"readme.md")}})
	if !r.OK || r.Added != 0 || !strings.HasSuffix(r.File, "readme.md") {
		t.Fatalf("answer %+v", r)
	}
	if m.listing() || m.suspended || m.index != 1 || !m.loading {
		t.Fatalf("menu=%v suspended=%v index=%d loading=%v", m.listing(), m.suspended, m.index, m.loading)
	}
}

func TestRemoteOpenRefusals(t *testing.T) {
	m := New(Options{Files: []string{samples + "shapes.svg"}, Render: "glyph", Page: 1, DPI: 72})
	defer m.Close()
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	m.kind = "svg"
	missing := filepath.Join(t.TempDir(), "gone.png")
	r := ask(t, m, remote.Request{Action: "open", Paths: []string{missing}})
	if r.OK || r.Added != 0 || len(r.Skipped) != 1 || !strings.Contains(r.Error, "nothing could be opened") {
		t.Fatalf("missing file: %+v", r)
	}
	if len(m.opts.Files) != 1 || m.loading {
		t.Fatal("a missing file changed the viewer")
	}
	for _, req := range []remote.Request{
		{Action: "open"},
		{Action: "open", Paths: []string{"relative.png"}},
		{Action: "open", Paths: []string{"https://example.com/a.pdf"}}, // No --fetch.
		{Action: "open", Paths: []string{"https://example.com/a.pdf", missing}},
		{Action: "dance"},
	} {
		if r := ask(t, m, req); r.OK || r.Error == "" {
			t.Fatalf("%+v was answered %+v", req, r)
		}
	}
}

func TestRemoteStatusReloadAndQuit(t *testing.T) {
	m := New(Options{Files: []string{samples + "shapes.svg", samples + "readme.md"}, Render: "glyph", Page: 1, DPI: 72})
	defer m.Close()
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	m.kind, m.page, m.pages, m.loading = "pdf", 2, 5, false
	r := ask(t, m, remote.Request{Action: "status"})
	if !r.OK || r.File != abs(t, samples+"shapes.svg") || r.Kind != "pdf" || r.Page != 2 || r.Pages != 5 || r.Files != 2 {
		t.Fatalf("status %+v", r)
	}
	if m.loading {
		t.Fatal("status loaded something")
	}
	m.help = true
	generation := m.generation
	if r := ask(t, m, remote.Request{Action: "reload"}); !r.OK || r.File != abs(t, samples+"shapes.svg") {
		t.Fatalf("reload %+v", r)
	}
	// The test harness runs the load to its end; it was a fresh one.
	if m.generation != generation+1 || m.help {
		t.Fatalf("generation=%d (was %d) help=%v", m.generation, generation, m.help)
	}
	if r := ask(t, m, remote.Request{Action: "quit"}); !r.OK {
		t.Fatalf("quit %+v", r)
	}
	if !m.exiting {
		t.Fatal("quit did not begin")
	}
	if r := ask(t, m, remote.Request{Action: "status"}); r.OK {
		t.Fatalf("after quit %+v", r)
	}
}

func TestRemoteReloadWithNothingOpen(t *testing.T) {
	m := New(Options{Render: "glyph", Page: 1, DPI: 72})
	defer m.Close()
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	if r := ask(t, m, remote.Request{Action: "reload"}); r.OK || r.Error == "" {
		t.Fatalf("reload of nothing: %+v", r)
	}
	if r := ask(t, m, remote.Request{Action: "status"}); !r.OK || r.File != "" || r.Files != 0 {
		t.Fatalf("status of nothing: %+v", r)
	}
}
