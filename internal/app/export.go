package app

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"nimbleterminal.dev/gloss/internal/chart"
	"nimbleterminal.dev/gloss/internal/document"
	"nimbleterminal.dev/gloss/internal/i18n"
)

type exportResult struct {
	name string
	size image.Point
	err  error
}

func (r exportResult) note() string {
	if r.err != nil {
		return i18n.Tf("app.export.failed", "export failed: %s", safe(r.err.Error()))
	}
	return i18n.Tf("app.export.saved", "saved %s · %d×%d", safe(r.name), r.size.X, r.size.Y)
}

// exportRequest asks the loader for the page as it is on screen: the same
// parts and paint, at export size.
func (m *Model) exportRequest() document.Request {
	q := m.request(false)
	if m.typeset != nil {
		q.TypesetWidth = m.typesetWidth
	}
	q.Preview, q.Animate, q.MaxEdge = false, false, m.opts.MaxEdge
	if m.video != nil {
		// The frame on screen, not the first; at the end, the one before it,
		// since ffmpeg finds no frame at or after the end.
		s := m.video.Status()
		q.At = s.CurrentTime
		if s.Duration > 0 && q.At >= s.Duration {
			q.At = max(0, s.Duration-time.Second/10)
		}
	}
	if q.MaxEdge == 0 {
		q.MaxEdge = 1536
	}
	return q
}

// export renders the current page as --output would, from the source rather
// than the terminal's cells. A mesh keeps the camera it is being viewed from.
func (m *Model) export() tea.Cmd {
	if m.loading || m.err != nil || m.kind == "" {
		return nil
	}
	var frame image.Image
	if m.markdown != nil {
		frame = m.markdown.animatedFrame()
	}
	if m.markdown != nil && frame == nil {
		m.note = i18n.T("app.export.textView", "text view cannot be exported as PNG")
		if m.typeset != nil {
			m.note += m.typesetKeyNote("s")
		} else if m.canTypeset() {
			m.note += m.typesetKeyNote("t")
		}
		return nil
	}
	if p := m.chartPane; p != nil && p.shown() {
		// The chart as it stands, cell for cell, at export size.
		q := m.exportRequest()
		name := strings.TrimSuffix(exportName(q.Path, m.kind, m.page, q.Stdin), ".png")
		if p.doc == nil {
			name += "-chart"
		}
		name += ".png"
		save := m.opts.Save
		if save == nil {
			save = func(name string, data []byte) (string, error) { return saveFile(".", name, data) }
		}
		frame, cols, rows := p.view, p.width, p.height
		pal := m.palette()
		var graphics *chart.Document
		if m.chartGraphics && p.img != nil && p.compiled != nil {
			graphics = p.compiled
		}
		m.note = i18n.T("app.export.running", "exporting…")
		return func() tea.Msg {
			var img image.Image
			if graphics != nil {
				// Graphics are drawn again at the export cell's size.
				var err error
				if img, err = chart.Default.Raster(context.Background(), *graphics, cols*document.ChartCellWidth, rows*document.ChartCellHeight); err != nil {
					return exportResult{err: err}
				}
			} else {
				img = document.ChartImage(frame, cols, rows, document.ChartCellWidth, document.ChartCellHeight, pal.Foreground, pal.Background)
			}
			var data bytes.Buffer
			if err := png.Encode(&data, img); err != nil {
				return exportResult{err: err}
			}
			name, err := save(name, data.Bytes())
			return exportResult{name: name, size: img.Bounds().Size(), err: err}
		}
	}
	if m.sheet != nil {
		m.note = i18n.T("app.export.sheet", "spreadsheets cannot be exported as PNG")
		return nil
	}
	q := m.exportRequest()
	edge := q.MaxEdge
	name, save := exportName(q.Path, m.kind, m.page, q.Stdin), m.opts.Save
	if save == nil {
		save = func(name string, data []byte) (string, error) { return saveFile(".", name, data) }
	}
	view := document.Result{CPU: m.exportOnCPU()}
	if m.chart != nil {
		camera := m.chart.Camera()
		view.Camera = &camera
	}
	at := m.mapExportView()
	if m.typeset != nil && m.markdown == nil {
		frame = m.typesetExportFrame()
	}
	if m.animation != nil {
		if shown := m.animation.Status().Presented; shown != nil {
			frame = shown.Image()
		}
	}
	loader := m.loader
	m.note = i18n.T("app.export.running", "exporting…")
	return func() tea.Msg {
		var r document.Result
		if frame != nil {
			r.Image = frame
		} else {
			r = loader.Load(q)
		}
		r.Camera, r.CPU = view.Camera, view.CPU
		if r.Map != nil && at != nil {
			r.Map.View = at
		}
		img, err := document.ExportImage(r, edge)
		if err != nil {
			return exportResult{err: err}
		}
		var data bytes.Buffer
		if err := png.Encode(&data, img); err != nil {
			return exportResult{err: err}
		}
		name, err := save(name, data.Bytes())
		return exportResult{name: name, size: img.Bounds().Size(), err: err}
	}
}

// exportOnCPU reports whether a renderer other than the GPU was asked for.
func (m *Model) exportOnCPU() bool { return m.opts.Render3D != "auto" && m.opts.Render3D != "" }

func exportName(path, kind string, page int, stdin bool) string {
	base := filepath.Base(path)
	if ext := filepath.Ext(base); ext != base {
		base = strings.TrimSuffix(base, ext)
	}
	if stdin {
		base = "stdin"
	}
	if kind == "pdf" {
		base += fmt.Sprintf("-page-%d", page)
	}
	return base + ".png"
}

// saveFile never replaces a file: a taken name gains a number instead.
func saveFile(dir, name string, data []byte) (string, error) {
	written, err := document.WriteNew(filepath.Join(dir, name), data)
	if err != nil {
		return "", err
	}
	return filepath.Base(written), nil
}
