package app

import (
	"strconv"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"nimbleterminal.dev/gloss/internal/i18n"
)

// lineInput is the go-to-line prompt: the digits typed so far.
type lineInput struct{ draft []rune }

// typesetLined says the typeset page on show is a document whose blocks are
// its lines, which the loader numbers and goes to.
func (m *Model) typesetLined() bool {
	return m.typeset != nil && m.markdown == nil && m.typesetShown != nil && m.typesetShown.Lines > 0
}

// lineKey gives the text and typeset views their line keys: # shows file
// line numbers, : asks for a line to go to. Rows that are not file lines,
// as rendered Markdown prose or a typeset Markdown page, have neither; the
// source view has both. # arms the numbers even so, for the next view that
// has lines.
func (m *Model) lineKey(k string) (tea.Cmd, bool) {
	typeset := m.typeset != nil && m.markdown == nil
	if m.markdown == nil && !typeset {
		return nil, false
	}
	lined := m.typesetLined()
	if m.markdown != nil {
		lined = m.markdown.numbered()
	}
	need, goNeed := i18n.T("app.line.numbersSource", "line numbers need the source view: press s"), i18n.T("app.line.goSource", "go to line need the source view: press s")
	if typeset {
		need, goNeed = i18n.T("app.line.numbersPages", "line numbers: text, source and JSON pages only"), i18n.T("app.line.goPages", "go to line: text, source and JSON pages only")
	}
	switch k {
	case "#":
		m.lineNumbers = !m.lineNumbers
		if !lined {
			m.note = need
		}
		if typeset {
			if !lined {
				return nil, true
			}
			return m.queueTypesetReflow(), true
		}
		return m.layoutMarkdown(), true
	case ":":
		if !lined {
			m.note = goNeed
			return nil, true
		}
		m.lineInput = &lineInput{}
		return nil, true
	case "S":
		// Chopping is for any text view, as less -S is; a typeset page chops
		// only where its blocks are lines.
		m.chopLines = !m.chopLines
		if typeset {
			if !lined {
				m.note = i18n.T("app.line.chopPages", "chopping lines: text, source and JSON pages only")
				return nil, true
			}
			return m.queueTypesetReflow(), true
		}
		return m.layoutMarkdown(), true
	}
	return nil, false
}

// reveal scrolls a chopped view sideways so that the search highlight on a
// row is in view, a third of the way in.
func (v *markdownView) reveal(row int) {
	hits := v.highlights[row]
	if !v.chop || len(hits) == 0 || row < 0 || row >= len(v.lines) {
		return
	}
	plain := ansi.Strip(v.lines[row].text)
	col := ansi.StringWidth(plain[:min(len(plain), hits[0][0])])
	avail := max(1, v.width-v.gutter)
	if col < v.left || col >= v.left+avail {
		v.left = max(0, col-avail/3)
		v.scrollX(0)
	}
}

// lineInputKey edits the go-to-line prompt: digits, Backspace, Enter goes,
// Esc leaves the view where it is.
func (m *Model) lineInputKey(msg tea.KeyPressMsg) tea.Cmd {
	in := m.lineInput
	switch msg.String() {
	case "ctrl+c":
		return m.quit()
	case "esc":
		m.lineInput = nil
	case "enter":
		m.lineInput = nil
		n, err := strconv.Atoi(string(in.draft))
		if err != nil || n < 1 {
			return nil
		}
		if m.markdown != nil {
			m.markdown.goLine(n)
		} else if m.typesetLined() {
			m.typesetGoLine = n
			return m.queueTypesetReflow()
		}
	case "backspace":
		if len(in.draft) > 0 {
			in.draft = in.draft[:len(in.draft)-1]
		}
	case "ctrl+u":
		in.draft = nil
	default:
		for _, r := range msg.Text {
			if r >= '0' && r <= '9' && len(in.draft) < 7 {
				in.draft = append(in.draft, r)
			}
		}
	}
	return nil
}

// lineInputStatus is the prompt as the hint line shows it.
func (in *lineInput) status(width int) string {
	return ansi.Truncate(":"+string(in.draft)+"\x1b[7m▏\x1b[27m  "+i18n.T("app.line.keys", "Enter go to line · Esc cancel"), width, "")
}

// topLine is the file line at the top of a text view, the first numbered
// row at or below the top when the top rows are margin.
func (v *markdownView) topLine() int {
	for i := v.offset; i < len(v.lines); i++ {
		if v.lines[i].line > 0 {
			return v.lines[i].line
		}
	}
	return v.lastLine
}

// lineDetail is the position part of the status bar for a text view: file
// lines when the rows are file lines, screen rows otherwise.
func (v *markdownView) lineDetail() string {
	detail := i18n.Tf("app.status.line", "line %d/%d", min(v.offset+1, len(v.lines)), len(v.lines))
	if v.numbered() {
		detail = i18n.Tf("app.status.line", "line %d/%d", v.topLine(), v.lastLine)
	}
	if v.chop && v.left > 0 {
		detail += " · " + i18n.Tf("app.status.column", "col %d", v.left+1)
	}
	return detail
}
