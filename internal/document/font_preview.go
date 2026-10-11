package document

import (
	"context"
	"fmt"
	"image"
	"image/draw"
	"strings"

	"github.com/NimbleTerminal/gloss/internal/theme"
)

// FontPreview draws the four bounded sample rows using the same shaper and
// rasterizer as the document. It is called by an asynchronous viewer command.
func FontPreview(ctx context.Context, source []byte, palette, override string, width, height int) (out image.Image, err error) {
	if width < 160 || height < 160 || width > 1200 || height > 1200 {
		return nil, fmt.Errorf("font preview does not fit")
	}
	defer func() {
		if p := recover(); p != nil {
			err = typesetError(p)
		}
	}()
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
	result := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(result, result.Bounds(), image.NewUniform(theme.Get(palette).Background), image.Point{}, draw.Src)
	rowHeight := height / 4
	for i, look := range fontLooks {
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		e, err := newEngine(look.Name, override)
		if err != nil {
			return nil, err
		}
		e.ctx = ctx
		e.theme = palette
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
		row := l.raster(0, rowHeight, 0, width, 1)
		draw.Draw(result, image.Rect(0, i*rowHeight, width, (i+1)*rowHeight), row, image.Point{}, draw.Src)
	}
	return result, nil
}
