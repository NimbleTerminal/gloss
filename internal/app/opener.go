//go:build !js

package app

import (
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"nimbleterminal.dev/gloss/internal/document"
	"nimbleterminal.dev/gloss/internal/i18n"
	"nimbleterminal.dev/ntfilepicker"
)

// The orders a folder can be listed in, cycled with Ctrl-S.
const (
	byName = iota
	byDate
	byKind
)

var orderNames = []string{"name", "date", "kind"}

// The marks of the kinds of file, by extension; a folder has its own, and
// what gloss cannot open has none.
var kindMarks = map[string]string{
	".png": "📷", ".jpg": "📷", ".jpeg": "📷", ".gif": "📷", ".webp": "📷", ".bmp": "📷", ".tif": "📷", ".tiff": "📷", ".heic": "📷", ".heif": "📷", ".hif": "📷", ".jxl": "📷",
	".mp4": "🎬", ".m4v": "🎬", ".mov": "🎬", ".webm": "🎬", ".mkv": "🎬", ".ogv": "🎬", ".avi": "🎬", ".mpg": "🎬", ".mpeg": "🎬", ".wmv": "🎬", ".flv": "🎬",
	".svg": "🎨", ".mmd": "🧜", ".mermaid": "🧜", ".pdf": "📕", ".stl": "🧊", ".3mf": "🧊", ".obj": "🧊", ".ply": "🧊", ".geojson": "📍", ".gpx": "📍", ".pmtiles": "📍",
	".md": "📝", ".markdown": "📝", ".mdown": "📝", ".html": "📝", ".htm": "📝", ".txt": "📝", ".text": "📝", ".log": "📝",
	".json": "🧾", ".jsonl": "🧾", ".ndjson": "🧾", ".ipynb": "📓",
	".docx": "📄", ".docm": "📄", ".xlsx": "📊", ".xlsm": "📊", ".csv": "📊", ".tsv": "📊", ".grist": "📊",
}

// typeFilters are the kinds of file the browser can be told to show alone, by
// the marks that say what a file is; the menu names them for people.
type typeFilter struct {
	name  string
	marks []string
}

var typeFilters = []typeFilter{
	{"Pictures", []string{"📷"}},
	{"Drawings (SVG, Mermaid)", []string{"🎨", "🧜"}},
	{"PDF", []string{"📕"}},
	{"Meshes (STL, 3MF, OBJ, PLY)", []string{"🧊"}},
	{"Maps (GeoJSON, GPX, PMTiles)", []string{"📍"}},
	{"Markdown, HTML, text", []string{"📝", "📃"}},
	{"JSON", []string{"🧾"}},
	{"Notebooks", []string{"📓"}},
	{"Word", []string{"📄"}},
	{"Tables (CSV, Excel, Grist)", []string{"📊"}},
	{"Videos", []string{"🎬"}},
}

// title is the kind's name in the menu, in the language in use. The chooser
// knows a kind by this name; gloss keeps the English one, so that a kind
// chosen stays chosen across a change of language.
func (f typeFilter) title() string {
	switch f.name {
	case "Pictures":
		return i18n.T("app.browser.kind.pictures", "Pictures")
	case "Drawings (SVG, Mermaid)":
		return i18n.T("app.browser.kind.drawings", "Drawings (SVG, Mermaid)")
	case "Meshes (STL, 3MF, OBJ, PLY)":
		return i18n.T("app.browser.kind.meshes", "Meshes (STL, 3MF, OBJ, PLY)")
	case "Maps (GeoJSON, GPX, PMTiles)":
		return i18n.T("app.browser.kind.maps", "Maps (GeoJSON, GPX, PMTiles)")
	case "Markdown, HTML, text":
		return i18n.T("app.browser.kind.text", "Markdown, HTML, text")
	case "Notebooks":
		return i18n.T("app.browser.kind.notebooks", "Notebooks")
	case "Tables (CSV, Excel, Grist)":
		return i18n.T("app.browser.kind.tables", "Tables (CSV, Excel, Grist)")
	case "Videos":
		return i18n.T("app.browser.kind.videos", "Videos")
	}
	return f.name // A format's own name: PDF, JSON, Word.
}

// shownKinds are the chooser's names for the kinds of file named in
// English, and keptKinds the English names of those it names.
func shownKinds(names []string) []string {
	var out []string
	for _, f := range typeFilters {
		if slices.Contains(names, f.name) {
			out = append(out, f.title())
		}
	}
	return out
}

func keptKinds(titles []string) []string {
	var out []string
	for _, f := range typeFilters {
		if slices.Contains(titles, f.title()) {
			out = append(out, f.name)
		}
	}
	return out
}

