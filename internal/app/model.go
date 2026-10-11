package app

import (
	"context"
	"fmt"
	"github.com/NimbleTerminal/gloss/internal/animate"
	"image"
	"image/color"
	"image/draw"
	"io/fs"
	"math/rand/v2"
	"os"
	"sync/atomic"

	tea "charm.land/bubbletea/v2"
	"github.com/NimbleMarkets/ntcharts-svg/svg"
	"github.com/NimbleMarkets/ntcharts/v2/picture"
	charts "github.com/NimbleMarkets/ntcharts3d"
	"github.com/NimbleTerminal/gloss/internal/document"
	"github.com/NimbleTerminal/gloss/internal/theme"
	"github.com/charmbracelet/x/ansi"
)

type Options struct {
	// OpenLink asks the embedding host to open an explicitly clicked web URL.
	// Nil leaves link activation to the terminal (OSC 8). It never fetches a document.
	OpenLink               func(string) error
	STLCamera              *charts.Camera  // Optional initial/reset view for an embedded gallery.
	Views                  []document.View // Views of a mesh asked for: the first is where the viewer starts.
	FilesFS                fs.FS           // Optional embedded files for the browser demo.
	Files                  []string
	Stdin                  string // The file standard input was read into, if one of Files is it.
	Type, Render, Render3D string
	MarkdownMode           string
	Typeset                bool
	Theme                  string
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
	// Columns picks the columns of every sheet to show; the rest are hidden.
	Columns document.ColumnFilter
	// Parts picks the parts of every 3MF to show; the rest are left out.
	Parts document.PartFilter
	// Color paints the faces of a mesh that its file left plain.
	Color *color.RGBA
	// Background is the color behind a mesh; nil keeps the default.
	Background *color.RGBA
	keptScreen bool // Set once picture numbers have been moved; previews inherit it.
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
	chart       *charts.Model
	chartID     int
	triangles   int
	parts       []document.Part // What a 3MF build places, when a mesh is made of parts.
	assemble    func([]bool) (*document.Mesh, error)
	partsShown  []bool // Which parts are on the chart; nil for all.
	partPicker  *partPicker
	mesh        *document.Mesh // The mesh on the chart.
	tint        *color.RGBA    // Paint given to the mesh, if any.
	bg          *color.RGBA    // Background chosen behind the mesh, if any.
	tintAll     bool           // On every face, not only the plain ones.
	colorPicker *colorPicker
	savedCamera *charts.Camera
	home        charts.Camera  // Where the camera starts, and returns on reset.
	homeView    *document.View // The view the home is fitted from, when it is a fit.
}

// textState is what the model keeps for a Markdown or sheet document, and the
// view to return to when a fetched file closes.
type textState struct {
	typesetLinks            []document.TypesetLink
	typesetSelect           *document.TypesetSelection // The text selected, or being selected, on the typeset page.
	typesetSelectShown      *document.TypesetSelection // The selection the shown picture highlights.
	typesetSelected         string                     // The shown selection's text.
	typesetDragging         bool                       // A left button is down on the typeset page.
	typesetDragFrom         image.Point                // Where it went down, on the page.
	pressedLink             string
	hoverLinkPoint          image.Point
	hoverLinkActive         bool
	linkPoint               image.Point
	typesetPlayback         *animate.CompositeSource
	typesetOverlays         []*typesetOverlay
	typesetPending          *document.Result // A page view whose overlays wait for the view to settle.
	typesetSettle           uint64
	search                  *textSearch
	searchWorker            *searchWorker
	markdown                *markdownView
	typeset                 *document.Markdown
	typesetWidth            int
	typesetRevision         uint64
	typesetActive           uint64
	typesetWaitSize         bool
	typesetCancel           context.CancelFunc
	typesetTop, typesetLeft int
	typesetEnd              bool
	typesetShown            *document.TypesetViewport
	typesetTheme            string
	typesetFont             string
	savedMarkdown           *markdownView
	sheet                   *sheetView
	savedSheet              *sheetView // The cell to return to when a fetched file closes.
}

