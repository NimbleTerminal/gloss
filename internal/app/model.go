package app

import (
	"context"
	"image"
	"image/color"
	"image/draw"
	"io/fs"
	"sync/atomic"
	"time"

	ntanimation "nimbleterminal.dev/ntpicture/animation"
	"nimbleterminal.dev/ntpicture/playback"

	tea "charm.land/bubbletea/v2"
	"nimbleterminal.dev/gloss/internal/document"
	"nimbleterminal.dev/gloss/internal/openwith"
	"nimbleterminal.dev/gloss/internal/theme"
	"nimbleterminal.dev/nt3d"
	widget "nimbleterminal.dev/nt3d/widget"
	"nimbleterminal.dev/ntmaps"
	ntpicture "nimbleterminal.dev/ntpicture"
	svg "nimbleterminal.dev/ntsvg"
	"nimbleterminal.dev/ntvideo"
)

type Options struct {
	Version     string // Build version shown in help; empty uses dev.
	OpenWith    openwith.Host
	IsTemporary func(string) bool
	// OpenLink asks the embedding host to open an explicitly clicked web URL.
	// Nil leaves link activation to the terminal (OSC 8). It never fetches a document.
	OpenLink func(string) error
	// CopyImage puts a PNG on the system clipboard as a picture. Nil where
	// pictures cannot reach the system clipboard.
	CopyImage func(png []byte) error
	// Scratch writes data to a new file called name in a folder of the
	// viewer's own temporaries, and gives its path. The file is the host's to
	// remove, never overwrites another, and is not the user's document. Nil
	// where there is no such folder.
	Scratch                func(name string, data []byte) (string, error)
	STLCamera              *nt3d.Camera    // Optional initial/reset view for an embedded gallery.
	Views                  []document.View // Views of a mesh asked for: the first is where the viewer starts.
	Times                  []time.Duration // Moments of a video asked for: an export draws them, the viewer starts at the first.
	FrameCount             int             // Frames of a video to export, spread through it.
	FilesFS                fs.FS           // Optional embedded files for the browser demo.
	Files                  []string
	Stdin                  string // The file standard input was read into, if one of Files is it.
	Type, Render, Render3D string
	MarkdownMode           string
	Typeset                bool
	LineNumbers            bool // Show file line numbers in text and typeset views of line-based documents.
	ChopLines              bool // Cut long lines at the right edge instead of wrapping them, as less -S does.
	Theme                  string
	Lang                   string             // The user's language tag, as es-MX; "" is English, and leaves handoff pages to the browser.
	SystemLang             string             // The language without a saved choice, which the picker's System stands for; "" is Lang.
	SaveLang               func(string) error // Keeps the picker's choice; "" is System. Nil where nothing is saved.
	SavedLang              string             // The kept choice the picker opens on; "" is System.
	LangFixed              bool               // --lang or GLOSS_LANG decided the language, over any saved choice.
	Font, Mono             string
	SaveFont               func(string, string) error
	ThemeChoices           []theme.Choice
	SaveTheme              func(string) error
	Accept                 document.AcceptFilter
	Page, DPI              int
	Menu, Preview          bool
	Browse                 string // Folder to open the file browser in at the start.
	KeepScreen             bool   // Draw on the main screen; the last view stays after quitting.
	Pick                   bool   // The caller waits for files: Enter sends them, and ends the viewer.
	Prompt                 string // What the caller asks of the user, shown in a box throughout.
	PromptTop              bool   // The box above the body, rather than below it.
	// Drops delivers files handed over from outside the terminal, as the
	// page serving the viewer does with what is dropped on it.
	Drops <-chan []string
	// OnQuit tells an embedding host that the user deliberately finished.
	OnQuit func()
	// Fetch downloads a web address to a file and returns its path. Nil
	// leaves the network alone, which is the default.
	Fetch func(address string) (string, error)
	// Maps is the online map provider the user named, for tiles under maps
	// and place lookup. Nil, the default, keeps maps offline; it is nil in
	// the browser, --serve and every headless mode.
	Maps *MapProvider
	// Columns picks the columns of every sheet to show; the rest are hidden.
	Columns document.ColumnFilter
	// Parts picks the parts of every 3MF to show; the rest are left out.
	Parts document.PartFilter
	// Up is the axis STL, OBJ and PLY files treat as up, "y" or "z"; empty
	// takes each format's usual one.
	Up string
	// Color paints the faces of a mesh that its file left plain.
	Color *color.RGBA
	// Background is the color behind a mesh; nil keeps the default.
	Background *color.RGBA
	terminal   *ntpicture.TerminalSession // Shared with previews on this terminal.
	// Save stores an export and returns the name it was given. Nil writes to
	// the working directory; the browser demo offers a download instead.
	Save              func(name string, png []byte) (string, error)
	Output, OutputDir string
	MaxEdge           int
	MarkdownBase      string // Base directory for piped Markdown assets.
}

// meshState is what the model keeps for a mesh on a chart: the chart itself,
// the parts and paint chosen for it, and the camera it starts from. It is
// embedded in Model, so its fields read as the model's own.
type meshState struct {
	chart       *widget.Model
	video       *ntvideo.Model  // A video being played; its first frame stands in elsewhere.
	videoInfo   *document.Video // The file it plays, as the loader found it.
	mapView     *ntmaps.Model   // A GeoJSON or GPX file's map.
	mapFile     *document.Map   // What it shows, as the loader read it.
	placeInput  *placeInput     // The find-a-place prompt, while it is open.
	triangles   int
	points      int             // A point cloud's, which has no triangles.
	parts       []document.Part // What a 3MF build places, when a mesh is made of parts.
	assemble    func([]bool) (*document.Mesh, error)
	partsShown  []bool // Which parts are on the chart; nil for all.
	partPicker  *partPicker
	mesh        *document.Mesh // The mesh on the chart.
	tint        *color.RGBA    // Paint given to the mesh, if any.
	bg          *color.RGBA    // Background chosen behind the mesh, if any.
	tintAll     bool           // On every face, not only the plain ones.
	colorPicker *colorPicker
	savedCamera *nt3d.Camera
	// Where a video was when the menu set it aside, to go back to.
	savedVideoAt time.Duration
	home         nt3d.Camera    // Where the camera starts, and returns on reset.
	homeView     *document.View // The view the home is fitted from, when it is a fit.
}