// browseFilters are those kinds as the chooser takes them. A file is looked
// into only for the kind that text by content is part of.
func browseFilters() []ntfilepicker.Filter {
	out := make([]ntfilepicker.Filter, len(typeFilters))
	for i, f := range typeFilters {
		byContent := slices.Contains(f.marks, "📃")
		out[i] = ntfilepicker.Filter{Name: f.title(), Mark: f.marks[0], Match: func(e fs.DirEntry) bool {
			if m, ok := extMark(e); ok {
				return slices.Contains(f.marks, m)
			}
			return byContent && slices.Contains(f.marks, mark(e))
		}}
	}
	return out
}

// extMark is the mark an extension gives, and whether it gave one: a folder has
// its own, and what gloss cannot show, or cannot tell by name, has none.
func extMark(e fs.DirEntry) (string, bool) {
	if e.IsDir() {
		return "📁", true
	}
	m, ok := kindMarks[strings.ToLower(filepath.Ext(e.Name()))]
	return m, ok
}

// mark is what stands before a name in the listing, in place of its mode. A file
// whose name says nothing is looked into, here, as it is drawn, and once.
func mark(e fs.DirEntry) string {
	if m, ok := extMark(e); ok {
		return m
	}
	if s, ok := e.(*sniffed); ok && s.isText() {
		return "📃" // Text by its content, whatever its name.
	}
	return "  "
}

// ordering compares two files, or two folders, for the order named.
func ordering(by int) func(a, b fs.DirEntry) int {
	switch by {
	case byDate:
		return func(a, b fs.DirEntry) int {
			ai, aErr := a.Info()
			bi, bErr := b.Info()
			if aErr != nil || bErr != nil {
				return strings.Compare(a.Name(), b.Name())
			}
			if c := bi.ModTime().Compare(ai.ModTime()); c != 0 {
				return c // Newest first.
			}
			return strings.Compare(a.Name(), b.Name())
		}
	case byKind:
		return func(a, b fs.DirEntry) int {
			if c := strings.Compare(kindOrder(a), kindOrder(b)); c != 0 {
				return c
			}
			return strings.Compare(a.Name(), b.Name())
		}
	}
	return nil
}

// The kinds in the order they are listed: pictures, drawings, documents,
// meshes, maps, text, data, notebooks, Word, and tables; the unmarked last.
var kindRanks = []string{"📷", "🎨", "📕", "🧊", "📍", "📝", "📃", "🧾", "📓", "📄", "📊"}

// kindOrder groups files by kind, then extension.
func kindOrder(e fs.DirEntry) string {
	rank := slices.Index(kindRanks, mark(e))
	if rank < 0 {
		rank = len(kindRanks)
	}
	return string(rune('a'+rank)) + strings.ToLower(filepath.Ext(e.Name()))
}

// opener browses the host's folders for a file to add to the session. The
// browsing itself is ntfilepicker's; this is what gloss adds to it.
type opener struct {
	picker ntfilepicker.Model
	reads  *readLog
	dir    string // The folder being browsed, for the status bar.
	going  bool   // A folder's path is being typed, to go to.
	find   *finder
	width  int
	height int
}

// filterPrompt and goPrompt start the line typed on: names to filter by,
// or a folder to go to.
func filterPrompt() string { return "  " + i18n.T("app.browser.filterPrompt", "Filter: ") }
func goPrompt() string     { return "  " + i18n.T("app.browser.goPrompt", "Go to: ") }

// goTo asks for a folder's path, starting from the one shown, with the
// picker's completion at hand.
func (o *opener) goTo() tea.Cmd {
	o.going = true
	o.picker.SetPrompt(goPrompt())
	o.picker.SetDirsOnlyCompletion(true) // The aim is a folder; its files are still listed.
	return o.picker.SetFilter(o.dir + string(filepath.Separator))
}

// leavePopup closes what floats over the folder, the menu of kinds or the
// cursor in the sidebar of places, and says whether there was any.
func (o *opener) leavePopup() bool {
	switch {
	case o.picker.InMenu():
		o.picker.CloseMenu()
	case o.picker.InSidebar():
		o.picker.LeaveSidebar()
	default:
		return false
	}
	return true
}

// stopGoing ends the go-to, the filter cleared.
func (o *opener) stopGoing() tea.Cmd {
	o.going = false
	o.picker.SetPrompt(filterPrompt())
	o.picker.SetDirsOnlyCompletion(false)
	return o.picker.ClearFilter()
}

type openResult struct {
	path string
	err  error
}

