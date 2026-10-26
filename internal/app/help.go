package app

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"nimbleterminal.dev/gloss/internal/i18n"
)

// helpRow is one key group and what it does; a row with no keys is a note.
type helpRow struct{ keys, action string }

type helpSection struct {
	title string
	rows  []helpRow
}

// helpLeft is what works everywhere and in text documents; helpRight is for
// pictures, pages, meshes, GIFs and videos. At 80 columns they sit side by side in
// 22 rows, the room a 24-row terminal leaves; narrower, they stack. Both are
// in the active language, each action within the 27 cells its keys leave at
// 80 columns.
func helpLeft() []helpSection {
	return []helpSection{
		{i18n.T("app.help.everywhere.title", "Everywhere"), []helpRow{
			{"q  Q", i18n.T("app.help.everywhere.quit", "quit · quit, keep the view")},
			{"?  Esc", i18n.T("app.help.everywhere.help", "help · back to the files")},
			{"]  [", i18n.T("app.help.everywhere.files", "next / previous file")},
			{"m  o", i18n.T("app.help.everywhere.menu", "file menu · browse folders")},
			{"w  W  E", i18n.T("app.help.everywhere.open", "open · choose app · edit")},
			{"", i18n.T("app.help.everywhere.chooser", "Tab in the app chooser: open / edit")},
			{"e  i  R", i18n.T("app.help.everywhere.export", "export · details · reload")},
			{"/  n  N", i18n.T("app.help.everywhere.search", "search · next / prev match")},
			{"T  F2  a", i18n.T("app.help.everywhere.theme", "theme · language · actions")},
			{"", i18n.T("app.help.everywhere.drop", "Drop files or paste paths to open")},
		}},
		{i18n.T("app.help.text.title", "Text"), []helpRow{
			{"t  s  f", i18n.T("app.help.text.typeset", "typeset · source · fonts")},
			{"Tab", i18n.T("app.help.text.items", "next Markdown item or file")},
			{"#  :  S", i18n.T("app.help.text.lines", "line numbers · go to · chop")},
			{"drag + y", i18n.T("app.help.text.copy", "select + copy typeset text")},
			{"Shift-drag", i18n.T("app.help.text.select", "select in text views")},
		}},
	}
}

func helpRight() []helpSection {
	return []helpSection{
		{i18n.T("app.help.pictures.title", "Pictures and pages"), []helpRow{
			{"+ - f 0", i18n.T("app.help.pictures.zoom", "zoom · fit / reset")},
			{"h j k l", i18n.T("app.help.pictures.pan", "pan (or the arrows)")},
			{"n p Space", i18n.T("app.help.pictures.pages", "next / prev page or file")},
			{"Home  End", i18n.T("app.help.pictures.ends", "first / last PDF page")},
			{"g", i18n.T("app.help.pictures.graphics", "Kitty / glyph graphics")},
			{"u", i18n.T("app.help.pictures.qr", "QR code of a table URL")},
		}},
		{i18n.T("app.help.meshes.title", "Meshes"), []helpRow{
			{"drag", i18n.T("app.help.meshes.orbit", "orbit (Shift-drag pans)")},
			{"wheel  5", i18n.T("app.help.meshes.zoom", "zoom · orthographic")},
			{"r", i18n.T("app.help.meshes.rotate", "auto-rotate")},
			{"C  B", i18n.T("app.help.meshes.colors", "model color · background")},
		}},
		{i18n.T("app.help.motion.title", "GIFs and videos"), []helpRow{
			{"Space , .", i18n.T("app.help.motion.play", "play / pause · step frames")},
			{"< > Bksp", i18n.T("app.help.motion.speed", "speed · back to 1×")},
			{"drag ← → c", i18n.T("app.help.motion.seek", "seek (video 5 s) · captions")},
		}},
	}
}

// Browser help describes the active file chooser rather than document controls.
var browserHelpLeft = []helpSection{
	{"File browser", []helpRow{
		{"type", "filter file names"},
		{"? / F1", "help (F1 while typing)"},
		{"↑/↓", "select an entry"},
		{"PgUp/PgDn", "move a screen"},
		{"Enter", "open file or enter folder"},
		{"←/→", "parent / enter folder"},
		{"Alt-↑", "parent folder"},
		{"Alt-←/→", "back / forward"},
		{"Tab", "complete name"},
		{"Shift-Tab", "previous completion"},
		{"Alt-Enter", "choose an external app"},
	}},
}
var browserHelpRight = []helpSection{
	{"Layout and filters", []helpRow{
		{"Ctrl-L", "list / columns / places"},
		{"Ctrl-G", "focus places sidebar"},
		{"Ctrl-F", "choose file types"},
		{"Ctrl-S", "sort by name / date / kind"},
		{"Ctrl-T", "hide / show unsupported"},
		{"Ctrl-X", "include / exclude text"},
	}},
	{"Find and leave", []helpRow{
		{"/", "find files (empty filter)"},
		{"G", "go to folder (empty filter)"},
		{"Esc", "close popup or return to view"},
		{"Ctrl-C", "quit gloss"},
		{"", "Help preserves your filter and selection"},
	}},
}