// textState is what the model keeps for a Markdown or sheet document, and the
// view to return to when a fetched file closes.
type textState struct {
	typesetLinks       []document.TypesetLink
	typesetSelect      *document.TypesetSelection // The text selected, or being selected, on the typeset page.
	typesetSelectShown *document.TypesetSelection // The selection the shown picture highlights.
	typesetSelected    string                     // The shown selection's text.
	typesetDragging    bool                       // A left button is down on the typeset page.
	typesetDragFrom    image.Point                // Where it went down, on the page.
	pressedLink        string
	hoverLinkPoint     image.Point
	hoverLinkActive    bool
	linkPoint          image.Point
	typesetPlayback    *ntanimation.Composite
	typesetLayers      *playback.Layers
	typesetPaused      bool
	typesetPending     *document.Result // A page view whose overlays wait for the view to settle.
	typesetSettle      uint64
	search             *textSearch
	searchWorker       *searchWorker
	markdown           *markdownView
	typeset            *document.Markdown
	typesetWidth       int
	typesetRevision    uint64
	typesetActive      uint64
	typesetWaitSize    bool
	typesetCancel      context.CancelFunc
	// The document being loaded, and what ends its loads when the pager
	// leaves it: another file, a reload, or closing.
	document                string
	documentCtx             context.Context
	documentCancel          context.CancelFunc
	typesetTop, typesetLeft int
	typesetEnd              bool
	lineNumbers             bool                   // Show file line numbers, where the view's rows are file lines.
	lineInput               *lineInput             // The go-to-line prompt, while it is open.
	typesetGoLine           int                    // A line the typeset page is asked to put at the top, until it answers.
	typesetGoLink           string                 // A document-local footnote target awaiting layout.
	typesetGoTop            int                    // A page top to reveal the focused item at, until it answers; -1 for none.
	typesetItems            []document.TypesetItem // The typeset Markdown page's pictures, links and tables laid so far.
	typesetDone             bool                   // typesetItems are all there are: the page is laid, or as far as its last item.
	typesetFocus            []image.Rectangle      // The item Tab is on, as page rectangles; nil for none.
	typesetFocusIndex       int                    // Its place in typesetItems, to find it again after a reflow.
	typesetFocusShown       []image.Rectangle      // The focus the shown picture outlines.
	typesetHover            []document.TypesetMark // The text of the link under the pointer, tinted; nil for none.
	typesetHoverShown       []document.TypesetMark // The text the shown picture tints.
	typesetTabPending       int                    // A Tab step to take again on the next page view, which lays more.
	typesetAhead            bool                   // Ask the next page view to lay on to the next item.
	chopLines               bool                   // Cut long lines at the right edge instead of wrapping them.
	typesetShown            *document.TypesetViewport
	typesetTheme            string
	typesetFont             string
	savedMarkdown           *markdownView
	sheet                   *sheetView
	savedSheet              *sheetView // The cell to return to when a fetched file closes.
	savedFocus              int        // 1 + the Markdown item to focus again when the document comes back; 0 for none.
}

// browserState is what the file browser remembers between visits.
type browserState struct {
	opener          *opener
	hideUnsupported bool // The browser's choice outlasts any one visit.
	sortBy          int  // The order the browser lists in; kept likewise.
	thumbs          bool // The list is shown as a grid of thumbnails, last time and next.
	grid            *grid
	noTextFiles     bool     // The browser sets text files aside; kept likewise.
	browseLayout    int      // How the browser lays folders out (an ntfilepicker.Layout); kept likewise.
	browseTypes     []string // The kinds of file the browser shows alone, if any; kept likewise.
}

// Model is the pager. A preview pane is itself a Model (isPreview set) that
// the list owns (see updatePreview): built with Menu and Preview off, so it
// has no preview of its own, it takes no drops or mouse input, and it loads
// with Request.Preview so the loader keeps it small.
type Model struct {
	keepOnMainScreen  bool // Q is moving the retained picture out of the alternate screen.
	openWithPicker    *openWithPicker
	openWithOperation int
	meshState
	textState
	helpOffset int // Rows the help page is scrolled, where it is taller than the screen.
	browserState

	opts                     Options
	loader                   *document.Loader
	terminal                 *ntpicture.TerminalSession
	ownTerminal              bool
	lastCellW, lastCellH     int  // Last geometry applied by this consumer of the shared session.
	kitty                    bool // Current graphics mode; capability and geometry belong to terminal.
	qr                       *qrOverlay
	actions                  *actionMenu
	width, height, index     int
	page, pages              int
	generation               uint64
	kind                     string
	loading, help, autoKitty bool
	theme                    string
	detectedTheme            string
	themePicker              *themePicker
	langPicker               *langPicker
	diagramsDark             bool // The palette the last request drew Mermaid diagrams in.
	fontPicker               *fontPicker
	err                      error
	source                   image.Image
	animation                *animation
	playback                 *playback.Controller
	still                    *stillPicture
	zoom                     int
	panX, panY               float64
	screen                   screen // What fills the body.
	layer                    layer  // What floats over a document.
	selection                int
	preview                  *Model
	isPreview                bool
	ownerID                  int
	previewDrag              bool
	suspended                bool
	note                     string // Outcome of the last drop, shown until the next key.
	fields                   []document.Field
	quitting                 bool     // The view being drawn is the one left behind.
	exiting                  bool     // Quit has begun: the status reported is the last.
	added                    []string // What the user has handed over, by full path.
	fetching                 bool
	fetched                  []fetchedDoc // Files fetched from the web this session.
	picked                   []string
	skipped                  []string // Reported on stderr once the terminal is restored.
}