// readLog is the filesystem the picker reads: where files that gloss cannot
// show are looked into, and left out when that is asked for. It reads from a
// command, outside the update loop, so its choices are kept under a lock.
type readLog struct {
	fs.ReadDirFS
	mu      sync.Mutex
	allowed []string // Extensions the picker does not grey; nil allows all.
	hide    bool
	text    bool // Files that read as text may be chosen, whatever their name.
}

// A file as listed, to be looked into if its name does not say what it is:
// whether it reads as text is found out when asked, as the row is drawn or the
// file chosen, and then kept. A folder is listed without opening what is in it,
// which the folders beside and under the one browsed would otherwise cost.
type sniffed struct {
	fs.DirEntry
	look func() bool
	once sync.Once
	text bool
}

// isText looks at the file's start the first time it is asked.
func (s *sniffed) isText() bool {
	s.once.Do(func() { s.text = s.look() })
	return s.text
}

// ReadDir lists a folder, links to folders made folders, and the files that
// cannot be chosen left out when that is asked for, which means looking into
// each file that no extension accounts for.
func (l *readLog) ReadDir(name string) ([]fs.DirEntry, error) {
	l.mu.Lock()
	hide, allowed := l.hide && l.allowed != nil, l.allowed
	l.mu.Unlock()
	entries, err := l.ReadDirFS.ReadDir(name)
	entries = ntfilepicker.FollowLinks(l, name, entries) // Before any are left out.
	if allowed == nil {
		return entries, err
	}
	for i, e := range entries {
		if e.IsDir() || slices.Contains(allowed, strings.ToLower(filepath.Ext(e.Name()))) {
			continue
		}
		file := path.Join(name, e.Name())
		entries[i] = &sniffed{DirEntry: e, look: func() bool { return l.readsAsText(file, e) }}
	}
	if !hide {
		return entries, err
	}
	return slices.DeleteFunc(entries, func(e fs.DirEntry) bool { return !e.IsDir() && !l.selectable(e) }), err
}

// Stat describes a file without opening it, as os.Stat does, if the
// filesystem can: opening is what a link to a pipe must not suffer.
func (l *readLog) Stat(name string) (fs.FileInfo, error) {
	if s, ok := l.ReadDirFS.(fs.StatFS); ok {
		return s.Stat(name)
	}
	return fs.Stat(l.ReadDirFS, name)
}

// readsAsText looks at the start of a file, if it is a file: a pipe or a
// device is not opened, for that may wait for ever or have an effect, and a link
// is followed only to a file.
func (l *readLog) readsAsText(name string, e fs.DirEntry) bool {
	if !e.Type().IsRegular() {
		if e.Type()&fs.ModeSymlink == 0 {
			return false
		}
		if info, err := l.Stat(name); err != nil || !info.Mode().IsRegular() {
			return false
		}
	}
	f, err := l.Open(name)
	if err != nil {
		return false
	}
	defer f.Close()
	head := make([]byte, document.TextSniff+1)
	n, _ := io.ReadFull(f, head)
	return document.IsText(head[:n])
}

// selectable says whether a listed file may be chosen: one of gloss's
// kinds by extension, or text while text files are offered.
func (l *readLog) selectable(e fs.DirEntry) bool {
	if s, ok := e.(*sniffed); ok {
		l.mu.Lock()
		text := l.text
		l.mu.Unlock()
		return text && s.isText() // Looking into the file is done without the lock.
	}
	return true
}

func (o *opener) view() string {
	if o.find != nil {
		return o.find.view(o.width, o.height)
	}
	return o.picker.View()
}

func (o *opener) resize(w, h int) {
	if o == nil {
		return
	}
	o.width, o.height = w, h
	if o.find != nil {
		o.find.input.SetWidth(max(1, w-lipgloss.Width(o.find.input.Prompt)-1))
	}
	o.picker.SetSize(w, h)
}

// The embedded gallery has no folders, and a preview never takes keys.
func (m *Model) canBrowse() bool { return m.opts.FilesFS == nil && !m.isPreview }

func (m *Model) openBrowser() tea.Cmd {
	if !m.canBrowse() {
		return nil
	}
	dir, err := os.Getwd()
	if len(m.opts.Files) > 0 && !m.opts.IsStdin(m.opts.Files[m.index]) {
		if file, fileErr := filepath.Abs(m.opts.Files[m.index]); fileErr == nil {
			dir, err = filepath.Dir(file), nil
		}
	}
	if err != nil {
		m.note = i18n.Tf("app.note.browseFailed", "cannot browse: %s", safe(err.Error()))
		return nil
	}
	return m.browseFrom(dir)
}

