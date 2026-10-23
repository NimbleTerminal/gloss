package app

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"path/filepath"
	"strings"
	"sync"

	tea "charm.land/bubbletea/v2"
	"nimbleterminal.dev/gloss/internal/document"
	"nimbleterminal.dev/gloss/internal/openwith"
)

// pictureItem is a picture the reader can act on: its own file, if it has
// one, and how to get what is on screen.
type pictureItem struct {
	name  string             // For titles and temporary names: the file's base, or the alt text.
	path  string             // Its own file; empty when it has none.
	shown func() image.Image // What is on screen now; called when an entry runs.
	link  string             // The link around it, if any.
	video bool               // Offer Save frame as PNG.
}

// imageCopied and shownSaved are the answers of the commands that copy a
// picture and write it for another app.
type imageCopied struct{ err error }
type shownSaved struct {
	generation uint64
	path       string
	err        error
}

// pictureEntries lists what can be done with p in this host. What cannot
// work here is left out.
func (m *Model) pictureEntries(p pictureItem) []menuEntry {
	var entries []menuEntry
	temporary := p.path == "" || m.opts.IsStdin(p.path) || (m.opts.IsTemporary != nil && m.opts.IsTemporary(p.path))
	if m.opts.OpenWith != nil {
		switch {
		case p.path != "":
			entries = append(entries, menuEntry{"Open with…", func(m *Model) tea.Cmd {
				return m.requestOpenWith(p.path, "", false, true, openwith.Open)
			}})
		case m.opts.Scratch != nil && p.shown != nil:
			entries = append(entries, menuEntry{"Open with…", func(m *Model) tea.Cmd { return m.openShown(p) }})
		}
		if !temporary {
			entries = append(entries, menuEntry{"Edit with…", func(m *Model) tea.Cmd {
				return m.requestOpenWith(p.path, "", false, true, openwith.Edit)
			}})
		}
	}
	if m.opts.CopyImage != nil && p.shown != nil {
		entries = append(entries, menuEntry{"Copy image", func(m *Model) tea.Cmd { return m.copyImage(p.shown()) }})
	}
	if p.path != "" {
		entries = append(entries, menuEntry{"Copy path", func(m *Model) tea.Cmd {
			path, err := filepath.Abs(p.path)
			if err != nil {
				path = p.path
			}
			m.note = "Path copied"
			return tea.SetClipboard(path)
		}})
	}
	if p.video {
		entries = append(entries, menuEntry{"Save frame as PNG", func(m *Model) tea.Cmd { return m.export() }})
	}
	// A picture inside a link also gets the link's entries (the linked
	// figures of Markdown); a whole-view picture has none.
	if p.link != "" {
		entries = append(entries, linkEntries(m, p.link, "Open link")...)
	}
	return entries
}

// laterImage is a picture made on first use, so the command that encodes it
// does the slow part, such as a frame from ffmpeg, off the event loop.
type laterImage struct {
	once sync.Once
	make func() (image.Image, error)
	img  image.Image
	err  error
}

func (l *laterImage) resolve() (image.Image, error) {
	l.once.Do(func() { l.img, l.err = l.make() })
	return l.img, l.err
}
func (l *laterImage) ColorModel() color.Model {
	if img, err := l.resolve(); err == nil {
		return img.ColorModel()
	}
	return color.RGBAModel
}
func (l *laterImage) Bounds() image.Rectangle {
	if img, err := l.resolve(); err == nil {
		return img.Bounds()
	}
	return image.Rectangle{}
}
func (l *laterImage) At(x, y int) color.Color {
	if img, err := l.resolve(); err == nil {
		return img.At(x, y)
	}
	return color.Transparent
}

// encodePNG is shown as a PNG, making it first if it is not made yet.
func encodePNG(shown image.Image) ([]byte, error) {
	if l, ok := shown.(*laterImage); ok {
		img, err := l.resolve()
		if err != nil {
			return nil, err
		}
		shown = img
	}
	if shown == nil || shown.Bounds().Empty() {
		return nil, errors.New("nothing to copy")
	}
	var data bytes.Buffer
	if err := png.Encode(&data, shown); err != nil {
		return nil, err
	}
	return data.Bytes(), nil
}