var nextMessageOwner atomic.Int64

// nextOwnerID identifies asynchronous work independently of terminal image IDs.
func nextOwnerID() int { return int(nextMessageOwner.Add(1)) }

func New(opts Options) *Model {
	if opts.Version == "" {
		opts.Version = "dev"
	}
	if opts.SystemLang == "" {
		opts.SystemLang = opts.Lang
	}
	ownTerminal := opts.terminal == nil
	if ownTerminal {
		opts.terminal = newTerminalSession(opts.Render)
	}
	id := nextOwnerID()
	m := &Model{opts: opts, terminal: opts.terminal, ownTerminal: ownTerminal, meshState: meshState{tint: opts.Color, bg: opts.Background}, loader: &document.Loader{Files: opts.FilesFS}, ownerID: id, kitty: opts.Render != "glyph" && opts.terminal.Snapshot().Kitty == ntpicture.KittyCapabilitySupported, page: opts.Page, pages: 1, autoKitty: opts.Render != "glyph"}
	if len(opts.Times) > 0 {
		m.savedVideoAt = opts.Times[0] // The first video opened starts there.
	}
	m.lineNumbers, m.chopLines = opts.LineNumbers, opts.ChopLines
	m.theme = "dark"
	m.detectedTheme = "dark"
	m.typesetFont = document.FontKey(opts.Font, opts.Mono)
	if opts.Theme != "" && opts.Theme != "auto" {
		m.theme = theme.Resolve(opts.Theme)
	}
	if (opts.Menu || opts.Preview) && len(opts.Files) > 0 {
		m.screen = screenList
	}
	m.lastCellW, m.lastCellH = m.cellPixelSize()
	m.typesetGoTop = -1
	return m
}

func (m *Model) Init() tea.Cmd {
	var browse tea.Cmd
	if m.opts.Browse != "" {
		browse = m.browseFrom(m.opts.Browse)
	}
	if m.screen == screenList {
		m.suspended = true
		return tea.Batch(m.themeRequest(), m.terminal.Init(), m.updatePreview(), browse, m.awaitDrops())
	}
	return tea.Batch(m.themeRequest(), m.terminal.Init(), m.load(false), browse, m.awaitDrops())
}

func (m *Model) Close() error {
	m.clearStandalone()
	m.clearTypesetLayers()
	m.stopTypeset()
	m.leaveDocument()
	m.clearStill()
	m.clearFontPicker()
	m.clearSearch()
	m.stopAnimation()
	if m.qr != nil {
		m.qr.model.Close()
	}
	if m.markdown != nil {
		m.markdown.close()
	}
	if m.preview != nil {
		_ = m.preview.Close()
	}
	if m.chart != nil {
		_ = m.chart.Close()
	}
	if m.video != nil {
		_ = m.video.Release()
		if m.videoInfo != nil && m.videoInfo.Revoke != nil {
			m.videoInfo.Revoke()
		}
	}
	if m.grid != nil {
		_ = m.grid.loader.Close() // A PDF thumbnail's worker.
	}
	if m.ownTerminal {
		m.terminal.Close()
	}
	return m.loader.Close()
}

func (m *Model) Err() error { return m.err }

// mouseSettle is how long quitting waits, with mouse reporting turned off,
// before giving the terminal back. Bubble Tea stops reading before it turns
// reporting off, so a report sent as the user quit, the pointer still moving,
// would be left for the shell, typed at its prompt; read here, it is dropped.
const mouseSettle = 50 * time.Millisecond

func settleMouse() tea.Msg {
	time.Sleep(mouseSettle)
	return nil
}

// quit gives the terminal back. Graphics are taken down with the alternate
// screen. On the main screen the document is left as it was last seen.
func (m *Model) quit() tea.Cmd {
	if m.opts.OnQuit != nil {
		m.opts.OnQuit()
	}
	m.exiting = true
	m.clearSearch()
	stopped := m.stopAnimation()
	qrCleanup := tea.Batch(stopped, m.clearQR(), m.clearFontPicker())
	if !m.opts.KeepScreen {
		return tea.Sequence(tea.Batch(qrCleanup, m.clearGraphics(), m.disposePreview()), settleMouse, tea.Quit)
	}
	m.quitting, m.help = true, false
	if m.browsing() {
		m.showDocument()
	}
	var markdownCleanup tea.Cmd
	if m.markdown != nil {
		markdownCleanup = m.markdown.keep(m.keepOnMainScreen)
	}
	var keepMesh tea.Cmd
	if m.chart != nil {
		keepMesh = m.chart.Keep()
	}
	if m.video != nil {
		m.video.Pause()
		keepMesh = tea.Batch(keepMesh, m.video.Keep())
	}
	if m.mapView != nil {
		keepMesh = tea.Batch(keepMesh, m.mapView.Keep())
	}
	return tea.Sequence(tea.Batch(qrCleanup, markdownCleanup, m.keepAnimationPicture(), keepMesh), settleMouse, tea.Quit)
}

// IsStdin says whether path is the file standard input was read into.
func (o Options) IsStdin(path string) bool { return o.Stdin != "" && path == o.Stdin }