const helpKeyWidth = 10

// helpColumn renders sections one under another, each row a key column and
// an action, within width columns.
func (m *Model) helpColumn(sections []helpSection, width int) []string {
	p := m.palette()
	title := lipgloss.NewStyle().Foreground(p.Accent).Bold(true)
	keys := lipgloss.NewStyle().Foreground(p.Foreground).Bold(true)
	action := lipgloss.NewStyle().Foreground(p.Muted)
	var rows []string
	for i, s := range sections {
		if i > 0 {
			rows = append(rows, "")
		}
		rows = append(rows, title.Render(ansi.Truncate(s.title, width, "")))
		for _, r := range s.rows {
			if r.keys == "" {
				rows = append(rows, action.Render(ansi.Truncate(r.action, width, "…")))
				continue
			}
			k := ansi.Truncate(r.keys, helpKeyWidth, "")
			pad := strings.Repeat(" ", max(0, helpKeyWidth-ansi.StringWidth(k)))
			rows = append(rows, keys.Render(k)+pad+" "+action.Render(ansi.Truncate(r.action, max(1, width-helpKeyWidth-1), "…")))
		}
	}
	return rows
}

// helpView is the help page at w×h: a title row, two columns or one, and
// the docs row, scrolled by helpOffset when it is taller than the screen.
func (m *Model) helpView(w, h int) string {
	leftSections, rightSections := helpLeft(), helpRight()
	if m.browsing() {
		leftSections, rightSections = browserHelpLeft, browserHelpRight
	}
	p := m.palette()
	name := lipgloss.NewStyle().Foreground(p.Accent).Bold(true).Render("gloss " + safe(m.opts.Version))
	closing := lipgloss.NewStyle().Foreground(p.Muted).Render(i18n.T("app.help.closes", "? or Esc closes"))
	heading := " " + name + lipgloss.NewStyle().Foreground(p.Foreground).Render(" — "+i18n.T("app.help.title", "a visual pager"))
	gap := max(1, w-ansi.StringWidth(heading)-ansi.StringWidth(closing)-1)
	rows := []string{heading + strings.Repeat(" ", gap) + closing, ""}
	if w >= 80 {
		col := (w - 4) / 2
		left, right := m.helpColumn(leftSections, col), m.helpColumn(rightSections, col)
		for i := range max(len(left), len(right)) {
			l, r := "", ""
			if i < len(left) {
				l = left[i]
			}
			if i < len(right) {
				r = right[i]
			}
			rows = append(rows, " "+l+strings.Repeat(" ", max(0, col-ansi.StringWidth(l)))+"  "+r)
		}
	} else {
		for _, row := range m.helpColumn(append(append([]helpSection{}, leftSections...), rightSections...), w-2) {
			rows = append(rows, " "+row)
		}
	}
	footer := lipgloss.NewStyle().Foreground(p.Muted).Render(" "+i18n.T("app.help.footer", "gloss --help lists options · docs: ")) + ansi.SetHyperlink(DocsURL) + DocsURL + ansi.ResetHyperlink()
	rows = append(rows, "", footer)
	// Scroll when the page is taller than the screen; the footer scrolls too.
	m.helpOffset = max(0, min(m.helpOffset, len(rows)-h))
	if len(rows) > h {
		rows = rows[m.helpOffset:min(len(rows), m.helpOffset+h)]
	}
	return strings.Join(rows, "\n")
}

// helpScreen is the help as plain text, every section one under another:
// the words of the page, for what reads it rather than looks at it.
func helpScreen() string {
	var b strings.Builder
	b.WriteString("gloss — " + i18n.T("app.help.title", "a visual pager") + "\n\n")
	for _, s := range append(helpLeft(), helpRight()...) {
		b.WriteString(s.title + "\n")
		for _, r := range s.rows {
			if r.keys == "" {
				b.WriteString(r.action + "\n")
				continue
			}
			b.WriteString(r.keys + "  " + r.action + "\n")
		}
		b.WriteString("\n")
	}
	b.WriteString(i18n.T("app.help.screenFooter", "gloss --help lists options. Docs: ") + ansi.SetHyperlink(DocsURL) + DocsURL + ansi.ResetHyperlink() + "\n")
	return b.String()
}
