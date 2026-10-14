package app

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"nimbleterminal.dev/gloss/internal/document"
)

// View draws the screen on show: its body, a layer over it where there is
// one, the status bar, and the keys that work. The help page and the
// quitting frame lie over any screen.
func (m *Model) View() tea.View {
	w, h := max(1, m.width), m.bodyHeight()
	body, mouse := m.body(w, h)
	if m.isPreview {
		// A preview is only the picture: the list beside it says the keys.
		v := tea.NewView(body)
		v.MouseMode = mouse
		return v
	}
	body = m.layered(body, w, h)
	bar, hint := m.statusLine(w, h)
	if m.scrubberVisible() {
		hint, mouse = m.scrubberView(), tea.MouseModeCellMotion
	}
	if m.search != nil && m.screen == screenDocument && !m.help && m.layer == layerNone {
		hint = ansi.Truncate(m.search.status(w), w, "…")
	}
	body = m.framed(body)
	palette := m.palette()
	hint = lipgloss.NewStyle().Foreground(palette.Muted).Background(palette.Background).Width(w).Render(hint)
	content := body + "\n" + bar + "\n" + hint
	if m.quitting {
		// Keys no longer answer. The frame keeps its height, or Bubble Tea
		// draws it below the last one; and it ends on an empty row, because
		// the last row is erased on the way out. The prompt lands there.
		content, mouse = body+"\n"+bar+"\n", tea.MouseModeNone
	}
	if m.height < 3 || (m.screen == screenDocument && m.chart != nil && m.height < 5) {
		content = ansi.Truncate("gloss: enlarge terminal", w, "")
	}
	v := tea.NewView(content)
	v.ReportFocus = m.opts.Theme == "" || m.opts.Theme == "auto"
	v.AltScreen, v.MouseMode = !m.opts.KeepScreen, mouse
	return v
}

// body is the screen's own content, w by h, and the mouse mode it wants.
func (m *Model) body(w, h int) (string, tea.MouseMode) {
	var body string
	mouse := tea.MouseModeNone
	plain := true  // Cut to the viewport; graphics keep their escapes whole.
	exact := false // The body is already w×h: a frame would measure every cell to pad nothing.
	switch {
	case m.openWithPicker != nil:
		body = m.openWithView(w, h)
	case m.help:
		body = m.helpView(w, h)
	case m.screen == screenBrowser:
		body = m.opener.view()
		if m.opener.find == nil {
			mouse = tea.MouseModeCellMotion // For clicks on crumbs and rows, and the wheel.
		}
	case len(m.opts.Files) == 0:
		body = m.emptyView(w)
	case m.listing():
		body, plain = m.menuView(), false
		if m.preview != nil && m.previewWidth() > 0 {
			mouse = m.preview.View().MouseMode
		}
		if m.screen == screenGrid {
			mouse = tea.MouseModeCellMotion
		}
	case m.loading && m.hasPictureFront():
		body, plain = m.pictureView(), false
		exact = !m.quitting && m.pictureExact(w, h)
	case m.loading:
		body = "Loading " + safe(filepath.Base(m.opts.Files[m.index])) + "…"
	case m.err != nil:
		body = "Cannot open file\n\n" + safe(m.err.Error()) + "\n\nR retry · ] next file · q quit"
	case m.chart != nil:
		v := m.chart.View()
		body, mouse, plain = m.meshFrame(v.Content, w), v.MouseMode, false
		// Kitty's grid already fills the widget's viewport. Measuring its
		// placeholder graphemes again to pad the same rectangle is expensive
		// on every camera update.
		exact = h >= 3 && m.meshKitty()
	case m.markdown != nil:
		body, mouse, plain = m.markdown.view(), tea.MouseModeCellMotion, false
	case m.sheet != nil:
		body, mouse, plain = m.sheet.view(w, h), tea.MouseModeCellMotion, false
	default:
		body, plain = m.pictureView(), false
		exact = !m.quitting && m.pictureExact(w, h)
		if m.typeset != nil {
			mouse = tea.MouseModeCellMotion
			if len(m.typesetLinks) > 0 && !m.isPreview && m.layer == layerNone {
				mouse = tea.MouseModeAllMotion
			}
		}
	}
	if plain {
		lines := strings.Split(body, "\n")
		for i := range lines {
			lines[i] = ansi.Truncate(lines[i], w, "")
		}
		body = strings.Join(lines[:min(len(lines), h)], "\n")
	}
	if exact {
		return m.spliceTypesetOverlays(body), mouse
	}
	frame := lipgloss.NewStyle().Width(w).Height(h)
	if plain || m.markdown != nil {
		p := m.palette()
		frame = frame.Foreground(p.Foreground).Background(p.Background)
	}
	if len(m.opts.Files) == 0 && !m.help && m.screen != screenBrowser {
		frame = frame.Align(lipgloss.Center, lipgloss.Center)
	}
	return frame.Render(body), mouse
}

