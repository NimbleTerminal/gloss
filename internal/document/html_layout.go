package document

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"image"
	"io/fs"
	"strings"
	"time"

	nthtml "nimbleterminal.dev/nthtml/document"
)

// An HTML page is laid out as a browser window this wide, in CSS pixels,
// and exported in slices this tall, as typeset text is.
const (
	htmlLayoutWidth = 800
	htmlSliceHeight = 1125
)

// layoutHTML draws a page as its own HTML and CSS lay it out (nthtml),
// rather than as the Markdown taken from it, and gives the slice asked
// for. Pictures are only those the page names beside it, decoded as
// Markdown's are, under the same budget; nothing is fetched.
func layoutHTML(q Request, data []byte, baseDir string, files fs.FS, out *Result) error {
	ctx, cancel := context.WithTimeout(cmp.Or(q.Context, context.Background()), 15*time.Second)
	defer cancel()
	res, err := layOutHTML(ctx, q, data, htmlLayoutWidth, baseDir, files)
	if err != nil {
		if errors.Is(err, nthtml.ErrLimit) {
			return fmt.Errorf("%w: export it with --typeset", err)
		}
		return err
	}
	b := res.Bounds()
	// Each slice ends between lines, not through one.
	breaks := []int{b.Min.Y}
	for top := b.Min.Y; top < b.Max.Y; {
		end := res.PageBreak(top, top+htmlSliceHeight)
		if end <= top {
			end = min(b.Max.Y, top+htmlSliceHeight)
		}
		breaks, top = append(breaks, end), end
	}
	if len(breaks) < 2 {
		breaks = append(breaks, b.Max.Y)
	}
	out.Pages = len(breaks) - 1
	if err := pastEnd(q, out.Pages, "slice", "page"); err != nil {
		return err
	}
	out.Page = max(1, min(q.Page, out.Pages))
	img, err := res.Paint(ctx, image.Rect(b.Min.X, breaks[out.Page-1], b.Max.X, breaks[out.Page]), 1)
	if err != nil {
		return err
	}
	out.Image = img
	return nil
}

// htmlImages decodes the page's pictures: files beside it, and data: URLs.
// One that cannot be had is left out, and the page shows its alt text.
func htmlImages(ctx context.Context, sources []string, baseDir string, files fs.FS) map[string]image.Image {
	images := map[string]image.Image{}
	pixels, encoded := 0, 0
	for _, src := range sources[:min(len(sources), MaxMarkdownImages)] {
		if ctx.Err() != nil || pixels >= maxMarkdownImagePixels {
			break
		}
		var img image.Image
		var err error
		if strings.HasPrefix(src, "data:") {
			img, err = nthtml.DecodeDataURL(src)
		} else {
			var path string
			var data []byte
			if path, err = localMarkdownPathFrom(baseDir, src, files); err == nil {
				data, err = readFileFrom(files, path)
			}
			if encoded += len(data); err == nil && encoded > MaxFileBytes {
				err = ErrTooLarge
			}
			if err == nil {
				img, err = decodeAsset(ctx, path, data)
			}
		}
		if err == nil {
			img, err = ExportImage(Result{Image: img}, 1600)
		}
		if err != nil {
			continue
		}
		if area := img.Bounds().Dx() * img.Bounds().Dy(); pixels+area <= maxMarkdownImagePixels {
			pixels += area
			images[src] = img
		}
	}
	return images
}

// decodeAsset decodes a picture a document names, still: an SVG drawn, or
// the first frame of an animation.
func decodeAsset(ctx context.Context, path string, data []byte) (image.Image, error) {
	kind, err := Detect(path, data, "")
	if err != nil {
		return nil, err
	}
	switch kind {
	case "svg":
		return renderSVG(path, data, 1600)
	case "gif", "webp", "heic", "jxl", "image", "png", "jpeg", "bmp", "tiff":
		return decodeRasterContext(ctx, data)
	}
	return nil, fmt.Errorf("embedded format %s is not an image or SVG", kind)
}
