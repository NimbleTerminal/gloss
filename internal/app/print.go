package app

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"nimbleterminal.dev/gloss/internal/document"
	"nimbleterminal.dev/gloss/internal/theme"
	"nimbleterminal.dev/ntpicture"
)

// Cell pixels for printed pictures: glyphs pack two pixel rows in a cell.
const printCellWidth, printCellHeight = 10, 20

// PrintView renders the initial content viewport without a Tea event loop,
// terminal queries, graphics protocols, animations or interactive chrome.
func PrintView(ctx context.Context, r document.Result, opts Options, width, height int) (string, error) {
	if width < 1 || width > 512 || height < 1 || height > 256 {
		return "", fmt.Errorf("print viewport must be 1..512 columns by 1..256 rows")
	}
	if r.Err != nil {
		return "", r.Err
	}
	var view string
	switch {
	case r.Sheet != nil:
		view = printSheet(r.Sheet, opts, width, height)
	case r.Markdown != nil:
		// Do not create animation players even if a caller supplies a document
		// previously loaded for the interactive viewer.
		doc := *r.Markdown
		doc.Images = append(doc.Images[:0:0], doc.Images...)
		for i := range doc.Images {
			doc.Images[i].Animation = nil
		}
		m := newMarkdownView(&doc, printSession())
		m.setTheme(theme.Resolve(opts.Theme))
		m.numbers, m.fenced, m.chop = opts.LineNumbers, r.Kind == "text" || r.Kind == "json", opts.ChopLines
		if err := settlePrint(tea.Sequence(m.setKitty(false), m.layout(width, height, printCellWidth, printCellHeight), m.syncPictures(true)), m.update); err != nil {
			return "", err
		}
		view = m.view()
		m.close()
	default:
		if r.Mesh != nil {
			r.CPU, r.Views = true, opts.Views
			var err error
			r.Image, err = document.ExportImage(r, min(4096, max(width*2, height*4)))
			if err != nil {
				return "", err
			}
		}
		if r.Image == nil {
			return "", fmt.Errorf("document has no printable view")
		}
		pic := ntpicture.New(ntpicture.Options{Session: printSession(), Render: ntpicture.Config{
			CellPixelWidth: printCellWidth, CellPixelHeight: printCellHeight, Background: theme.Get(opts.Theme).Background,
		}})
		if err := settlePrint(tea.Batch(pic.SetSize(width, height), pic.SetImage(r.Image)), pic.Update); err != nil {
			return "", err
		}
		view = pic.View()
		pic.Close()
	}
	// Raw stdout has no terminal renderer to filter control sequences. Only
	// text, line breaks and numeric SGR styles may cross this boundary.
	view = printSafe(view)
	lines := strings.Split(view, "\n")
	lines = lines[:min(len(lines), height)]
	for i := range lines {
		lines[i] = ansi.Truncate(lines[i], width, "")
	}
	out := strings.TrimRight(strings.Join(lines, "\n"), "\n")
	if strings.Contains(out, "\x1b[") {
		out += "\x1b[m" // A clipped or supplied style must not color the shell prompt.
	}
	return out, nil
}

// printSession is a glyph-only terminal: it never discovers Kitty, and the
// environment cannot force it on.
func printSession() *ntpicture.TerminalSession {
	return ntpicture.NewTerminalSession(ntpicture.TerminalOptions{
		Kitty:           ntpicture.KittyCapabilityUnsupported,
		SharedMemory:    ntpicture.KittyCapabilityUnsupported,
		CellPixelWidth:  printCellWidth,
		CellPixelHeight: printCellHeight,
	})
}

// settlePrint runs a picture's commands to completion, as the event loop
// would, handing each message back to update. Pictures prepare their glyphs
// off the event loop; printing waits for them. Nothing reaches a terminal:
// a printed picture is only its View.
func settlePrint(cmd tea.Cmd, update func(tea.Msg) tea.Cmd) error {
	pending := []tea.Cmd{cmd}
	for steps := 0; len(pending) > 0; steps++ {
		if steps > 10000 {
			return fmt.Errorf("print view did not settle")
		}
		next := pending[0]
		pending = pending[1:]
		if next == nil {
			continue
		}
		msg := next()
		if msg == nil {
			continue
		}
		// tea.Batch and tea.Sequence both carry their commands as a []tea.Cmd
		// (Sequence's type is unexported); order does not matter here.
		if v := reflect.ValueOf(msg); v.Kind() == reflect.Slice && v.Type().Elem() == reflect.TypeFor[tea.Cmd]() {
			for i := range v.Len() {
				pending = append(pending, v.Index(i).Interface().(tea.Cmd))
			}
			continue
		}
		pending = append(pending, update(msg))
	}
	return nil
}

func printSheet(sheet *document.Sheet, opts Options, width, height int) string {
	v := newSheetView(sheet)
	v.setHidden(opts.Columns.Hidden(sheet))
	return v.view(width, height)
}

func printSafe(text string) string {
	var out strings.Builder
	var state byte
	for len(text) > 0 {
		seq, width, n, next := ansi.DecodeSequence(text, state, nil)
		if n == 0 {
			break
		}
		text, state = text[n:], next
		switch {
		case width > 0 || (len(seq) > 0 && seq[0] >= 0xc0):
			out.WriteString(safe(seq))
		case seq == "\n":
			out.WriteByte('\n')
		case strings.HasPrefix(seq, "\x1b[") && strings.HasSuffix(seq, "m"):
			params := seq[2 : len(seq)-1]
			if strings.IndexFunc(params, func(r rune) bool { return (r < '0' || r > '9') && r != ';' && r != ':' }) < 0 {
				out.WriteString(seq)
			}
		}
	}
	return out.String()
}