// DocsURL is the documentation site.
const DocsURL = "https://nimbleterminal.github.io/gloss/docs/"

// emptyView is the drop target shown before any file, and what it opens.
func (m *Model) emptyView(w int) string {
	body := "Drop files here to open\n\nDrag them from a file manager, or paste their paths."
	if m.opts.FilesFS != nil {
		// The browser: files come from drops, or from the page around.
		body = "Drop files here to open\n\nDrag them from a file manager onto the terminal."
	}
	if m.canBrowse() {
		body = "Drop files here to open\n\nDrag them from a file manager, paste their paths, or press o to browse."
	}
	if m.opts.Pick {
		body = "Drop a file here to send it\n\nDrag it from a file manager, paste its path, or press o to browse."
	}
	// The name outranks the rest: as the screen shortens, the list of
	// formats goes first, then the words on what to do, and the name stays.
	h, lines := m.bodyHeight(), strings.Count(body, "\n")+1
	long := lipgloss.NewStyle().Width(lipgloss.Width(strings.Join(document.Formats, "\n"))).Render(strings.Join(document.Formats, "\n\n"))
	short := lipgloss.NewStyle().Width(w).Align(lipgloss.Center).Render(document.FormatsShort)
	name, gap := strings.Join(banner, "\n"), "\n\n\n"
	art := lipgloss.NewStyle().Foreground(m.palette().Accent).Bold(true).Render(name)
	room := func(extra int) bool { return h >= len(banner)+2+lines+extra }
	switch {
	case w < lipgloss.Width(name) || h < len(banner):
		// Too small to draw the name, so it is said, with what to do if
		// there is room to say it.
		if w >= len(tiny) {
			return tiny
		}
		return "GLOSS"
	case room(3 + strings.Count(long, "\n") + 1):
		return art + gap + body + gap + long
	case room(3 + 1):
		return art + gap + body + gap + short
	case room(0):
		return art + gap + body
	}
	return art
}

// tiny stands in for the whole screen where the banner will not fit.
const tiny = "GLOSS -- drag here"

// banner is the name, drawn small; its lines are of one width, so that
// centring the lot keeps them aligned.
var banner = []string{
	"  __     _   __  __ ",
	" /__ |  / \\ (_  (_  ",
	" \\_| |_ \\_/ __) __) ",
}

// layered lays the document's layer over its body, where the screen is a
// document's and the help is down.
func (m *Model) layered(body string, w, h int) string {
	if m.openWithPicker != nil || m.help || m.screen != screenDocument {
		return body
	}
	switch m.layer {
	case layerFont:
		return overlay(body, m.fontPickerView(), w, 0)
	case layerTheme:
		return overlay(body, m.themePickerView(w, h), w, 0)
	case layerQR:
		return overlay(body, m.qrView(w, h), w, 0)
	case layerInfo:
		// The mesh view keeps its own title on the first row.
		top := 0
		if m.chart != nil {
			top = 1
		}
		return overlay(body, m.infoBox(w, h-top), w, top)
	case layerColumns:
		if m.sheet != nil && m.sheet.picker != nil {
			return overlay(body, m.sheet.picker.view(m.sheet, w, h, m.palette()), w, 0)
		}
	case layerParts:
		if m.pickingParts() && m.partPicker != nil {
			return overlay(body, m.partsView(w, h-1), w, 1)
		}
	case layerColor:
		if m.pickingColor() && m.colorPicker != nil {
			return overlay(body, m.colorView(w, h-1), w, 1)
		}
	}
	return body
}