// request is what the loader is asked for the current file and page.
func (m *Model) request(reload bool) document.Request {
	q := document.Request{Path: m.opts.Files[m.index], Type: m.opts.Type, MarkdownMode: m.opts.MarkdownMode, Typeset: m.opts.Typeset, Theme: DocumentTheme(m.theme), Locale: DocumentLocale(m.opts.Lang), Font: m.opts.Font, Mono: m.opts.Mono, Page: m.page, DPI: m.opts.DPI, Generation: m.generation, Reload: reload, Preview: m.isPreview, Animate: !m.isPreview,
		Stdin: m.opts.IsStdin(m.opts.Files[m.index]), Parts: m.opts.Parts, Up: m.opts.Up, Shown: m.partsShown, Color: m.tint, PaintAll: m.tintAll}
	m.diagramsDark = theme.Dark(m.theme)
	q.TypesetWidth = m.typesetMeasure()
	q.TypesetLineNumbers, q.TypesetChop = m.lineNumbers, m.chopLines
	if !reload {
		if m.typesetPlayback != nil {
			q.Playback = m.typesetPlayback.Positions()
		}
		if m.typesetLayers != nil {
			q.Playback = m.typesetLayers.Positions()
		}
	}
	if m.opts.typesetEnabled() && !m.isPreview {
		view := m.typesetViewport()
		if m.typeset == nil && m.page > 1 {
			view.Top = (m.page - 1) * 1125
		}
		q.TypesetView = &view
		if m.typesetSelect != nil {
			// The loader reads its copy off the event loop; a drag moves ours.
			sel := *m.typesetSelect
			q.TypesetSelect = &sel
		}
		q.TypesetFocus, q.TypesetMarks = m.typesetFocus, m.typesetHover
	}
	if m.opts.IsStdin(q.Path) {
		q.BaseDir = m.opts.MarkdownBase
	}
	return q
}

func (m *Model) load(reload bool) tea.Cmd {
	if reload {
		m.clearSearch()
	}
	if len(m.opts.Files) == 0 {
		return nil
	}
	if m.opts.typesetEnabled() && m.width == 0 && !m.isPreview {
		m.typesetWaitSize, m.loading = true, true
		return nil
	}
	m.typesetWaitSize = false
	stopped := m.stopAnimation()
	m.generation++
	m.loading, m.err = true, nil
	q := m.request(reload)
	m.stopTypeset()
	same := m.documentCtx != nil && q.Path == m.document && !reload
	if !same {
		m.leaveDocument()
		m.document = q.Path
		m.documentCtx, m.documentCancel = context.WithCancel(context.Background())
	}
	q.Context, m.typesetCancel = context.WithCancel(m.documentCtx)
	if same && m.kind == "pdf" {
		// The worker that has the PDF open answers for every page: turning
		// a page must not stop the worker the next page needs.
		m.typesetCancel()
		q.Context, m.typesetCancel = m.documentCtx, nil
	}
	preview := m.isPreview
	return tea.Batch(stopped, func() tea.Msg {
		r := m.loader.Load(q)
		if preview {
			return previewResult{owner: m.ownerID, result: r}
		}
		return r
	})
}

// leaveDocument ends the loads of the document the pager is leaving.
func (m *Model) leaveDocument() {
	if m.documentCancel != nil {
		m.documentCancel()
	}
	m.document, m.documentCtx, m.documentCancel = "", nil, nil
}

func (m *Model) bodyHeight() int {
	if m.isPreview {
		return max(1, m.height) // No bars: the list beside it has them.
	}
	return max(1, m.height-2-m.promptHeight())
}

func (m *Model) switchFile(delta int) tea.Cmd {
	i := m.index + delta
	if i < 0 || i >= len(m.opts.Files) {
		return nil
	}
	return m.show(i)
}

// show loads the file at i afresh: its page, zoom, and camera start over.
func (m *Model) show(i int) tea.Cmd { return m.showRestoring(i, nil) }

// showRestoring is show, with restore run after the old file's state is
// cleared and before the new one is asked for, to set what it starts from.
func (m *Model) showRestoring(i int, restore func()) tea.Cmd {
	m.clearSearch()
	m.index, m.page, m.pages, m.kind = i, 1, 1, ""
	m.source, m.zoom, m.panX, m.panY = nil, 0, 0, 0
	m.savedCamera, m.savedMarkdown, m.savedFocus, m.fields = nil, nil, 0, nil
	m.partsShown = nil // A choice of parts is for one file.
	cleared := m.clearGraphics()
	if restore != nil {
		restore()
	}
	return tea.Sequence(cleared, m.load(false))
}

// Close releases renderer resources; Kitty images must also be explicitly
// deleted while the terminal is still running.
func (m *Model) clearChart() tea.Cmd {
	if m.chart == nil {
		return nil
	}
	chart := m.chart
	m.chart = nil
	cleanup := chart.Dispose()
	_ = chart.Close()
	return cleanup
}

func (m *Model) clearGraphics() tea.Cmd {
	m.stopTypeset()
	m.typesetTop, m.typesetLeft, m.typesetEnd, m.typesetShown = 0, 0, false, nil
	m.typesetGoLine, m.lineInput = 0, nil
	m.typesetGoLink, m.typesetGoTop = "", -1
	m.typeset = nil
	m.typesetLinks = nil
	m.clearTypesetFocus()
	m.typesetItems, m.typesetDone, m.typesetFocusShown = nil, false, nil
	m.typesetHover, m.typesetHoverShown = nil, nil
	m.clearTypesetSelection()
	m.typesetSelectShown = nil
	m.pressedLink = ""
	m.hoverLinkActive = false
	m.typesetPlayback = nil
	m.typesetPending = nil
	overlayCleanup := m.clearTypesetLayers()
	stopped := m.stopAnimation()
	m.sheet = nil
	if m.layer == layerActions {
		// A new file, a reload or a drop leaves no menu holding the mouse.
		m.closeLayer()
	}
	return tea.Batch(stopped, m.clearStandalone(), m.clearStill(), m.clearFontPicker(), overlayCleanup, m.clearQR(), m.clearChart(), m.clearVideo(), m.clearMap(), m.clearMarkdown())
}

