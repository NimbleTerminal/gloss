package document

import (
	"context"
	"fmt"
	"image"
	"strings"

	"nimbleterminal.dev/gloss/internal/document/fonts"
	"nimbleterminal.dev/nttypeset"
)

// FontPreview draws the four bounded sample rows with nttypeset, the same
// shaper and rasterizer as the document. It is called by an asynchronous viewer command.
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
	img, err := nttypeset.Preview(ctx, FontLooks(), override, body, code, width, height, typesetPalette(themeName), nttypeset.WithFonts(fonts.Embedded))
	if err != nil {
		return nil, err
	}
	return img, nil
}