// statusLine is the status bar and the keys line for the screen on show.
func (m *Model) statusLine(w, h int) (bar, hint string) {
	switch {
	case m.openWithPicker != nil:
		return m.openWithStatus(w)
	case m.screen == screenBrowser && !m.help:
		return m.browserStatus(w)
	case m.listing():
		return m.listStatus(w)
	}
	return m.documentStatus(w, h)
}

func (m *Model) browserStatus(w int) (bar, hint string) {
	// The end of a long path says where you are; the start rarely does.
	dir, note := safe(m.opener.dir), " · by "+orderNames[m.sortBy]
	if m.note != "" {
		note += " · " + m.note
	}
	if over := ansi.StringWidth(" Open · "+dir+note) - w; over > 0 && over+1 < ansi.StringWidth(dir) {
		dir = "…" + ansi.TruncateLeft(dir, over+1, "")
	}
	bar = lipgloss.NewStyle().Width(w).Render(ansi.Truncate(" Open · "+dir+note, w, "…"))
	greyed, text := "hide greyed", "no text"
	if m.hideUnsupported {
		greyed = "show greyed"
	}
	if m.noTextFiles {
		text = "text"
	}
	hint = ansi.Truncate(" ?/F1 help · Enter open · ←/→ folders · / find · G go to · Tab complete · Ctrl-F types · Ctrl-L layout · Ctrl-G places · Ctrl-S sort · Ctrl-T "+greyed+" · Ctrl-X "+text+" · Alt-Enter open with · Esc cancel", w, "")
	if m.opener.going {
		hint = ansi.Truncate(" Type a folder's path · Tab completes folders · Enter go · Esc back", w, "")
	}
	if f := m.opener.find; f != nil {
		bar = lipgloss.NewStyle().Width(w).Render(ansi.Truncate(" Find · "+f.status(), w, "…"))
		hint = ansi.Truncate(" Enter search, then open · ↑/↓ select · Ctrl-A add all · Esc back", w, "")
	}
	return bar, hint
}

func (m *Model) listStatus(w int) (bar, hint string) {
	status := fmt.Sprintf(" Files · %d/%d selected · current %d", m.selection+1, len(m.opts.Files), m.index+1)
	hint = " ↑/↓ select · Enter open · t thumbnails · v preview · w/W apps · Esc cancel · q quit"
	if m.screen == screenGrid && m.grid != nil {
		status = fmt.Sprintf(" Files · grid · %d/%d selected · current %d", m.selection+1, len(m.opts.Files), m.index+1)
		if m.grid.pending() {
			status += " · making thumbnails"
		}
		hint = " ←→↑↓ select · Enter open · t list · Esc cancel · q quit"
	}
	if m.note != "" {
		status += " · " + m.note
	}
	return lipgloss.NewStyle().Width(w).Render(ansi.Truncate(status, w, "")), ansi.Truncate(hint, w, "")
}