func (m *Model) clearMarkdown() tea.Cmd {
	if m.markdown == nil {
		return nil
	}
	cmd := m.markdown.close()
	m.markdown = nil
	return cmd
}

func (m *Model) layoutMarkdown() tea.Cmd {
	if m.markdown == nil {
		return nil
	}
	m.markdown.setTheme(m.theme)
	m.markdown.numbers, m.markdown.fenced, m.markdown.chop = m.lineNumbers, m.kind == "text" || m.kind == "json", m.chopLines
	cw, ch := m.cellPixelSize()
	return m.markdown.layout(m.width, m.bodyHeight(), cw, ch)
}

// paged reports whether n and p turn pages, or sheets, rather than files.
func (m *Model) paged() bool { return m.kind == "pdf" || m.booked() }

// booked reports whether the file is one of several sheets: a workbook, or
// the tables of a Grist document.
func (m *Model) booked() bool { return m.kind == "xlsx" || m.kind == "grist" }

func (m *Model) movePage(page int) tea.Cmd {
	if !m.paged() {
		return nil
	}
	page = max(1, min(page, m.pages))
	if page == m.page {
		return nil
	}
	m.page, m.zoom, m.panX, m.panY = page, 0, 0, 0
	return m.load(false)
}

// Update hands the message to the screen it is for, then lets every picture
// consumer see it: they must, even when a dialog took the message.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.exiting {
		switch msg.(type) {
		case tea.KeyPressMsg, tea.MouseMsg, tea.PasteMsg:
			return m, nil // Quit has begun: what is still typed or pointed at is dropped.
		}
	}
	terminalCmd := m.terminal.Update(msg)
	if m.autoKitty {
		m.kitty = m.terminal.Snapshot().Kitty == ntpicture.KittyCapabilitySupported
	}
	cmd := m.handle(msg)
	return m, tea.Batch(terminalCmd, cmd, m.graphicsTick(msg))
}

// graphicsTick passes a message to the pictures, after the screen has handled
// it: search, the still picture, animation, typeset layers, Markdown pictures,
// the font preview and the QR code, which follow whatever the screen became.
func (m *Model) graphicsTick(msg tea.Msg) tea.Cmd {
	cmd := tea.Batch(m.syncSearch(), m.updateStill(msg))
	if m.typeset == nil {
		m.typesetPending = nil
		cmd = tea.Batch(cmd, m.clearTypesetLayers())
	}
	cmd = tea.Batch(cmd, m.updateStandalone(msg), m.updateTypesetLayers(msg), m.updateVideo(msg))
	if m.markdown != nil {
		cmd = tea.Batch(cmd, m.markdown.syncPictures(!m.loading && !m.suspended && !m.quitting && !m.help && m.screen == screenDocument), m.markdown.update(msg))
	}
	if p := m.fontPicker; p != nil {
		cmd = tea.Batch(cmd, p.pic.Update(msg), m.layoutFontPicker())
	}
	if m.qr != nil {
		cw, ch := m.cellPixelSize()
		cmd = tea.Batch(cmd, m.qr.model.SetTerminal(m.kitty, cw, ch), m.layoutQR(), m.qr.model.Update(msg))
	}
	return cmd
}