// browserState is what the file browser remembers between visits.
type browserState struct {
	opener          *opener
	hideUnsupported bool // The browser's choice outlasts any one visit.
	sortBy          int  // The order the browser lists in; kept likewise.
	thumbs          bool // The list is shown as a grid of thumbnails, last time and next.
	grid            *grid
	noTextFiles     bool     // The browser sets text files aside; kept likewise.
	browseLayout    int      // How the browser lays folders out (a browse.Layout); kept likewise.
	browseTypes     []string // The kinds of file the browser shows alone, if any; kept likewise.
}

// Model is the pager. A preview pane is itself a Model (isPreview set) that
// the list owns (see updatePreview): built with Menu and Preview off, so it
// has no preview of its own, it takes no drops or mouse input, and it loads
// with Request.Preview so the loader keeps it small.
type Model struct {
	meshState
	textState
	browserState

	opts                     Options
	loader                   *document.Loader
	pic                      picture.Model
	qr                       *qrOverlay
	width, height, index     int
	page, pages              int
	generation               uint64
	kind                     string
	loading, help, autoKitty bool
	theme                    string
	detectedTheme            string
	themePicker              *themePicker
	fontPicker               *fontPicker
	err                      error
	source                   image.Image
	animation                *animation
	presentation             *animate.Animator
	zoom                     int
	panX, panY               float64
	screen                   screen // What fills the body.
	layer                    layer  // What floats over a document.
	selection                int
	preview                  *Model
	isPreview                bool
	kittyID                  int
	picID                    int // The Kitty image id the document picture goes under now.
	previewDrag              bool
	suspended                bool
	note                     string // Outcome of the last drop, shown until the next key.
	fields                   []document.Field
	quitting                 bool     // The view being drawn is the one left behind.
	added                    []string // What the user has handed over, by full path.
	fetching                 bool
	fetched                  []fetchedDoc // Files fetched from the web this session.
	picked                   []string
	skipped                  []string // Reported on stderr once the terminal is restored.
}

var nextModelID atomic.Int64

// nextKittyID gives the next number for pictures to be transmitted under.
// Each is a thousand apart, so a model's pages and frames have room, and
// placeholder cells carry the number in a 24-bit color.
func nextKittyID() int { return 100 + int(nextModelID.Add(1))*1000 }

func New(opts Options) *Model {
	pic, id := terminalPicture(opts.Render, !opts.keptScreen)
	opts.keptScreen = true
	m := &Model{opts: opts, meshState: meshState{tint: opts.Color, bg: opts.Background}, loader: &document.Loader{Files: opts.FilesFS}, kittyID: id, picID: id, pic: pic, page: opts.Page, pages: 1, autoKitty: opts.Render != "glyph"}
	m.presentation = animate.New(&m.pic, animate.Config{NextID: nextKittyID, Background: pictureBackground})
	m.presentation.ID = id
	m.theme = "dark"
	m.detectedTheme = "dark"
	m.typesetFont = document.FontKey(opts.Font, opts.Mono)
	if opts.Theme != "" && opts.Theme != "auto" {
		m.theme = theme.Resolve(opts.Theme)
	}
	if (opts.Menu || opts.Preview) && len(opts.Files) > 0 {
		m.screen = screenList
	}
	return m
}

// terminalPicture is shared by the pager and the handoff screen. Capability
// detection and geometry replies always run through picture's Tea commands.
func terminalPicture(render string, fresh bool) (picture.Model, int) {
	switch render {
	case "kitty":
		picture.ForceKittyCapability(picture.KittyCapabilitySupported)
	case "glyph":
		picture.ForceKittyCapability(picture.KittyCapabilityUnsupported)
	}
	if fresh {
		// Pictures left in the scrollback, by -X or by Q, are owned by the
		// terminal, under their numbers. Every run starts somewhere new, so
		// that the next gloss does not draw over what the last one left.
		nextModelID.Store(rand.Int64N(8000))
	}
	id := nextKittyID()
	// Frames go as zlib-compressed raw pixels: PNG's encoder tries every
	// filter on every row and took three times as long for the same bytes.
	pic := picture.NewWithConfig(picture.Config{KittyID: id, KittyZ: -1, Background: pictureBackground, KittyFormat: picture.KittyFormatZlib})
	if render == "kitty" {
		pic.Toggle()
	}
	return pic, id
}