// documentStatus names the file and says what is shown of it, and which
// keys work: the document's own, and those of the layer over it.
func (m *Model) documentStatus(w, h int) (bar, hint string) {
	empty := len(m.opts.Files) == 0
	name := "no files"
	if !empty {
		name = safe(filepath.Base(m.opts.Files[m.index]))
		if m.opts.IsStdin(m.opts.Files[m.index]) {
			name = "stdin"
		}
	}
	detail := m.detail(w, h)
	// A note comes before the detail: it is brief, and must not be cut.
	if m.note != "" {
		detail = m.note + " · " + detail
	}
	status := fmt.Sprintf(" %s  [%d/%d]  %s", name, m.index+1, len(m.opts.Files), detail)
	if empty {
		status = " No files yet"
		if m.note != "" {
			status += " · " + m.note
		}
	}
	if m.hoverLinkActive && m.typeset != nil {
		if address := m.linkAt(m.hoverLinkPoint.X, m.hoverLinkPoint.Y); address != "" {
			status = " Link: " + safe(address)
			if _, internal := document.FootnotePeer(address); internal {
				status = " Return to footnote reference"
				if strings.HasPrefix(address, "gloss:fn:") {
					status = " Jump to footnote"
				}
			}
		}
	}
	bar = m.statusStyle().Width(w).Render(ansi.Truncate(status, w, "…"))
	return bar, ansi.Truncate(m.hints(), w, "")
}

// detail says what is shown of the document. How things are drawn, the
// renderer and the transport, is the info box's to say.
func (m *Model) detail(w, h int) string {
	detail := fmt.Sprintf("%s · %dx", m.kind, 1<<m.zoom)
	if m.animation != nil {
		state := "playing"
		if m.animation.Paused {
			state = "paused"
		} else if m.animation.Finished {
			state = "finished"
		}
		if m.animation.seek != nil {
			state = fmt.Sprintf("seeking %d", m.animation.target)
		}
		detail += fmt.Sprintf(" · %g× speed · frame %d/%d · %s", m.animation.rate(), m.animation.Player.Frame(), m.animation.Player.Frames(), state)
	}
	if m.kind == "pdf" {
		detail += fmt.Sprintf(" · page %d/%d", m.page, m.pages)
	}
	switch {
	case m.chart != nil:
		detail = fmt.Sprintf("%s · %s", strings.ToUpper(m.kind), plural(m.triangles, "triangle"))
		if m.hasParts() && m.partsShown != nil {
			detail += fmt.Sprintf(" · parts %d/%d", m.partsOnScreen(), len(m.parts))
		}
		if m.tint != nil {
			detail += fmt.Sprintf(" · painted #%02x%02x%02x", m.tint.R, m.tint.G, m.tint.B)
		}
		if m.bg != nil {
			detail += fmt.Sprintf(" · background #%02x%02x%02x", m.bg.R, m.bg.G, m.bg.B)
		}
	case m.typeset != nil && m.markdown == nil:
		detail = map[string]string{"markdown": "Markdown", "text": "Text", "json": "JSON"}[m.kind] + " · typeset"
		if s := m.typesetShown; s != nil && s.Lines > 0 {
			detail += fmt.Sprintf(" · line %d/%d", s.Line, s.Lines)
		}
	case m.markdown != nil:
		mode := "rendered"
		if m.markdown.raw {
			mode = "source"
		}
		format := map[string]string{"docx": "Word", "json": "JSON", "ipynb": "notebook", "html": "HTML", "text": "text"}[m.kind]
		if format == "" {
			format = "Markdown"
		}
		if m.kind == "text" {
			// Source says its language, as the highlighter names it.
			for _, f := range m.fields {
				if f.Label == "Language" && f.Value != "" {
					format = f.Value
				}
			}
		}
		detail = fmt.Sprintf("%s · %s · %s", format, mode, m.markdown.lineDetail())
	case m.sheet != nil:
		detail = fmt.Sprintf("%s · %s", m.kind, m.sheet.status(w, h))
		switch m.kind {
		case "xlsx":
			detail = fmt.Sprintf("xlsx · sheet %d/%d · %s · %s", m.page, m.pages, safe(m.sheet.sheet.Name), m.sheet.status(w, h))
		case "grist":
			detail = fmt.Sprintf("grist · table %d/%d · %s · %s", m.page, m.pages, safe(m.sheet.sheet.Name), m.sheet.status(w, h))
		}
	case m.kind == "3mf":
		// Too large to draw, or a preview: the picture the file carries.
		detail = fmt.Sprintf("3MF · thumbnail · %dx", 1<<m.zoom)
	}
	return detail
}