// handle is the screen's answer to a message; Update runs graphicsTick after
// it on every path.
func (m *Model) handle(msg tea.Msg) tea.Cmd {
	cw, ch := m.lastCellW, m.lastCellH
	if _, mouse := msg.(tea.MouseMsg); mouse && (m.qr != nil || m.themePicker != nil || m.fontPicker != nil || m.langPicker != nil) {
		return nil
	}
	if cmd, handled := m.openWithBrowse(msg); handled {
		return cmd
	}
	if _, mouse := msg.(tea.MouseMsg); mouse && m.layer == layerActions {
		// The menu takes every click, before the sheet, the links and the
		// scrubber; behind the help page it takes them and does nothing.
		if m.help || m.openWithPicker != nil || m.screen != screenDocument {
			return nil
		}
		cmd, _ := m.actionsMouse(m.onBody(msg))
		return cmd
	}
	if _, ok := msg.(tea.MouseMsg); ok && m.openWithPicker != nil {
		return nil
	}
	if mouse, ok := msg.(tea.MouseMsg); ok {
		if cmd, took := m.scrubberMouse(mouse); took {
			return cmd
		}
	}
	if m.browsing() {
		if _, ok := msg.(tea.MouseMsg); ok {
			if m.help || m.opener.find != nil {
				return nil
			}
			return m.browse(m.onBody(msg))
		}
	}
	msg = m.onBody(msg)
	if click, ok := msg.(tea.MouseClickMsg); ok && click.Button == tea.MouseRight && m.itemsHere() {
		return m.openItemActions(image.Pt(click.X, click.Y), true)
	}
	hoverCmd := m.typesetHoverMouse(msg)
	selectCmd, selecting := m.typesetSelectMouse(msg)
	if cmd, handled := m.linkMouse(msg); handled || selecting {
		return tea.Batch(cmd, selectCmd, hoverCmd)
	}
	if hoverCmd != nil {
		return hoverCmd
	}
	if mouse, ok := msg.(tea.MouseMsg); ok && m.listing() {
		if m.help {
			return nil
		}
		if m.screen == screenGrid && m.grid != nil {
			return m.gridMouse(mouse)
		}
		return m.listMouse(mouse)
	}
	if mouse, ok := msg.(tea.MouseMsg); ok && m.pickingColor() {
		if cmd, took := m.colorMouse(mouse); took {
			if m.colorPicker == nil {
				m.layer = layerNone
			}
			return cmd
		}
	}
	if cmd, handled := m.terminalEditorMessage(msg); handled {
		return cmd
	}
	switch v := msg.(type) {
	case openWithAppsLoaded:
		return m.openWithAppsLoaded(v)
	case openWithLoaded:
		return m.openWithLoaded(v)
	case openWithFinished:
		return m.openWithFinished(v)
	case fontPreviewResult:
		if p := m.fontPicker; p != nil && p.owner == v.owner && p.revision == v.revision {
			p.base, p.err = v.image, v.err
			return m.showFontPreview()
		}
		return nil
	case fontSaved:
		if v.err != nil {
			m.note = "save font: " + safe(v.err.Error())
		}
		return nil
	case themeSaved:
		if v.err != nil {
			m.note = "save theme: " + safe(v.err.Error())
		}
		return nil
	case langSaved:
		if v.err != nil {
			m.note = "save language: " + safe(v.err.Error())
		}
		return nil
	case linkOpened:
		if v.generation == m.generation && v.err != nil {
			m.note = "open link: " + safe(v.err.Error())
		}
		return nil
	case tea.BackgroundColorMsg:
		if v.Color == nil {
			return nil
		}
		name := "light"
		if v.IsDark() {
			name = "dark"
		}
		m.detectedTheme = name
		if m.themePicker != nil || m.opts.Theme != "" && m.opts.Theme != "auto" {
			return nil
		}
		return m.applyTheme(name)
	case tea.FocusMsg:
		return m.themeRequest()
	case searchResult:
		return m.searched(v)
	case placeFound:
		return m.showPlace(v)
	case previewResult:
		if m.preview != nil && v.owner == m.preview.ownerID {
			_, cmd := m.preview.Update(v.result)
			return cmd
		}
		return nil
	case typesetResize:
		return m.reflowTypeset(v)
	case typesetSettled:
		return m.typesetSettled(v)
	case typesetReflowResult:
		return m.typesetReflowed(v)
	case tea.WindowSizeMsg:
		oldWidth, oldHeight := m.width, m.height
		m.width, m.height = max(1, v.Width), max(1, v.Height)
		m.opener.resize(m.width, m.bodyHeight())
		var resize tea.Cmd
		if m.source != nil && m.kitty {
			if oldWidth != m.width || oldHeight != m.height {
				if m.animation != nil {
					resize = m.resizeAnimation()
				} else {
					resize = m.refreshImage()
				}
			}
		}
		cmd := tea.Batch(resize, m.resizeStill(), m.resizePreview(), m.layoutMarkdown(), m.layoutGrid())
		if m.typesetWaitSize && !m.suspended {
			cmd = tea.Batch(cmd, m.load(false))
		}
		if m.typeset != nil {
			if m.markdown == nil {
				cmd = tea.Sequence(cmd, m.refreshImage())
			}
			cmd = tea.Batch(cmd, m.queueTypesetReflow())
		}
		if m.chart != nil {
			return tea.Batch(cmd, m.chart.SetSize(m.width, m.meshHeight()), m.refit())
		}
		if m.mapView != nil {
			return tea.Batch(cmd, m.mapView.SetSize(m.width, m.bodyHeight()))
		}
		return tea.Batch(cmd, m.layoutVideo())
	case tea.PasteMsg:
		if m.search != nil && m.search.editing {
			m.search.insert(v.Content)
			return nil
		}
		// Terminals deliver dropped files as a bracketed paste of their paths.
		if m.isPreview || m.opts.FilesFS != nil {
			return nil
		}
		if address := document.DropURL(v.Content); address != "" {
			return m.open(address)
		}
		return m.probeDrop(document.ParseDrop(v.Content))
	case DropMsg:
		if m.isPreview {
			return nil
		}
		return m.probeDrop(v.Paths)
	case outsideDrop:
		return tea.Batch(m.probeDrop(v.paths), m.awaitDrops())
	case dropResult:
		return m.addDropped(v)
	case exportResult:
		m.note = v.note()
		return nil
	case imageCopied:
		return m.imageCopied(v)
	case shownSaved:
		return m.shownSaved(v)
	case tableSaved:
		return m.tableSaved(v)
	case openResult:
		return m.opened(v)
	case findResult:
		if m.browsing() && m.opener.find != nil {
			m.opener.find.answered(v)
		}
		return nil
	case typesetAddress:
		return m.typesetAddressFound(v)
	case linkSaved:
		m.linkSavedNote(v)
		return nil
	case fetchResult:
		return m.fetchedFile(v)
	case partsResult:
		return m.assembled(v)
	case thumbResult:
		return m.thumbnailMade(v)
	case document.Result:
		return m.loaded(v)
	case tea.MouseClickMsg:
		// A click picks a cell; a second click on it does what Enter does.
		if m.sheet != nil && !m.help && m.screen == screenDocument && m.layer == layerNone && v.Button == tea.MouseLeft {
			again, ok := m.sheet.click(v.X, v.Y, m.width, m.bodyHeight())
			if ok && again && m.sheet.url() != "" {
				return m.open(m.sheet.url())
			}
			return nil
		}
	case tea.MouseWheelMsg:
		if m.typeset != nil && m.markdown == nil && !m.help && m.screen == screenDocument {
			// Scrolling gives up a Tab still waiting for more of the page,
			// as a key does.
			m.typesetTabPending, m.typesetAhead = 0, false
			if v.Button == tea.MouseWheelUp {
				cmd, _ := m.typesetKey("up")
				return cmd
			}
			if v.Button == tea.MouseWheelDown {
				cmd, _ := m.typesetKey("down")
				return cmd
			}
			return nil
		}
		if m.markdown != nil && !m.help && m.screen == screenDocument {
			if v.Button == tea.MouseWheelUp {
				m.markdown.scroll(-3)
			} else if v.Button == tea.MouseWheelDown {
				m.markdown.scroll(3)
			}
			return nil
		}
		if m.pictureMouse() {
			// What the terminal sent for the wheel before the picture took
			// the mouse.
			switch v.Button {
			case tea.MouseWheelUp:
				cmd, _ := m.key(tea.KeyPressMsg{Code: tea.KeyUp})
				return cmd
			case tea.MouseWheelDown:
				cmd, _ := m.key(tea.KeyPressMsg{Code: tea.KeyDown})
				return cmd
			}
			return nil
		}
		if m.sheet != nil && !m.help && m.screen == screenDocument {
			if v.Button == tea.MouseWheelUp {
				m.sheet.scroll(-3, m.width, m.bodyHeight()-1)
			} else if v.Button == tea.MouseWheelDown {
				m.sheet.scroll(3, m.width, m.bodyHeight()-1)
			}
			return nil
		}
	case tea.KeyPressMsg:
		// A key may move the page under the pointer: the tint goes until it
		// moves again.
		var unhover tea.Cmd
		if m.typesetHover != nil {
			m.typesetHover, unhover = nil, m.queueTypesetReflow()
		}
		if cmd, handled := m.key(v); handled || unhover != nil {
			return tea.Batch(unhover, cmd)
		}
	}
	var cmds []tea.Cmd
	if m.browsing() {
		cmds = append(cmds, m.browse(msg))
	}
	// Picture messages carry image IDs and sequence numbers, so late frames
	// from an earlier page cannot overwrite the current source.
	m.lastCellW, m.lastCellH = m.cellPixelSize()
	var picCmd tea.Cmd
	if nw, nh := m.cellPixelSize(); (cw != nw || ch != nh) && m.source != nil && m.kitty {
		// Font-size changes have the same placement lifetime as window resizes.
		if m.animation != nil {
			picCmd = m.resizeAnimation()
		} else {
			picCmd = m.refreshImage()
		}
	}
	cmds = append(cmds, picCmd)
	if nw, nh := m.cellPixelSize(); (cw != nw || ch != nh) && m.typeset != nil {
		cmds = append(cmds, m.queueTypesetReflow())
		if m.markdown == nil {
			cmds = append(cmds, m.refreshImage())
		}
	}
	if m.markdown != nil {
		if nw, nh := m.cellPixelSize(); cw != nw || ch != nh {
			cmds = append(cmds, m.layoutMarkdown())
		}
		cmds = append(cmds, m.markdown.setKitty(m.kitty))
	}
	_, mouseMessage := msg.(tea.MouseMsg)
	if m.chart != nil && !((m.help || m.listing()) && mouseMessage) {
		before := m.chart.Camera()
		_, cmd := m.chart.Update(meshMessage(msg))
		cmds = append(cmds, cmd)
		// The mesh widget orbits the camera toward horizontal drags. In a model
		// viewer, drag the object instead. Preserve picking, pan, and keys.
		if motion, ok := msg.(tea.MouseMotionMsg); ok && motion.Mod&tea.ModShift == 0 {
			after := m.chart.Camera()
			if after.Beta != before.Beta {
				after.Beta = 2*before.Beta - after.Beta
				cmds = append(cmds, m.chart.SetCamera(after))
			}
		}
	}
	if m.mapView != nil && !((m.help || m.listing()) && mouseMessage) {
		cmds = append(cmds, m.updateMap(msg))
	}
	if m.preview != nil && !mouseMessage {
		_, cmd := m.preview.Update(msg)
		cmds = append(cmds, cmd)
	}
	return tea.Batch(cmds...)
}