func updateTerminalPicture(pic *picture.Model, autoKitty bool, msg tea.Msg) tea.Cmd {
	cmd := pic.Update(msg)
	if autoKitty && pic.KittySupported() == picture.KittyCapabilitySupported && pic.Mode() != picture.PictureKitty {
		return tea.Batch(cmd, pic.Toggle())
	}
	return cmd
}

func (m *Model) Init() tea.Cmd {
	var browse tea.Cmd
	if m.opts.Browse != "" {
		browse = m.browseFrom(m.opts.Browse)
	}
	if m.screen == screenList {
		m.suspended = true
		return tea.Batch(m.themeRequest(), m.pic.Init(), m.updatePreview(), browse, m.awaitDrops())
	}
	return tea.Batch(m.themeRequest(), m.pic.Init(), m.load(false), browse, m.awaitDrops())
}

func (m *Model) Close() error {
	m.stopTypeset()
	m.clearFontPicker()
	m.clearSearch()
	m.stopAnimation()
	m.clearAnimationFront()
	m.retireAnimationPicture(m.pic.SetImage(nil))
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
	return m.loader.Close()
}

func (m *Model) Err() error { return m.err }

// quit gives the terminal back. Graphics are taken down with the alternate
// screen. On the main screen the document is left as it was last seen.
func (m *Model) quit() tea.Cmd {
	if m.opts.OnQuit != nil {
		m.opts.OnQuit()
	}
	m.clearSearch()
	m.stopAnimation()
	qrCleanup := tea.Batch(m.clearQR(), m.clearFontPicker())
	if !m.opts.KeepScreen {
		return tea.Sequence(tea.Batch(qrCleanup, m.clearGraphics(), m.disposePreview()), tea.Quit)
	}
	m.quitting, m.help = true, false
	if m.browsing() {
		m.showDocument()
	}
	var markdownCleanup tea.Cmd
	if m.markdown != nil {
		markdownCleanup = m.markdown.keep()
	}
	return tea.Sequence(tea.Batch(qrCleanup, markdownCleanup, m.keepTypesetOverlays(), m.keepAnimationPicture()), tea.Quit)
}

// IsStdin says whether path is the file standard input was read into.
func (o Options) IsStdin(path string) bool { return o.Stdin != "" && path == o.Stdin }