// hints names the keys that work over the document.
func (m *Model) hints() string {
	if m.fontPicker != nil {
		return " ↑/↓ fonts · Enter keep · Esc cancel · +/- zoom"
	}
	if m.themePicker != nil {
		return " ↑/↓ preview theme · Enter keep · Esc cancel"
	}
	if m.lineInput != nil {
		return m.lineInput.status(m.width)
	}
	if len(m.opts.Files) == 0 {
		keys := " q quit · ? help · i info"
		if m.canBrowse() {
			keys += " · o browse"
		}
		return keys
	}
	// A layer takes the keys while it is up.
	switch m.layer {
	case layerTheme:
		return " ↑/↓ preview theme · Enter keep · Esc cancel"
	case layerQR:
		return " Esc/u close QR · g graphics · e export QR · q quit"
	case layerColumns:
		return " ↑/↓ select · Space show/hide · a all · n none · Esc close"
	case layerParts:
		return " ↑/↓ select · Space show/hide · Enter focus · a all · n only · X all and close · Esc close"
	case layerColor:
		return " Tab mode · ↑/↓ ←/→ choose · Space apply · r file's colors · Enter keep · Esc undo"
	}
	keys := " q quit · ? help · m files · [/] files · n/p pages · +/- zoom · e export · i info"
	switch {
	case m.typeset != nil && m.markdown == nil:
		keys = " q quit · ↑/↓/wheel scroll · Space/b screen · +/- zoom · f fit width · s source · t text · e export"
		if m.typesetLined() {
			keys += " · : line · # numbers · S " + map[bool]string{false: "chop", true: "wrap"}[m.typesetShown.Chop]
		}
		if m.typesetSelected != "" {
			keys = " y copy selection · Esc clear · ↑/↓/wheel scroll · +/- zoom · e export · q quit"
		}
	case m.typeset != nil:
		keys = " q quit · m files · ↑/↓ scroll · Space/b scroll · s typeset view · t text"
	case m.animation != nil:
		keys = " Space pause/play · </> speed · Backspace 1× · +/- zoom · e export frame · ? help · q quit"
	case m.markdown != nil:
		keys = " q quit · m files · ↑/↓ scroll · Space/b page · s source · g graphics"
		if m.canTypeset() {
			keys += " · t typeset"
		}
		if m.markdown.numbered() {
			keys += " · : line · # numbers"
		}
		keys += " · S " + map[bool]string{false: "chop", true: "wrap"}[m.markdown.chop]
	case m.sheet != nil:
		keys = " q quit · m files · ↑/↓ ←/→ move · Space/b page · n/p sheets · c columns · i info"
		switch {
		case m.kind == "grist":
			keys = " q quit · m files · ↑/↓ ←/→ move · Space/b page · n/p tables · c columns · i info"
		case !m.booked():
			keys = " q quit · m files · ↑/↓ ←/→ move · Space/b page · n/p files · c columns · i info"
		}
		if m.sheet.url() != "" && m.opts.Fetch != nil {
			keys = " Enter open address ·" + strings.TrimPrefix(keys, " q quit ·")
		}
		if m.sheet.url() != "" {
			keys = " u QR of address ·" + strings.TrimPrefix(keys, " q quit ·")
		}
	case m.chart != nil && m.mesh != nil:
		keys = " q quit · ? help · m files · [/] files · e export · i info · C color · B bg-color"
		if m.hasParts() {
			keys += " · c parts"
		}
	}
	if m.isFetched() {
		keys = " Esc close ·" + strings.TrimPrefix(keys, " q quit ·")
	}
	if m.searchable() {
		keys = " / search ·" + keys
	}
	if m.canBrowse() {
		keys += " · o browse"
	}
	if what := m.picking(); what != "" {
		// Say what will be sent before it is.
		keys = " Enter send " + what + " · q cancel ·" + strings.TrimPrefix(keys, " q quit ·")
	}
	return keys
}