// loaded takes what the loader made of the current file onto the screen.
func (m *Model) loaded(v document.Result) tea.Cmd {
	if v.Generation != m.generation || m.suspended {
		return nil
	}
	stopped := m.stopAnimation()
	m.loading, m.err, m.fields = false, v.Err, v.Info
	if v.Err != nil {
		return tea.Batch(stopped, m.clearGraphics())
	}
	animationCleanup := tea.Batch(stopped, m.clearStandalone())
	if v.Image == nil || v.Animation != nil {
		animationCleanup = tea.Batch(animationCleanup, m.clearStill())
	}

	cleanup := tea.Batch(animationCleanup, m.clearQR(), m.clearMarkdown(), m.clearChart(), m.clearMap())
	m.sheet = nil
	m.typeset, m.typesetWidth = v.Typeset, v.TypesetWidth
	overlays := m.acceptTypesetView(v)
	if m.typeset != nil && m.savedFocus > 0 {
		if i := m.savedFocus - 1; i < len(m.typesetItems) {
			m.typesetFocus, m.typesetFocusIndex = m.typesetItems[i].Rects, i
		}
		m.savedFocus = 0
	}
	m.kind, m.page, m.pages = v.Kind, v.Page, v.Pages
	if v.Sheet != nil {
		m.source, m.sheet = nil, newSheetView(v.Sheet)
		m.sheet.setHidden(m.opts.Columns.Hidden(v.Sheet))
		if m.savedSheet != nil {
			m.sheet.setHidden(m.savedSheet.hidden)
			m.sheet.row, m.sheet.col, m.sheet.at = m.savedSheet.row, m.savedSheet.col, m.savedSheet.at
			m.sheet.settle(m.width, m.bodyHeight()-1)
			m.savedSheet = nil
		}
		return cleanup
	}
	if v.Markdown != nil {
		m.source = nil
		m.markdown = newMarkdownView(v.Markdown, m.terminal)
		if m.savedMarkdown != nil {
			m.markdown.raw, m.markdown.offset, m.markdown.left = m.savedMarkdown.raw, m.savedMarkdown.offset, m.savedMarkdown.left
			m.savedMarkdown = nil
		}
		laid := tea.Batch(m.markdown.setKitty(m.kitty), m.layoutMarkdown())
		if m.savedFocus > 0 {
			if items := m.markdown.items(); m.savedFocus <= len(items) {
				m.markdown.focus = &items[m.savedFocus-1]
			}
			m.savedFocus = 0
		}
		return tea.Sequence(cleanup, laid)
	}
	if v.Mesh != nil {
		return m.showMesh(v, cleanup)
	}
	if v.Video != nil && !m.isPreview {
		return m.showVideo(v, cleanup)
	}
	if v.Map != nil && !m.isPreview {
		return m.showMap(v, cleanup)
	}
	m.source = v.Image
	if v.Animation != nil && !m.isPreview {
		return tea.Sequence(cleanup, m.startAnimation(v.Animation))
	}
	return tea.Sequence(cleanup, tea.Batch(m.refreshImage(), overlays), m.queueTypesetReflow())
}

