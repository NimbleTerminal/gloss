package app

import (
	"path/filepath"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"nimbleterminal.dev/gloss/internal/remote"
)

// RemoteMsg is a request from outside the terminal: another gloss run with
// --remote, or a signal. It is carried out whatever the viewer is doing;
// help, a search prompt, a picker or the file browser gets out of its way.
// Reply, when not nil, is sent one answer; the sender buffers it.
type RemoteMsg struct {
	Request remote.Request
	Reply   chan<- remote.Response
}

func (v RemoteMsg) answer(r remote.Response) {
	if v.Reply != nil {
		v.Reply <- r
	}
}

// remote carries out a request from outside.
func (m *Model) remote(v RemoteMsg) tea.Cmd {
	if m.isPreview {
		return nil
	}
	if m.exiting {
		v.answer(remote.Response{Error: "the viewer is quitting"})
		return nil
	}
	switch v.Request.Action {
	case "status":
		v.answer(m.remoteStatus(remote.Response{OK: true}))
		return nil
	case "quit":
		v.answer(remote.Response{OK: true})
		return m.quit()
	case "reload":
		if len(m.opts.Files) == 0 {
			v.answer(remote.Response{Error: "nothing is open"})
			return nil
		}
		dismissed := m.dismissOverlays()
		v.answer(m.remoteStatus(remote.Response{OK: true}))
		return tea.Batch(dismissed, m.load(true))
	case "open":
		return m.remoteOpen(v)
	}
	v.answer(remote.Response{Error: "unknown action " + v.Request.Action})
	return nil
}

// remoteOpen shows the files asked for: they are probed as a drop is, and the
// answer says which were taken.
func (m *Model) remoteOpen(v RemoteMsg) tea.Cmd {
	paths := v.Request.Paths
	if len(paths) == 0 {
		v.answer(remote.Response{Error: "open needs paths"})
		return nil
	}
	if m.opts.FilesFS == nil {
		for i, path := range paths {
			if !isAddress(path) && !filepath.IsAbs(path) {
				v.answer(remote.Response{Error: "open needs absolute paths: " + path})
				return nil
			}
			paths[i] = filepath.Clean(path)
		}
	}
	if len(paths) == 1 && isAddress(paths[0]) {
		if m.opts.Fetch == nil {
			v.answer(remote.Response{Error: "the viewer was not run with --fetch, so it opens no addresses"})
			return nil
		}
		cmd := m.open(paths[0])
		if cmd == nil {
			v.answer(remote.Response{Error: m.note})
			return nil
		}
		var resume tea.Cmd
		if m.listing() {
			resume = m.closeMenu(false)
		}
		dismissed := m.dismissOverlays()
		v.answer(m.remoteStatus(remote.Response{OK: true}))
		return tea.Batch(resume, dismissed, cmd)
	}
	if slices.ContainsFunc(paths, isAddress) {
		v.answer(remote.Response{Error: "open takes one address at a time, by itself"})
		return nil
	}
	return m.probeInto(paths, dropResult{remote: true, page: v.Request.Page, reply: v.Reply})
}

func isAddress(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

// remoteStatus fills in what the viewer shows.
func (m *Model) remoteStatus(r remote.Response) remote.Response {
	r.Files = len(m.opts.Files)
	if len(m.opts.Files) > 0 && m.index < len(m.opts.Files) {
		r.File = m.opts.Files[m.index]
		if full, err := filepath.Abs(r.File); err == nil && m.opts.FilesFS == nil {
			r.File = full
		}
		r.Kind, r.Page, r.Pages = m.kind, m.page, m.pages
	}
	return r
}

// answerDrop tells the remote sender what became of its files: the number
// taken, the paths skipped, and the file now shown.
func (m *Model) answerDrop(r dropResult, added int, opened bool) {
	if r.reply == nil {
		return
	}
	resp := remote.Response{OK: opened, Added: added, Skipped: r.skipped}
	if !opened {
		resp.Error = "nothing could be opened"
		if len(r.skipped) > 0 {
			resp.Error += ": " + strings.Join(r.skipped, "; ")
		}
	}
	r.reply <- m.remoteStatus(resp)
}

// dismissOverlays puts away whatever lies over the document or in its place,
// so a request from outside lands on the document itself: help, a layer and
// its picker, the search prompt, a selection, the file browser, and the file
// menu, which is left for the file it set aside.
func (m *Model) dismissOverlays() tea.Cmd {
	m.help, m.helpOffset = false, 0
	m.lineInput, m.placeInput = nil, nil
	cmd := m.closeLayer()
	m.clearSearch()
	if m.typeset != nil {
		if selectCmd, ok := m.typesetSelectKey("esc"); ok {
			cmd = tea.Batch(cmd, selectCmd)
		}
	}
	if m.browsing() {
		m.showDocument()
	}
	if m.listing() || m.suspended {
		// As closeMenu, without loading: the request loads what it wants.
		m.selection = m.index
		m.screen, m.suspended = screenDocument, false
		cmd = tea.Batch(cmd, m.disposePreview(), m.clearPicture())
	}
	return cmd
}