// request is what the loader is asked for the current file and page.
func (m *Model) request(reload bool) document.Request {
	q := document.Request{Path: m.opts.Files[m.index], Type: m.opts.Type, MarkdownMode: m.opts.MarkdownMode, Typeset: m.opts.Typeset, Theme: m.theme, Font: m.opts.Font, Mono: m.opts.Mono, Page: m.page, DPI: m.opts.DPI, Generation: m.generation, Reload: reload, Preview: m.isPreview, Animate: !m.isPreview,
		Stdin: m.opts.IsStdin(m.opts.Files[m.index]), Parts: m.opts.Parts, Shown: m.partsShown, Color: m.tint, PaintAll: m.tintAll}
	q.TypesetWidth = m.typesetMeasure()
	if !reload {
		q.Playback = m.typesetPlayback
		if sources := m.typesetOverlaySources(); q.Playback != nil && len(sources) > 0 {
			q.Playback = q.Playback.WithPositions(sources...)
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
	m.stopAnimation()
	m.generation++
	m.loading, m.err = true, nil
	q := m.request(reload)
	m.stopTypeset()
	q.Context, m.typesetCancel = context.WithCancel(context.Background())
	preview := m.isPreview
	return func() tea.Msg {
		r := m.loader.Load(q)
		if preview {
			return previewResult{owner: m.kittyID, result: r}
		}
		return r
	}
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
func (m *Model) show(i int) tea.Cmd {
	m.clearSearch()
	m.index, m.page, m.pages, m.kind = i, 1, 1, ""
	m.source, m.zoom, m.panX, m.panY = nil, 0, 0, 0
	m.savedCamera, m.savedMarkdown, m.fields = nil, nil, nil
	m.partsShown = nil // A choice of parts is for one file.
	return tea.Sequence(m.clearGraphics(), m.load(false))
}

// Close releases renderer resources; Kitty images must also be explicitly
// deleted while the terminal is still running.
func (m *Model) clearChart() tea.Cmd {
	if m.chart == nil {
		return nil
	}
	kitty := m.chart.PictureMode() == picture.PictureKitty
	_ = m.chart.Close()
	m.chart = nil
	if !kitty {
		return nil
	}
	seq := fmt.Sprintf("\x1b_Ga=d,d=I,i=%d,q=2\x1b\\", m.chartID)
	if os.Getenv("TMUX") != "" {
		seq = ansi.TmuxPassthrough(seq)
	}
	return tea.Raw(seq)
}

func (m *Model) clearGraphics() tea.Cmd {
	m.stopTypeset()
	m.typesetTop, m.typesetLeft, m.typesetEnd, m.typesetShown = 0, 0, false, nil
	m.typeset = nil
	m.typesetLinks = nil
	m.clearTypesetSelection()
	m.typesetSelectShown = nil
	m.pressedLink = ""
	m.hoverLinkActive = false
	m.typesetPlayback = nil
	m.typesetPending = nil
	overlayCleanup := m.stopTypesetOverlays()
	m.stopAnimation()
	m.sheet = nil
	frontCleanup := m.clearAnimationFront()
	cleanup := m.pic.SetImage(nil)
	m.retireAnimationPicture(cleanup)
	return tea.Batch(m.clearFontPicker(), frontCleanup, overlayCleanup, m.clearQR(), m.clearChart(), m.clearMarkdown(), cleanup)
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
	cw, ch := m.pic.CellPixelSize()
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

func (m *Model) Update(msg tea.Msg) (model tea.Model, cmd tea.Cmd) {
	defer func() {
		cmd = tea.Batch(cmd, m.syncSearch())
		if m.pic.Mode() != picture.PictureKitty {
			cmd = tea.Batch(cmd, m.clearAnimationFront())
		}
		if m.typeset == nil || m.markdown != nil || m.pic.Mode() != picture.PictureKitty {
			m.typesetPending = nil
			cmd = tea.Batch(cmd, m.stopTypesetOverlays())
		}
		cmd = tea.Batch(cmd, m.scheduleAnimation(), m.scheduleTypesetOverlays())
		if m.markdown != nil {
			cmd = tea.Batch(cmd, m.markdown.scheduleAnimations(!m.isPreview && !m.loading && !m.suspended && !m.quitting && !m.help && m.screen == screenDocument))
		}
		if p := m.fontPicker; p != nil {
			cmd = tea.Batch(cmd, p.pic.Update(msg), m.layoutFontPicker())
		}
		if m.qr != nil {
			cw, ch := m.pic.CellPixelSize()
			cmd = tea.Batch(cmd, m.qr.model.SetTerminal(m.pic.Mode() == picture.PictureKitty, cw, ch), m.layoutQR(), m.qr.model.Update(msg))
		}
	}()
	if _, mouse := msg.(tea.MouseMsg); mouse && (m.qr != nil || m.themePicker != nil || m.fontPicker != nil) {
		return m, nil
	}
	if mouse, ok := msg.(tea.MouseMsg); ok {
		if cmd, took := m.scrubberMouse(mouse); took {
			return m, cmd
		}
	}
	if m.browsing() {
		if _, ok := msg.(tea.MouseMsg); ok {
			if m.help || m.opener.find != nil {
				return m, nil
			}
			return m, m.browse(m.onBody(msg))
		}
	}
	msg = m.onBody(msg)
	selectCmd, selecting := m.typesetSelectMouse(msg)
	if cmd, handled := m.linkMouse(msg); handled || selecting {
		return m, tea.Batch(cmd, selectCmd)
	}
	if mouse, ok := msg.(tea.MouseMsg); ok && m.listing() {
		if m.help {
			return m, nil
		}
		if m.screen == screenGrid && m.grid != nil {
			return m, m.gridMouse(mouse)
		}
		return m, m.previewMouse(mouse)
	}
	if mouse, ok := msg.(tea.MouseMsg); ok && m.pickingColor() {
		if cmd, took := m.colorMouse(mouse); took {
			if m.colorPicker == nil {
				m.layer = layerNone
			}
			return m, cmd
		}
	}
	if m.markdown != nil {
		if cmd, handled := m.markdown.animationMessage(msg); handled {
			return m, cmd
		}
	}
	if cmd, handled := m.typesetOverlayMessage(msg); handled {
		return m, cmd
	}
	switch v := msg.(type) {
	case fontPreviewResult:
		if p := m.fontPicker; p != nil && p.owner == v.owner && p.revision == v.revision {
			p.base, p.err = v.image, v.err
			return m, m.showFontPreview()
		}
		return m, nil
	case fontSaved:
		if v.err != nil {
			m.note = "save font: " + safe(v.err.Error())
		}
		return m, nil
	case themeSaved:
		if v.err != nil {
			m.note = "save theme: " + safe(v.err.Error())
		}
		return m, nil
	case linkOpened:
		if v.generation == m.generation && v.err != nil {
			m.note = "open link: " + safe(v.err.Error())
		}
		return m, nil
	case tea.BackgroundColorMsg:
		if v.Color == nil {
			return m, nil
		}
		name := "light"
		if v.IsDark() {
			name = "dark"
		}
		m.detectedTheme = name
		if m.themePicker != nil || m.opts.Theme != "" && m.opts.Theme != "auto" {
			return m, nil
		}
		return m, m.applyTheme(name)
	case tea.FocusMsg:
		return m, m.themeRequest()
	case searchResult:
		return m, m.searched(v)
	case animationTick:
		return m, m.animationTick(v)
	case animationFrame:
		return m, m.animationFrame(v)
	case animationSeekResult:
		return m, m.animationSought(v)
	case animationPresented:
		cmd, _ := m.presentation.Update(v)
		return m, cmd
	case previewResult:
		if m.preview != nil && v.owner == m.preview.kittyID {
			_, cmd := m.preview.Update(v.result)
			return m, cmd
		}
		return m, nil
	case typesetResize:
		return m, m.reflowTypeset(v)
	case typesetSettled:
		return m, m.typesetSettled(v)
	case typesetReflowResult:
		return m, m.typesetReflowed(v)
	case tea.WindowSizeMsg:
		oldWidth, oldHeight := m.width, m.height
		m.width, m.height = max(1, v.Width), max(1, v.Height)
		m.opener.resize(m.width, m.bodyHeight())
		var resize tea.Cmd
		if m.source != nil && m.pic.Mode() == picture.PictureKitty {
			if oldWidth != m.width || oldHeight != m.height {
				if m.animation != nil {
					resize = m.resizeAnimation()
				} else {
					resize = m.refreshImage()
				}
			}
		} else {
			resize = m.pic.SetSize(m.width, m.bodyHeight())
		}
		cmd := tea.Batch(resize, m.resizePreview(), m.layoutMarkdown(), m.layoutGrid())
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
			return m, tea.Batch(cmd, m.chart.SetSize(m.width, m.bodyHeight()), m.refit())
		}
		return m, cmd
	case tea.PasteMsg:
		if m.search != nil && m.search.editing {
			m.search.insert(v.Content)
			return m, nil
		}
		// Terminals deliver dropped files as a bracketed paste of their paths.
		if m.isPreview || m.opts.FilesFS != nil {
			return m, nil
		}
		if address := document.DropURL(v.Content); address != "" {
			return m, m.open(address)
		}
		return m, m.probeDrop(document.ParseDrop(v.Content))
	case DropMsg:
		if m.isPreview {
			return m, nil
		}
		return m, m.probeDrop(v.Paths)
	case outsideDrop:
		return m, tea.Batch(m.probeDrop(v.paths), m.awaitDrops())
	case dropResult:
		return m, m.addDropped(v)
	case exportResult:
		m.note = v.note()
		return m, nil
	case openResult:
		return m, m.opened(v)
	case findResult:
		if m.browsing() && m.opener.find != nil {
			m.opener.find.answered(v)
		}
		return m, nil
	case fetchResult:
		return m, m.fetchedFile(v)
	case partsResult:
		return m, m.assembled(v)
	case thumbResult:
		return m, m.thumbnailMade(v)
	case document.Result:
		return m, m.loaded(v)
	case tea.MouseWheelMsg:
		if m.typeset != nil && m.markdown == nil && !m.help && m.screen == screenDocument {
			if v.Button == tea.MouseWheelUp {
				cmd, _ := m.typesetKey("up")
				return m, cmd
			}
			if v.Button == tea.MouseWheelDown {
				cmd, _ := m.typesetKey("down")
				return m, cmd
			}
			return m, nil
		}
		if m.markdown != nil && !m.help && m.screen == screenDocument {
			if v.Button == tea.MouseWheelUp {
				m.markdown.scroll(-3)
			} else if v.Button == tea.MouseWheelDown {
				m.markdown.scroll(3)
			}
			return m, nil
		}
		if m.sheet != nil && !m.help && m.screen == screenDocument {
			if v.Button == tea.MouseWheelUp {
				m.sheet.scroll(-3, m.width, m.bodyHeight()-1)
			} else if v.Button == tea.MouseWheelDown {
				m.sheet.scroll(3, m.width, m.bodyHeight()-1)
			}
			return m, nil
		}
	case tea.KeyPressMsg:
		if cmd, handled := m.key(v); handled {
			return m, cmd
		}
	}
	var cmds []tea.Cmd
	if m.browsing() {
		cmds = append(cmds, m.browse(msg))
	}
	// Picture messages carry image IDs and sequence numbers, so late frames
	// from an earlier page cannot overwrite the current source.
	cw, ch := m.pic.CellPixelSize()
	picCmd := updateTerminalPicture(&m.pic, m.autoKitty, msg)
	if nw, nh := m.pic.CellPixelSize(); (cw != nw || ch != nh) && m.source != nil && m.pic.Mode() == picture.PictureKitty {
		// Font-size changes have the same placement lifetime as window resizes.
		if m.animation != nil {
			picCmd = m.resizeAnimation()
		} else {
			picCmd = m.refreshImage()
		}
	}
	cmds = append(cmds, m.animationTransmission(msg, picCmd))
	if nw, nh := m.pic.CellPixelSize(); (cw != nw || ch != nh) && m.typeset != nil {
		cmds = append(cmds, m.queueTypesetReflow())
		if m.markdown == nil {
			cmds = append(cmds, m.refreshImage())
		}
	}
	if m.markdown != nil {
		cmds = append(cmds, m.markdown.update(msg), m.markdown.setKitty(m.pic.Mode() == picture.PictureKitty))
	}
	cmds = append(cmds, m.updateTypesetOverlayPictures(msg))
	_, mouseMessage := msg.(tea.MouseMsg)
	if m.chart != nil && !((m.help || m.listing()) && mouseMessage) {
		before := m.chart.Camera()
		_, cmd := m.chart.Update(msg)
		cmds = append(cmds, cmd)
		// NTCharts orbits the camera toward horizontal drags. In a model
		// viewer, drag the object instead. Preserve picking, pan, and keys.
		if motion, ok := msg.(tea.MouseMotionMsg); ok && motion.Mod&tea.ModShift == 0 {
			after := m.chart.Camera()
			if after.Beta != before.Beta {
				after.Beta = 2*before.Beta - after.Beta
				cmds = append(cmds, m.chart.SetCamera(after))
			}
		}
	}
	if m.preview != nil && !mouseMessage {
		_, cmd := m.preview.Update(msg)
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

// loaded takes what the loader made of the current file onto the screen.
func (m *Model) loaded(v document.Result) tea.Cmd {
	if v.Generation != m.generation || m.suspended {
		return nil
	}
	m.stopAnimation()
	m.loading, m.err, m.fields = false, v.Err, v.Info
	if v.Err != nil {
		return m.clearGraphics()
	}
	var animationCleanup tea.Cmd
	if v.Image == nil || v.Animation != nil {
		animationCleanup = m.clearAnimationFront()
	}
	if m.presentation.Flight != nil && (m.presentation.Front == nil || m.presentation.Front.ID != m.picID) {
		pendingCleanup := m.pic.SetImage(nil)
		m.retireAnimationPicture(pendingCleanup)
		animationCleanup = tea.Batch(animationCleanup, pendingCleanup)
	}
	cleanup := tea.Batch(animationCleanup, m.clearQR(), m.clearMarkdown(), m.clearChart())
	m.sheet = nil
	m.typeset, m.typesetWidth = v.Typeset, v.TypesetWidth
	overlays := m.acceptTypesetView(v)
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
		return tea.Sequence(cleanup, m.pic.SetImage(nil))
	}
	if v.Markdown != nil {
		m.source = nil
		m.markdown = newMarkdownView(v.Markdown, nextKittyID())
		if m.savedMarkdown != nil {
			m.markdown.raw, m.markdown.offset = m.savedMarkdown.raw, m.savedMarkdown.offset
			m.savedMarkdown = nil
		}
		return tea.Sequence(tea.Batch(cleanup, m.pic.SetImage(nil)), tea.Batch(m.markdown.setKitty(m.pic.Mode() == picture.PictureKitty), m.layoutMarkdown()))
	}
	if v.Mesh != nil {
		return m.showMesh(v, cleanup)
	}
	m.source = v.Image
	if v.Animation != nil && !m.isPreview {
		m.presentation.Speed = 0
		m.presentation.SetPlayer(v.Animation)
		m.presentation.Paused = false
		m.animation = &animation{Animator: m.presentation}
	}
	return tea.Sequence(cleanup, tea.Batch(m.refreshImage(), overlays), m.queueTypesetReflow())
}

// showMesh puts a loaded mesh on a new chart, at the camera it starts from.
func (m *Model) showMesh(v document.Result, cleanup tea.Cmd) tea.Cmd {
	mode := charts.WebGPU
	switch m.opts.Render3D {
	case "software":
		mode = charts.Software
	case "wireframe":
		mode = charts.Wireframe
	}
	m.chartID = nextKittyID()
	m.chart = charts.New(m.width, m.bodyHeight(), charts.WithKittyID(m.chartID), charts.WithAutoRotate(false), charts.WithRenderMode(mode), charts.WithBackground(m.background()))
	// The chart draws only through the commands it returns, so each
	// is kept, whether or not it has anything to say before Init.
	var setup []tea.Cmd
	setup = append(setup, m.chart.SetAxes(charts.Axes{X: charts.Axis{Hidden: true}, Y: charts.Axis{Hidden: true}, Z: charts.Axis{Hidden: true}}))
	setup = append(setup, m.chart.SetColorLegendVisible(false), m.chart.SetSeries(v.Mesh))
	m.mesh = v.Mesh
	m.parts, m.assemble, m.partsShown = v.Parts, v.Assemble, v.Shown
	m.keepLayer()
	// The start, and where f returns to, is fitted to this mesh: the
	// view asked for, or the default angles at a distance that shows
	// the whole of it, as an export would.
	d := charts.DefaultCamera()
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
	m.triangles, m.err = v.Mesh.Triangles(), m.chart.Err()
	return tea.Sequence(tea.Batch(cleanup, m.pic.SetImage(nil)), tea.Batch(setup...), m.chart.Init())
}

func (m *Model) refreshImage() tea.Cmd {
	if m.source == nil {
		return nil
	}
	if m.typeset != nil && !m.isPreview {
		if !m.typesetMatches() {
			return m.queueTypesetReflow()
		}
		if m.presentation.Source == nil && m.typesetPlayback != nil && len(m.typesetOverlays) == 0 && m.typesetPending == nil {
			m.presentation.SetSource(m.typesetPlayback)
		}
		return m.showPicture(m.source)
	}
	span := 1 / float64(int(1)<<m.zoom)
	limit := (1 - span) / 2
	m.panX, m.panY = max(-limit, min(limit, m.panX)), max(-limit, min(limit, m.panY))
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
	m.presentation.SetSize(m.width, m.bodyHeight())
	m.presentation.SetBackground(m.pictureBackground())
	zoom, panX, panY := m.zoom, m.panX, m.panY
	m.presentation.Transform = func(frame image.Image) image.Image { return crop(frame, zoom, panX, panY) }
	if m.typeset != nil {
		m.presentation.Transform = nil
	}
	cmd := m.presentation.Show(img)
	m.picID = m.presentation.ID
	return cmd
}

func crop(src image.Image, zoom int, panX, panY float64) image.Image {
	if zoom == 0 {
		return src
	}
	b := src.Bounds()
	w, h := max(1, b.Dx()>>zoom), max(1, b.Dy()>>zoom)
	x := b.Min.X + int(float64(b.Dx()-w)/2+panX*float64(b.Dx()))
	y := b.Min.Y + int(float64(b.Dy()-h)/2+panY*float64(b.Dy()))
	x, y = max(b.Min.X, min(b.Max.X-w, x)), max(b.Min.Y, min(b.Max.Y-h, y))
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(out, out.Bounds(), src, image.Pt(x, y), draw.Src)
	return out
}

func safe(s string) string { return svg.SanitizeForTerminal(s) }