// showMesh puts a loaded mesh on a new chart, at the camera it starts from.
func (m *Model) showMesh(v document.Result, cleanup tea.Cmd) tea.Cmd {
	mode := nt3d.WebGPU
	switch m.opts.Render3D {
	case "software":
		mode = nt3d.Software
	case "wireframe":
		mode = nt3d.Wireframe
	}
	scene, err := v.Mesh.Scene()
	if err != nil {
		m.err = err
		return cleanup
	}
	m.chart = widget.New(scene, m.width, m.meshHeight(),
		widget.WithTerminalSession(m.terminal),
		widget.WithKittyMedium(ntpicture.KittyMediumSharedMemory),
		widget.WithGPUFailure(widget.FallBackOnGPUFailure),
		widget.WithRenderMode(mode), widget.WithBackground(m.background()))
	var setup []tea.Cmd
	m.mesh = v.Mesh
	m.parts, m.assemble, m.partsShown = v.Parts, v.Assemble, v.Shown
	m.keepLayer()
	// The start, and where f returns to, is fitted to this mesh: the
	// view asked for, or the default angles at a distance that shows
	// the whole of it, as an export would.
	d := nt3d.DefaultCamera()
	home := document.View{Alpha: d.Alpha, Beta: d.Beta, Projection: d.Projection}
	m.homeView = &home
	switch {
	case len(m.opts.Views) > 0:
		m.homeView = &m.opts.Views[0]
	case m.opts.STLCamera != nil:
		m.homeView = nil
	}
	if m.opts.STLCamera != nil {
		m.home = *m.opts.STLCamera
	}
	if m.homeView != nil {
		m.home = m.homeView.CameraFor(v.Mesh, m.frameAspect())
	}
	if m.savedCamera != nil {
		setup = append(setup, m.chart.SetCamera(*m.savedCamera))
		m.savedCamera = nil
	} else {
		setup = append(setup, m.chart.SetCamera(m.home))
	}
	// Apply a capability already established before this chart existed.
	_, applied := m.chart.Update(struct{}{})
	setup = append(setup, applied)
	m.triangles, m.points, m.err = v.Mesh.Triangles(), v.Mesh.Points(), m.chart.Err()
	return tea.Sequence(cleanup, tea.Batch(setup...), m.chart.Init())
}

func (m *Model) refreshImage() tea.Cmd {
	if m.source == nil {
		return nil
	}
	if m.typeset != nil && !m.isPreview {
		if !m.typesetMatches() {
			return m.queueTypesetReflow()
		}
		return m.showPicture(m.source)
	}
	span := 1 / float64(int(1)<<m.zoom)
	limit := (1 - span) / 2
	m.panX, m.panY = max(-limit, min(limit, m.panX)), max(-limit, min(limit, m.panY))
	if m.animation != nil {
		return m.syncStandalone()
	}
	return m.showPicture(crop(m.source, m.zoom, m.panX, m.panY))
}

// pictureBackground is behind a picture that does not fill its place.
var pictureBackground = color.RGBA{R: 24, G: 26, B: 30, A: 255}

// showPicture puts img on screen. With Kitty graphics it goes under a new
// image id, retaining the old placement until transmission completes.
// ghostty-web, the browser app's terminal,
// keeps a texture for each id and keeps it for a new picture of the same size
// that lands at the same place in its memory, so another page, file, or zoom
// would show the last one again.
func (m *Model) showPicture(img image.Image) tea.Cmd {
	if m.animation != nil {
		return m.syncStandalone()
	}
	if m.typesetLayers != nil {
		return m.syncTypesetLayers()
	}
	return m.showStill(img)
}

func crop(src image.Image, zoom int, panX, panY float64) image.Image {
	if zoom == 0 {
		return src
	}
	rect := cropRect(src.Bounds(), zoom, panX, panY)
	w, h, x, y := rect.Dx(), rect.Dy(), rect.Min.X, rect.Min.Y
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(out, out.Bounds(), src, image.Pt(x, y), draw.Src)
	return out
}

func cropRect(b image.Rectangle, zoom int, panX, panY float64) image.Rectangle {
	w, h := max(1, b.Dx()>>zoom), max(1, b.Dy()>>zoom)
	x := b.Min.X + int(float64(b.Dx()-w)/2+panX*float64(b.Dx()))
	y := b.Min.Y + int(float64(b.Dy()-h)/2+panY*float64(b.Dy()))
	x, y = max(b.Min.X, min(b.Max.X-w, x)), max(b.Min.Y, min(b.Max.Y-h, y))
	return image.Rect(x, y, x+w, y+h)
}

func safe(s string) string { return svg.SanitizeForTerminal(s) }