func (m *Model) browseFrom(dir string) tea.Cmd {
	dir, err := filepath.Abs(dir)
	if err != nil || !m.canBrowse() {
		return nil
	}
	reads := &readLog{ReadDirFS: os.DirFS(string(filepath.Separator)).(fs.ReadDirFS), hide: m.hideUnsupported, text: !m.noTextFiles}
	options := []ntfilepicker.Option{ntfilepicker.WithMarker(mark), ntfilepicker.WithSort(ordering(m.sortBy)), ntfilepicker.WithLayout(ntfilepicker.Layout(m.browseLayout)), ntfilepicker.WithPrompt(filterPrompt()),
		ntfilepicker.WithFilters(browseFilters()), ntfilepicker.WithActiveFilters(shownKinds(m.browseTypes)...), ntfilepicker.WithLinksFollowed(), ntfilepicker.WithUnchoosableNote(i18n.T("app.browser.unchoosable", "gloss cannot show this file"))}
	if m.opts.Type == "" {
		// Judged by name where the name says; the rest by a look inside.
		reads.allowed = document.Extensions
		options = append(options, ntfilepicker.WithSelectable(reads.selectable))
	}
	m.opener = &opener{picker: ntfilepicker.New(reads, filepath.ToSlash(dir), options...), reads: reads, dir: dir}
	m.opener.resize(m.width, m.bodyHeight())
	m.screen, m.help = screenBrowser, false
	m.keepLayer()
	return tea.Batch(m.clearQR(), m.opener.picker.Init())
}

func (m *Model) browse(msg tea.Msg) tea.Cmd {
	o := m.opener
	entered := false
	if k, ok := msg.(tea.KeyPressMsg); ok {
		if o.find != nil {
			if cmd, ok := m.findKey(k); ok {
				return cmd
			}
		}
		switch {
		case k.String() == "/" && !o.going && o.picker.FilterValue() == "":
			o.leavePopup() // Or it would keep the keys of the finder.
			o.find = newFinder(o.dir, o.width)
			return nil
		case k.String() == "G" && !o.going && o.picker.FilterValue() == "":
			o.leavePopup() // Or Enter would check a kind, or go to a place, not the folder.
			return o.goTo()
		case k.String() == "enter" && o.going:
			entered = true
		}
	}
	was := o.picker.Dir()
	var cmd tea.Cmd
	o.picker, cmd = o.picker.Update(msg)
	o.dir, m.browseLayout, m.browseTypes = filepath.FromSlash(o.picker.Dir()), int(o.picker.Layout()), keptKinds(o.picker.ActiveFilters())
	if o.going && (o.picker.Dir() != was || entered && o.picker.FilterValue() == "") {
		// The folder aimed at was gone to: by Enter on it, or by any other way
		// of changing folders (a click on a crumb or a place, up, back), after
		// which the filter is the filter again and G and / mean what they did.
		o.going = false
		o.picker.SetPrompt(filterPrompt())
		o.picker.SetDirsOnlyCompletion(false)
	}
	chosen, ok := o.picker.Chosen()
	if !ok {
		return cmd
	}
	o.picker.ClearChosen()
	path, forced, accept := filepath.FromSlash(chosen), m.opts.Type, m.opts.Accept
	return tea.Batch(cmd, func() tea.Msg {
		_, err := document.Probe(path, forced)
		if err == nil {
			err = accept.CheckFile(nil, path)
		}
		return openResult{path: path, err: err}
	})
}

// reorder lists the folder in the next order.
func (m *Model) reorder() tea.Cmd {
	m.sortBy = (m.sortBy + 1) % len(orderNames)
	m.opener.picker.SetSort(ordering(m.sortBy))
	return nil
}

func (m *Model) toggleUnsupported() tea.Cmd {
	m.hideUnsupported = !m.hideUnsupported
	return m.reread()
}

// toggleText offers text files, or sets them aside.
func (m *Model) toggleText() tea.Cmd {
	m.noTextFiles = !m.noTextFiles
	return m.reread()
}

// reread lists the folders again under the choices made.
func (m *Model) reread() tea.Cmd {
	o := m.opener
	o.reads.mu.Lock()
	o.reads.hide, o.reads.text = m.hideUnsupported, !m.noTextFiles
	o.reads.mu.Unlock()
	return o.picker.Reload()
}

func (m *Model) opened(r openResult) tea.Cmd {
	if !m.browsing() {
		return nil // Cancelled while the file was being examined.
	}
	if r.err != nil {
		// The browser stays open: choosing something else is one key away.
		m.note = safe(filepath.Base(r.path)) + ": " + safe(document.SkipReason(r.err))
		return nil
	}
	return m.addDropped(dropResult{files: []string{r.path}})
}

// A question mark starts help only before text entry; F1 is always available.
func (m *Model) browserQuestionHelp() bool {
	return m.opener != nil && m.opener.find == nil && !m.opener.going && m.opener.picker.FilterValue() == ""
}
