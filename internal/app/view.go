package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"nimbleterminal.dev/gloss/internal/document"
	"nimbleterminal.dev/gloss/internal/i18n"
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
	if m.openWithPicker != nil {
		// The chooser lies over the screen it was opened from, which keeps
		// its pictures; the keys are the chooser's, and the mouse the
		// terminal's.
		body, mouse = m.overlayOpenWith(body, w, h), tea.MouseModeNone
	}
	bar, hint := m.statusLine(w, h)
	if m.scrubberVisible() {
		hint, mouse = m.scrubberView(), tea.MouseModeCellMotion
	}
	if m.search != nil && m.screen == screenDocument && !m.help && m.layer == layerNone {
		hint = ansi.Truncate(m.search.status(w), w, "…")
	}
	if m.layer == layerActions && mouse < tea.MouseModeCellMotion {
		mouse = tea.MouseModeCellMotion // For clicks on the entries.
	}
	body = m.framed(body)
	palette := m.palette()
	hint = lipgloss.NewStyle().Foreground(palette.Muted).Background(palette.Background).Width(w).Render(hint)
	content := body + "\n" + bar + "\n" + hint
	if m.exiting {
		mouse = tea.MouseModeNone // See mouseSettle.
	}
	if m.quitting {
		// Keys no longer answer. The frame keeps its height, or Bubble Tea
		// draws it below the last one; and it ends on an empty row, because
		// the last row is erased on the way out. The prompt lands there.
		content, mouse = body+"\n"+bar+"\n", tea.MouseModeNone
	}
	if m.height < 3 || (m.screen == screenDocument && m.chart != nil && m.height < 5) {
		content = ansi.Truncate(i18n.T("app.enlarge", "gloss: enlarge terminal"), w, "")
	}
	v := tea.NewView(content)
	v.ProgramStatus = m.programStatus()
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
		mouse = tea.MouseModeCellMotion // Clicks on rows, and the wheel.
		if m.preview != nil && m.previewWidth() > 0 {
			mouse = max(mouse, m.preview.View().MouseMode)
		}
		if m.screen == screenGrid {
			mouse = tea.MouseModeCellMotion
		}
	case m.loading && m.hasPictureFront():
		body, plain = m.pictureView(), false
		exact = !m.quitting && m.pictureExact(w, h)
	case m.loading:
		body = i18n.Tf("app.loading", "Loading %s…", safe(filepath.Base(m.opts.Files[m.index])))
	case m.err != nil:
		body = i18n.Tf("app.loadError", "Cannot open file\n\n%s\n\nR retry · ] next file · q quit", safe(m.err.Error()))
	case m.video != nil:
		body, mouse, plain = m.video.View(), tea.MouseModeCellMotion, false
	case m.mapView != nil:
		// Drags pan it and the wheel zooms.
		body, mouse, plain = m.mapView.View().Content, tea.MouseModeCellMotion, false
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
		if m.typeset != nil || m.pictureMouse() {
			mouse = tea.MouseModeCellMotion
			if m.typeset != nil && len(m.typesetLinks) > 0 && !m.isPreview && m.layer == layerNone {
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
		return body, mouse
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
	title, how := i18n.T("app.landing.drop", "Drop files here to open"), i18n.T("app.landing.paste", "Drag them from a file manager, or paste their paths.")
	if m.opts.FilesFS != nil {
		// The browser: files come from drops, or from the page around.
		how = i18n.T("app.landing.terminal", "Drag them from a file manager onto the terminal.")
	}
	if m.canBrowse() {
		how = i18n.T("app.landing.browse", "Drag them from a file manager, paste their paths, or press o to browse.")
	}
	if m.opts.Pick {
		title = i18n.T("app.landing.send", "Drop a file here to send it")
		how = i18n.T("app.landing.sendHow", "Drag it from a file manager, paste its path, or press o to browse.")
	}
	body := title + "\n\n" + how
	// The name outranks the rest: as the screen shortens, the list of
	// formats goes first, then the words on what to do, and the name stays.
	h, lines := m.bodyHeight(), strings.Count(body, "\n")+1
	formats := formatLines()
	long := lipgloss.NewStyle().Width(lipgloss.Width(strings.Join(formats, "\n"))).Render(strings.Join(formats, "\n\n"))
	short := lipgloss.NewStyle().Width(w).Align(lipgloss.Center).Render(wrapList(shortFormats(), i18n.T("app.formats.comma", ", "), w))
	name, gap := strings.Join(banner, "\n"), "\n\n\n"
	art := lipgloss.NewStyle().Foreground(m.palette().Accent).Bold(true).Render(name)
	room := func(extra int) bool { return h >= len(banner)+2+lines+extra }
	switch {
	case w < lipgloss.Width(name) || h < len(banner):
		// Too small to draw the name, so it is said, with what to do if
		// there is room to say it.
		if tiny := i18n.T("app.landing.tiny", "GLOSS -- drag here"); w >= ansi.StringWidth(tiny) {
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
	case layerLang:
		return overlay(body, m.langPickerView(w, h), w, 0)
	case layerQR:
		return overlay(body, m.qrView(w, h), w, 0)
	case layerActions:
		box := m.actionsView(w, h)
		if box == "" {
			return body
		}
		return overlayAt(body, box, w, m.actions.box.Min.X, m.actions.box.Min.Y)
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

// orderName names the file browser's sort order.
func orderName(i int) string {
	switch orderNames[i] {
	case "name":
		return i18n.T("app.browser.byName", "name")
	case "date":
		return i18n.T("app.browser.byDate", "date")
	case "kind":
		return i18n.T("app.browser.byKind", "kind")
	}
	return orderNames[i]
}

func (m *Model) browserStatus(w int) (bar, hint string) {
	// The end of a long path says where you are; the start rarely does.
	dir, note := safe(m.opener.dir), " · "+i18n.Tf("app.browser.order", "by %s", orderName(m.sortBy))
	if m.note != "" {
		note += " · " + m.note
	}
	open := " " + i18n.T("app.browser.open", "Open") + " · "
	if over := ansi.StringWidth(open+dir+note) - w; over > 0 && over+1 < ansi.StringWidth(dir) {
		dir = "…" + ansi.TruncateLeft(dir, over+1, "")
	}
	bar = lipgloss.NewStyle().Width(w).Render(ansi.Truncate(open+dir+note, w, "…"))
	greyed, text := i18n.T("app.browser.hideGreyed", "hide greyed"), i18n.T("app.browser.noText", "no text")
	if m.hideUnsupported {
		greyed = i18n.T("app.browser.showGreyed", "show greyed")
	}
	if m.noTextFiles {
		text = i18n.T("app.browser.text", "text")
	}
	hint = ansi.Truncate(" "+i18n.Tf("app.browser.keys", "?/F1 help · Enter open · ←/→ folders · / find · G go to · Tab complete · Ctrl-F types · Ctrl-L layout · Ctrl-G places · Ctrl-S sort · Ctrl-T %s · Ctrl-X %s · Alt-Enter open with · Esc cancel", greyed, text), w, "")
	if m.opener.going {
		hint = ansi.Truncate(" "+i18n.T("app.browser.goKeys", "Type a folder's path · Tab completes folders · Enter go · Esc back"), w, "")
	}
	if f := m.opener.find; f != nil {
		bar = lipgloss.NewStyle().Width(w).Render(ansi.Truncate(" "+i18n.T("app.browser.find", "Find")+" · "+f.status(), w, "…"))
		hint = ansi.Truncate(" "+i18n.T("app.browser.findKeys", "Enter search, then open · ↑/↓ select · Ctrl-A add all · Esc back"), w, "")
	}
	return bar, hint
}

func (m *Model) listStatus(w int) (bar, hint string) {
	status := " " + i18n.Tf("app.files.status", "Files · %d/%d selected · current %d", m.selection+1, len(m.opts.Files), m.index+1)
	hint = " " + i18n.T("app.files.keys", "↑/↓ select · Enter open · t thumbnails · v preview · w/W apps · Esc cancel · q quit")
	if m.screen == screenGrid && m.grid != nil {
		status = " " + i18n.Tf("app.files.gridStatus", "Files · grid · %d/%d selected · current %d", m.selection+1, len(m.opts.Files), m.index+1)
		if m.grid.pending() {
			status += " · " + i18n.T("app.files.thumbnails", "making thumbnails")
		}
		hint = " " + i18n.T("app.files.gridKeys", "←→↑↓ select · Enter open · t list · Esc cancel · q quit")
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
	name := i18n.T("app.status.noFiles", "no files")
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
	if note := m.actionsNote(); note != "" {
		status = " " + note + " · " + status[1:]
	}
	if empty {
		status = " " + i18n.T("app.status.noFilesYet", "No files yet")
		if m.note != "" {
			status += " · " + m.note
		}
	}
	if m.hoverLinkActive && m.typeset != nil {
		if address := m.linkAt(m.hoverLinkPoint.X, m.hoverLinkPoint.Y); address != "" {
			status = " " + i18n.Tf("app.status.link", "Link: %s", safe(address))
			if _, internal := document.FootnotePeer(address); internal {
				status = " " + i18n.T("app.status.footnoteBack", "Return to footnote reference")
				if strings.HasPrefix(address, "gloss:fn:") {
					status = " " + i18n.T("app.status.footnote", "Jump to footnote")
				}
			}
		}
	}
	bar = m.statusStyle().Width(w).Render(ansi.Truncate(status, w, "…"))
	return bar, ansi.Truncate(m.hints(), w, "")
}

// counted fills form, a plural form from Tn with %s for n, with n grouped
// by thousands.
func counted(form string, n int) string { return fmt.Sprintf(form, grouped(n)) }

// detail says what is shown of the document. How things are drawn, the
// renderer and the transport, is the info box's to say.
func (m *Model) detail(w, h int) string {
	detail := fmt.Sprintf("%s · %dx", m.kind, 1<<m.zoom)
	if m.animation != nil {
		state := i18n.T("app.status.playing", "playing")
		if m.animation.Status().Paused {
			state = i18n.T("app.status.paused", "paused")
		} else if m.animation.Status().Finished {
			state = i18n.T("app.status.finished", "finished")
		}
		if m.animation.target != 0 {
			state = i18n.Tf("app.status.seeking", "seeking %d", m.animation.target)
		}
		detail += " · " + i18n.Tf("app.status.speed", "%g× speed", m.animation.rate()) + " · " + i18n.Tf("app.status.frame", "frame %d/%d", m.animation.player().Frame(), m.animation.player().Frames()) + " · " + state
	}
	if m.kind == "pdf" {
		detail += " · " + i18n.Tf("app.status.page", "page %d/%d", m.page, m.pages)
	}
	switch {
	case m.video != nil:
		detail = m.videoDetail()
	case m.mapFile != nil && m.mapFile.Tiles != nil:
		detail = "PMTiles · " + i18n.Tf("app.status.zoom", "zoom %d", m.mapView.Zoom())
	case m.mapFile != nil:
		n := len(m.mapFile.Data.Features)
		detail = strings.ToUpper(m.kind) + " · " + counted(i18n.Tn("app.status.features", "%s feature", "%s features", n), n) + " · " + i18n.Tf("app.status.zoom", "zoom %d", m.mapView.Zoom())
	case m.chart != nil:
		detail = strings.ToUpper(m.kind) + " · " + counted(i18n.Tn("app.status.triangles", "%s triangle", "%s triangles", m.triangles), m.triangles)
		if m.triangles == 0 && m.points > 0 {
			detail = strings.ToUpper(m.kind) + " · " + counted(i18n.Tn("app.status.points", "%s point", "%s points", m.points), m.points)
		}
		if m.hasParts() && m.partsShown != nil {
			detail += " · " + i18n.Tf("app.status.parts", "parts %d/%d", m.partsOnScreen(), len(m.parts))
		}
		if m.tint != nil {
			detail += " · " + i18n.Tf("app.status.painted", "painted #%02x%02x%02x", m.tint.R, m.tint.G, m.tint.B)
		}
		if m.bg != nil {
			detail += " · " + i18n.Tf("app.status.background", "background #%02x%02x%02x", m.bg.R, m.bg.G, m.bg.B)
		}
	case m.typeset != nil && m.markdown == nil:
		format := map[string]string{"markdown": "Markdown", "json": "JSON"}[m.kind]
		if m.kind == "text" {
			format = i18n.T("app.status.textFormat", "Text")
		}
		detail = format + " · " + i18n.T("app.status.typeset", "typeset")
		if s := m.typesetShown; s != nil && s.Lines > 0 {
			detail += " · " + i18n.Tf("app.status.line", "line %d/%d", s.Line, s.Lines)
		}
	case m.markdown != nil:
		mode := i18n.T("app.status.rendered", "rendered")
		if m.markdown.raw {
			mode = i18n.T("app.status.source", "source")
		}
		format := map[string]string{"docx": "Word", "json": "JSON", "html": "HTML"}[m.kind]
		switch m.kind {
		case "ipynb":
			format = i18n.T("app.status.notebook", "notebook")
		case "text":
			format = i18n.T("app.status.text", "text")
		}
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
			detail = "xlsx · " + i18n.Tf("app.status.sheet", "sheet %d/%d", m.page, m.pages) + " · " + safe(m.sheet.sheet.Name) + " · " + m.sheet.status(w, h)
		case "grist":
			detail = "grist · " + i18n.Tf("app.status.table", "table %d/%d", m.page, m.pages) + " · " + safe(m.sheet.sheet.Name) + " · " + m.sheet.status(w, h)
		}
	case m.kind == "3mf":
		// Too large to draw, or a preview: the picture the file carries.
		detail = "3MF · " + i18n.T("app.status.thumbnail", "thumbnail") + fmt.Sprintf(" · %dx", 1<<m.zoom)
	}
	return detail
}

// keyRow is a row of key hints, drawn " a · b · c".
type keyRow []string

func (r keyRow) String() string {
	if len(r) == 0 {
		return ""
	}
	return " " + strings.Join(r, " · ")
}

// instead is the row led by first, in place of the quit key where the row
// starts with it.
func (r keyRow) instead(first ...string) keyRow {
	if len(r) > 0 && r[0] == quitKey() {
		r = r[1:]
	}
	return append(keyRow(first), r...)
}

// The keys that most rows name.
func quitKey() string    { return i18n.T("app.keys.quit", "q quit") }
func helpKey() string    { return i18n.T("app.keys.help", "? help") }
func menuKey() string    { return i18n.T("app.keys.menu", "m files") }
func filesKey() string   { return i18n.T("app.keys.files", "[/] files") }
func zoomKey() string    { return i18n.T("app.keys.zoom", "+/- zoom") }
func exportKey() string  { return i18n.T("app.keys.export", "e export") }
func infoKey() string    { return i18n.T("app.keys.info", "i info") }
func actionsKey() string { return i18n.T("app.keys.actions", "a actions") }

// chopKey names what S does next: chop long lines, or wrap them again.
func chopKey(chopped bool) string {
	if chopped {
		return i18n.T("app.keys.wrap", "S wrap")
	}
	return i18n.T("app.keys.chop", "S chop")
}

// hints names the keys that work over the document.
func (m *Model) hints() string {
	if m.fontPicker != nil {
		return " " + i18n.T("app.font.status", "↑/↓ fonts · Enter keep · Esc cancel · +/- zoom")
	}
	if m.themePicker != nil {
		return " " + i18n.T("app.theme.status", "↑/↓ preview theme · Enter keep · Esc cancel")
	}
	if m.langPicker != nil {
		return " " + i18n.T("app.lang.status", "↑/↓ preview language · Enter keep · Esc cancel")
	}
	if m.lineInput != nil {
		return m.lineInput.status(m.width)
	}
	if m.placeInput != nil {
		return m.placeInput.status(m.width)
	}
	if len(m.opts.Files) == 0 {
		keys := keyRow{quitKey(), helpKey(), infoKey()}
		if m.canBrowse() {
			keys = append(keys, i18n.T("app.keys.browse", "o browse"))
		}
		return keys.String()
	}
	// A layer takes the keys while it is up.
	switch m.layer {
	case layerTheme:
		return " " + i18n.T("app.theme.status", "↑/↓ preview theme · Enter keep · Esc cancel")
	case layerActions:
		return " " + i18n.T("app.keys.actionsLayer", "↑/↓ choose · Enter do · Esc close")
	case layerQR:
		return " " + i18n.T("app.keys.qrLayer", "Esc/u close QR · g graphics · e export QR · q quit")
	case layerColumns:
		return " " + i18n.T("app.keys.columnsLayer", "↑/↓ select · Space show/hide · a all · n none · Esc close")
	case layerParts:
		return " " + i18n.T("app.keys.partsLayer", "↑/↓ select · Space show/hide · Enter focus · a all · n only · X all and close · Esc close")
	case layerColor:
		return " " + i18n.T("app.keys.colorLayer", "Tab mode · ↑/↓ ←/→ choose · Space apply · r file's colors · Enter keep · Esc undo")
	}
	keys := keyRow{quitKey(), helpKey(), menuKey(), filesKey(), i18n.T("app.keys.pages", "n/p pages"), zoomKey(), exportKey(), infoKey()}
	switch {
	case m.typeset != nil && m.markdown == nil:
		keys = keyRow{quitKey(), i18n.T("app.keys.wheel", "↑/↓/wheel scroll"), i18n.T("app.keys.screen", "Space/b screen"), zoomKey(), i18n.T("app.keys.fitWidth", "f fit width"), i18n.T("app.keys.source", "s source"), i18n.T("app.keys.text", "t text"), exportKey()}
		if m.typesetLined() {
			keys = append(keys, i18n.T("app.keys.line", ": line"), i18n.T("app.keys.numbers", "# numbers"), chopKey(m.typesetShown.Chop))
		}
		if m.typesetSelected != "" {
			keys = keyRow{i18n.T("app.keys.copy", "y copy selection"), i18n.T("app.keys.clear", "Esc clear"), i18n.T("app.keys.wheel", "↑/↓/wheel scroll"), zoomKey(), exportKey(), quitKey()}
		}
	case m.typeset != nil:
		keys = keyRow{quitKey(), menuKey(), i18n.T("app.keys.scroll", "↑/↓ scroll"), i18n.T("app.keys.spaceScroll", "Space/b scroll"), i18n.T("app.keys.typesetView", "s typeset view"), i18n.T("app.keys.text", "t text")}
	case m.animation != nil:
		keys = keyRow{i18n.T("app.keys.pause", "Space pause/play"), i18n.T("app.keys.speed", "</> speed"), i18n.T("app.keys.normalSpeed", "Backspace 1×"), zoomKey(), i18n.T("app.keys.exportFrame", "e export frame"), helpKey(), quitKey()}
	case m.markdown != nil:
		keys = keyRow{quitKey(), menuKey(), i18n.T("app.keys.scroll", "↑/↓ scroll"), i18n.T("app.keys.page", "Space/b page"), i18n.T("app.keys.source", "s source"), i18n.T("app.keys.graphics", "g graphics")}
		if m.canTypeset() {
			keys = append(keys, i18n.T("app.keys.typeset", "t typeset"))
		}
		if m.markdown.numbered() {
			keys = append(keys, i18n.T("app.keys.line", ": line"), i18n.T("app.keys.numbers", "# numbers"))
		}
		keys = append(keys, chopKey(m.markdown.chop), selectHint())
	case m.sheet != nil:
		turn := i18n.T("app.keys.sheets", "n/p sheets")
		switch {
		case m.kind == "grist":
			turn = i18n.T("app.keys.tables", "n/p tables")
		case !m.booked():
			turn = i18n.T("app.keys.nextFiles", "n/p files")
		}
		keys = keyRow{quitKey(), menuKey(), i18n.T("app.keys.move", "↑/↓ ←/→ move"), i18n.T("app.keys.page", "Space/b page"), turn, i18n.T("app.keys.columns", "c columns"), infoKey()}
		if m.sheet.rows() > 0 { // An empty sheet has no cell to act on.
			keys = append(keys, actionsKey())
		}
		if m.sheet.url() != "" && m.opts.Fetch != nil {
			keys = keys.instead(i18n.T("app.keys.openAddress", "Enter open address"))
		}
		if m.sheet.url() != "" {
			keys = keys.instead(i18n.T("app.keys.qrAddress", "u QR of address"))
		}
	case m.video != nil:
		keys = keyRow{quitKey(), i18n.T("app.keys.play", "Space play/pause"), i18n.T("app.keys.seek", "←/→ seek")}
		if len(m.video.Status().CaptionTracks) > 0 {
			keys = append(keys, i18n.T("app.keys.subtitles", "c subtitles"))
		}
		keys = append(keys, helpKey(), menuKey(), filesKey(), exportKey(), infoKey())
	case m.mapView != nil:
		keys = keyRow{quitKey(), i18n.T("app.keys.pan", "arrows pan"), zoomKey(), i18n.T("app.keys.fit", "f fit")}
		if m.opts.Maps != nil && m.opts.Maps.Geocoder != nil {
			keys = append(keys, i18n.T("app.keys.findPlace", "/ find a place"))
		}
		keys = append(keys, helpKey(), menuKey(), filesKey(), exportKey(), infoKey())
	case m.chart != nil && m.mesh != nil:
		keys = keyRow{quitKey(), helpKey(), menuKey(), filesKey(), exportKey(), infoKey(), i18n.T("app.keys.color", "C color"), i18n.T("app.keys.background", "B bg-color")}
		if m.hasParts() {
			keys = append(keys, i18n.T("app.keys.parts", "c parts"))
		}
	}
	if p, ok := m.viewPicture(); ok {
		if len(m.pictureEntries(p)) > 0 {
			keys = append(keys, actionsKey())
		}
	} else if m.tabItems() {
		keys = append(keys, i18n.T("app.keys.items", "Tab items"), actionsKey())
	}
	if m.isFetched() {
		keys = keys.instead(i18n.T("app.keys.close", "Esc close"))
	}
	if m.searchable() {
		keys = append(keyRow{i18n.T("app.keys.search", "/ search")}, keys...)
	}
	if m.canBrowse() {
		keys = append(keys, i18n.T("app.keys.browse", "o browse"))
	}
	if what := m.picking(); what != "" {
		// Say what will be sent before it is.
		keys = keys.instead(i18n.Tf("app.keys.send", "Enter send %s", what), i18n.T("app.keys.cancel", "q cancel"))
	}
	return keys.String()
}

// selectHint says how to select text the terminal's way on a screen that
// takes the mouse: with Option held in iTerm2, with Shift in most others.
func selectHint() string {
	if os.Getenv("TERM_PROGRAM") == "iTerm.app" {
		return i18n.T("app.keys.optionSelect", "⌥-drag select")
	}
	return i18n.T("app.keys.shiftSelect", "Shift-drag select")
}