// copyImage puts shown on the system clipboard as a PNG.
func (m *Model) copyImage(shown image.Image) tea.Cmd {
	copyPNG := m.opts.CopyImage
	if copyPNG == nil {
		return nil
	}
	m.note = "Copying image…"
	return func() tea.Msg {
		data, err := encodePNG(shown)
		if err == nil {
			err = copyPNG(data)
		}
		return imageCopied{err: err}
	}
}

func (m *Model) imageCopied(v imageCopied) tea.Cmd {
	m.note = "Image copied"
	if v.err != nil {
		m.note = "copy image: " + safe(v.err.Error())
	}
	return nil
}

// openShown writes what p shows to a PNG in the viewer's scratch folder and
// asks which app should open it.
func (m *Model) openShown(p pictureItem) tea.Cmd {
	scratch := m.opts.Scratch
	if scratch == nil || p.shown == nil {
		return nil
	}
	name, shown, generation := safe(p.name), p.shown(), m.generation
	if !strings.HasSuffix(strings.ToLower(name), ".png") {
		name += ".png"
	}
	return func() tea.Msg {
		data, err := encodePNG(shown)
		if err != nil {
			return shownSaved{generation: generation, err: err}
		}
		path, err := scratch(name, data)
		return shownSaved{generation: generation, path: path, err: err}
	}
}

func (m *Model) shownSaved(v shownSaved) tea.Cmd {
	if v.generation != m.generation {
		return nil
	}
	if v.err != nil {
		m.note = "open image: " + safe(v.err.Error())
		return nil
	}
	return m.requestOpenWith(v.path, "png", false, true, openwith.Open)
}

// viewPicture is the whole-view item for the file on screen: a picture, a
// page, an animation or a video. A mesh, a table or text is not one.
func (m *Model) viewPicture() (pictureItem, bool) {
	if m.loading || m.err != nil || m.kind == "" || len(m.opts.Files) == 0 {
		return pictureItem{}, false
	}
	if m.markdown != nil || m.typeset != nil || m.sheet != nil || m.chart != nil {
		return pictureItem{}, false
	}
	file := m.opts.Files[m.index]
	p := pictureItem{name: filepath.Base(file), path: file}
	switch {
	case m.opts.IsStdin(file):
		p.name = "stdin"
	case m.kind == "pdf":
		p.name, p.path = exportName(file, m.kind, m.page, false), ""
	}
	switch {
	case m.video != nil:
		p.video = true
		p.shown = func() image.Image {
			q, loader := m.exportRequest(), m.loader
			return &laterImage{make: func() (image.Image, error) {
				return document.ExportImage(loader.Load(q), q.MaxEdge)
			}}
		}
	case m.source != nil:
		p.shown = func() image.Image {
			if m.animation != nil {
				if shown := m.animation.Status().Presented; shown != nil {
					return shown.Image()
				}
			}
			return m.source
		}
	default:
		return pictureItem{}, false
	}
	return p, true
}

// pictureArea is where a picture is drawn, in the body's cells: all of the
// body, less a video's scrub bar on its last row.
func (m *Model) pictureArea() image.Rectangle {
	h := m.bodyHeight()
	if m.video != nil {
		h--
	}
	return image.Rect(0, 0, m.width, h)
}

// itemsHere reports whether the view has things a menu can act on. The
// pointer is swallowed there, whether or not a menu opens.
func (m *Model) itemsHere() bool {
	if m.help || m.layer != layerNone || m.openWithPicker != nil || m.screen != screenDocument || m.isPreview || m.suspended {
		return false
	}
	_, ok := m.viewPicture()
	return ok || m.tabItems()
}

// openItemActions opens the menu of what is at the pointer, or, from the
// keyboard, of the item the view is on.
func (m *Model) openItemActions(at image.Point, pointer bool) tea.Cmd {
	if !m.itemsHere() {
		return nil
	}
	if p, ok := m.viewPicture(); ok {
		if pointer && !at.In(m.pictureArea()) {
			return nil
		}
		if !pointer {
			at = image.Pt(0, 0)
		}
		return m.openActions(p.name, m.pictureEntries(p), at)
	}
	if m.tabItems() {
		return m.markdownItemActions(at, pointer)
	}
	return nil
}
