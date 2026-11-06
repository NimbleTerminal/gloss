package app

import (
	"errors"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"nimbleterminal.dev/gloss/internal/document"
	"nimbleterminal.dev/gloss/internal/i18n"
	"nimbleterminal.dev/gloss/internal/remote"
)

// DropMsg adds files to the session. Hosts without bracketed paste, such as
// the browser demo, send it directly.
type DropMsg struct{ Paths []string }

type dropResult struct {
	files   []string
	skipped []string
	folder  string // The first folder dropped, to browse when no file came.
	// A request from outside says where to start and waits to hear what
	// became of its files; a drop does neither.
	remote bool
	page   int
	reply  chan<- remote.Response
}

// Skipped lists the dropped files that could not be added.
func (m *Model) Skipped() []string { return m.skipped }

// Probing reads from disk, so it runs outside the update loop.
func (m *Model) probeDrop(paths []string) tea.Cmd { return m.probeInto(paths, dropResult{}) }

// probeInto probes paths into r, which keeps what the result is to carry
// besides them.
func (m *Model) probeInto(paths []string, r dropResult) tea.Cmd {
	if len(paths) == 0 {
		return nil
	}
	files, forced, accept := m.opts.FilesFS, m.opts.Type, m.opts.Accept
	strict := m.opts.StrictMode
	return func() tea.Msg {
		var folders []string
		for _, path := range paths {
			if _, err := document.ProbeFS(files, path, forced, strict); err != nil {
				if errors.Is(err, document.ErrDirectory) && files == nil {
					folders = append(folders, path)
					continue
				}
				r.skipped = append(r.skipped, document.Skipped(path, err))
				continue
			}
			if err := accept.CheckFile(files, path, strict); err != nil {
				r.skipped = append(r.skipped, document.Skipped(path, err))
				continue
			}
			r.files = append(r.files, path)
		}
		// A folder alone is somewhere to look; beside files it is skipped,
		// as on the command line, where a glob may sweep one in.
		for _, folder := range folders {
			if len(r.files) == 0 && r.folder == "" {
				r.folder = folder
			} else {
				r.skipped = append(r.skipped, document.Skipped(folder, document.ErrDirectory))
			}
		}
		return r
	}
}

func (m *Model) fileIndex(path string) int {
	same := func(a, b string) bool { return a == b }
	if m.opts.FilesFS == nil {
		same = func(a, b string) bool {
			a, errA := filepath.Abs(a)
			b, errB := filepath.Abs(b)
			return errA == nil && errB == nil && a == b
		}
	}
	for i, file := range m.opts.Files {
		if same(file, path) {
			return i
		}
	}
	return -1
}

func (m *Model) addDropped(r dropResult) tea.Cmd {
	m.skipped = append(m.skipped, r.skipped...)
	first, added := -1, 0
	for _, path := range r.files {
		if !m.tableFile(path) {
			m.handOver(path) // A table's CSV is the viewer's, not the user's to send.
		}
		i := m.fileIndex(path)
		if i < 0 {
			m.opts.Files = append(m.opts.Files, path)
			i = len(m.opts.Files) - 1
			if added == 0 {
				first = i
			}
			added++
		}
		if first < 0 {
			first = i
		}
	}
	var notes []string
	if added > 0 {
		notes = append(notes, i18n.Tn("app.note.added", "added {{.Count}}", "added {{.Count}}", added))
	}
	if len(r.skipped) > 0 {
		notes = append(notes, i18n.Tf("app.note.skipped", "skipped %d: %s", len(r.skipped), r.skipped[0]))
	}
	m.note = strings.Join(notes, " · ")
	if first < 0 {
		m.answerDrop(r, added, r.folder != "")
		if r.folder != "" {
			var dismissed tea.Cmd
			if r.remote {
				dismissed = m.dismissOverlays()
			}
			return tea.Batch(dismissed, m.browseFrom(r.folder))
		}
		return nil
	}
	var dismissed tea.Cmd
	if r.remote {
		dismissed = m.dismissOverlays()
	}
	m.help = false
	if m.browsing() {
		m.showDocument()
	}
	defer m.answerDrop(r, added, true)
	var cmd tea.Cmd
	switch {
	case added > 1 && m.listing():
		m.selection = first
		cmd = tea.Batch(m.updatePreview(), m.layoutGrid())
	case added > 1:
		cmd = m.openMenu(first)
	case m.listing():
		m.selection = first
		cmd = m.closeMenu(true)
	case r.remote || r.page > 0:
		// Asked from outside, or to start on a page: the file starts over,
		// even the one on screen.
		cmd = m.showRestoring(first, func() {
			if r.page > 0 {
				m.page = r.page
			}
		})
	case first == m.index:
		cmd = m.load(true)
	default:
		cmd = m.switchFile(first - m.index)
	}
	return tea.Batch(dismissed, cmd)
}

type outsideDrop struct{ paths []string }

// awaitDrops waits for the next files from outside; each arrival renews it.
func (m *Model) awaitDrops() tea.Cmd {
	drops := m.opts.Drops
	if drops == nil {
		return nil
	}
	return func() tea.Msg {
		paths, ok := <-drops
		if !ok {
			return nil
		}
		return outsideDrop{paths}
	}
}

// handOver notes a file the user gave, whether or not it was already listed.
func (m *Model) handOver(path string) {
	if full, err := filepath.Abs(path); err == nil && m.opts.FilesFS == nil {
		path = full
	}
	for _, have := range m.added {
		if have == path {
			return
		}
	}
	m.added = append(m.added, path)
}

// Picked lists the files sent to the caller; nil if the user sent none.
func (m *Model) Picked() []string { return m.picked }

// picking says what Enter would send, or "" when it would send nothing.
func (m *Model) picking() string {
	switch {
	case !m.opts.Pick || len(m.opts.Files) == 0:
		return ""
	case len(m.added) > 1:
		return i18n.Tn("app.pick.files", "{{.Count}} file", "{{.Count}} files", len(m.added))
	case len(m.added) == 1:
		return safe(filepath.Base(m.added[0]))
	}
	if m.tableFile(m.opts.Files[m.index]) {
		return ""
	}
	return safe(filepath.Base(m.opts.Files[m.index]))
}

// pick sends what the user handed over. Files the caller offered are
// choices: with nothing handed over, the one on screen is the answer.
func (m *Model) pick() tea.Cmd {
	if m.picking() == "" {
		return nil
	}
	if m.picked = append([]string(nil), m.added...); len(m.picked) == 0 {
		path := m.opts.Files[m.index]
		if full, err := filepath.Abs(path); err == nil && m.opts.FilesFS == nil {
			path = full
		}
		m.picked = []string{path}
	}
	return m.quit()
}
