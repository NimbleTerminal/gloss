package app

import (
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// lineInput is the go-to-line prompt: the digits typed so far.
type lineInput struct{ draft []rune }

const lineNumbersNeedSource = "line numbers need the source view: press s"

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
	need := lineNumbersNeedSource
	if typeset {
		need = "line numbers: text, source and JSON pages only"
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
			m.note = strings.Replace(need, "line numbers", "go to line", 1)
			return nil, true
		}
		m.lineInput = &lineInput{}
		return nil, true
	}
	return nil, false
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
	return ansi.Truncate(":"+string(in.draft)+"\x1b[7m▏\x1b[27m  Enter go to line · Esc cancel", width, "")
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
	if v.numbered() {
		return fmt.Sprintf("line %d/%d", v.topLine(), v.lastLine)
	}
	return fmt.Sprintf("line %d/%d", min(v.offset+1, len(v.lines)), len(v.lines))
}
