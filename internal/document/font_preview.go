package document

import (
	"context"
	"fmt"
	"image"
	"image/draw"
	"strings"
)

// FontPreview draws the four bounded sample rows using the same shaper and
// rasterizer as the document. It is called by an asynchronous viewer command.
func FontPreview(ctx context.Context, source []byte, themeName, override string, width, height int) (image.Image, error) {
	if width < 160 || height < 160 || width > 1200 || height > 1200 {
		return nil, fmt.Errorf("font preview does not fit")
	}
	body, code := "", ""
	fenced := false
	for _, line := range strings.Split(string(source), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~") {
			fenced = !fenced
			continue
		}
		if line == "" {
			continue
		}
		if fenced {
			if code == "" {
				code = line
			}
		} else if body == "" {
			body = strings.TrimLeft(line, "#>*- ")
		}
		if body != "" && code != "" {
			break
		}
	}
	trim := func(s string) string {
		r := []rune(s)
		if len(r) > 120 {
			return string(r[:117]) + "…"
		}
		return s
	}
	body, code = trim(body), trim(code)
	build := func(name, override string) (*engine, error) { return newEngine(name, override) }
	return previewRows(ctx, fontLooks, override, body, code, width, height, paletteFor(themeName), build)
}

// previewRows draws one sample row per look: the look's name in bold, then the
// body and code strings, each on a single line, rasterized with the document's
// shaper. It is a cancellation entry point and turns panics into errors.
func previewRows(ctx context.Context, looks []FontLook, override, body, code string, width, height int, p palette, newEngine func(name, override string) (*engine, error)) (out *image.RGBA, err error) {
	defer func() {
		if r := recover(); r != nil {
			out, err = nil, typesetError(r)
		}
	}()
	result := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(result, result.Bounds(), image.NewUniform(p.background), image.Point{}, draw.Src)
	rowHeight := height / 4
	for i, look := range looks {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		e, err := newEngine(look.Name, override)
		if err != nil {
			return nil, err
		}
		row := func() *image.RGBA {
			defer e.enter(ctx)()
			l := e.layout(&prepared{}, width)
			size := max(10, min(24, rowHeight/5))
			y := 4
			for j, s := range []string{strings.ToUpper(look.Name[:1]) + look.Name[1:], body, code} {
				if s == "" {
					continue
				}
				style := 0
				if j == 0 {
					style = bold
				}
				if j == 2 {
					style = mono
				}
				ops, _ := l.text([]span{{text: s, style: style}}, size, 10, y, width-20)
				// One wrapped line per sample; keep all runs of its first baseline.
				if len(ops) > 0 {
					baseline := ops[0].baseline
					for _, op := range ops {
						if op.baseline == baseline {
							l.ops = append(l.ops, op)
						}
					}
				}
				y += size*3/2 + 2
			}
			return l.raster(ctx, 0, rowHeight, 0, width, 1, p)
		}()
		draw.Draw(result, image.Rect(0, i*rowHeight, width, (i+1)*rowHeight), row, image.Point{}, draw.Src)
	}
	return result, nil
}
