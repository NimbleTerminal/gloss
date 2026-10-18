package document

import (
	"bytes"
	"embed"
	"fmt"
	"image"
	"io/fs"
	"os"
	"path/filepath"
	"unicode/utf8"

	"github.com/yuin/goldmark/ast"
	"nimbleterminal.dev/ntpicture/animation"
)

// Default export measure: 800 pixels wide at 150 DPI. Viewer requests can
// supply a bounded width; DPI affects raster detail, not the reading measure.
const typesetWidth = 384

func typesetMeasure(width int) int {
	if width == 0 {
		return typesetWidth
	}
	return max(192, min(width, 960))
}

//go:embed fonts/Gloss*.ttf
var typesetFonts embed.FS

type typesetAsset struct {
	data      []byte
	svg       bool
	image     image.Image
	animation *animation.Player
	gif       bool
	ready     bool        // Drawn during preflight, as a Mermaid diagram is.
	layout    ImageLayout // A drawn diagram's reading size.
}

// prepareTypeset finishes validation before constructing a layout. In particular,
// budgets count references (including duplicates) and decoded, not resized pixels.
func prepareTypeset(q Request, data []byte, files fs.FS) (*Markdown, ast.Node, map[string]*typesetAsset, error) {
	if len(data) > MaxMarkdownBytes {
		return nil, nil, nil, fmt.Errorf("Markdown exceeds 2 MiB")
	}
	if !utf8.Valid(data) {
		return nil, nil, nil, fmt.Errorf("Markdown must be UTF-8")
	}
	if err := checkMarkdownNesting(string(data)); err != nil {
		return nil, nil, nil, err
	}
	root, err := readMarkdownTree(data)
	if err != nil {
		return nil, nil, nil, err
	}
	base := q.BaseDir
	if base == "" {
		base = filepath.Dir(q.Path)
		if q.Stdin {
			base, err = os.Getwd()
			if err != nil {
				return nil, nil, nil, err
			}
		}
	}
	assets := map[string]*typesetAsset{}
	var destinations []string
	var undrawn []*ast.Image // Mermaid diagrams to put back as their source.
	references, pixels, encodedBytes, nodes := 0, 0, 0, 0
	err = ast.Walk(root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if q.Context != nil {
			if err := q.Context.Err(); err != nil {
				return ast.WalkStop, err
			}
		}
		if entering {
			nodes++
			if nodes > 32768 {
				return ast.WalkStop, fmt.Errorf("typeset document exceeds 32,768 nodes")
			}
		}
		im, ok := n.(*ast.Image)
		if !entering || !ok {
			return ast.WalkContinue, nil
		}
		references++
		if references > MaxMarkdownImages {
			return ast.WalkStop, fmt.Errorf("typeset Markdown exceeds %d image references", MaxMarkdownImages)
		}
		dest := string(im.Destination)
		if code, ok := mermaidSource(im); ok {
			if a, seen := assets[dest]; seen {
				if a != nil {
					im.SetAttributeString("gloss-image-layout", a.layout)
				} else {
					undrawn = append(undrawn, im)
				}
				return ast.WalkContinue, nil
			}
			destinations = append(destinations, dest)
			assets[dest] = nil
			img, e := renderMermaidImage(im, code)
			if e != nil {
				undrawn = append(undrawn, im)
				return ast.WalkContinue, nil
			}
			count := img.Bounds().Dx() * img.Bounds().Dy()
			if count > maxMarkdownImagePixels-pixels {
				return ast.WalkStop, fmt.Errorf("typeset Markdown exceeds 16 Mi decoded image pixels")
			}
			pixels += count
			assets[dest] = &typesetAsset{image: img, ready: true, layout: MarkdownImageLayout(im)}
			return ast.WalkContinue, nil
		}
		if _, ok := assets[dest]; ok {
			return ast.WalkContinue, nil
		}
		destinations = append(destinations, dest)
		assets[dest] = nil // Every unavailable or non-local image gets a placeholder.
		name, e := localMarkdownPathFrom(base, dest, files)
		if e != nil {
			return ast.WalkContinue, nil
		}
		b, e := readFileFrom(files, name)
		if e != nil {
			if e == ErrTooLarge {
				return ast.WalkStop, e
			}
			return ast.WalkContinue, nil
		}
		encodedBytes += len(b)
		if encodedBytes > MaxFileBytes {
			return ast.WalkStop, fmt.Errorf("typeset Markdown assets exceed 128 MiB")
		}
		kind, e := Detect(name, b, "")
		if e != nil {
			return ast.WalkContinue, nil
		}
		a := &typesetAsset{data: b, svg: kind == "svg", gif: kind == "gif"}
		count := 1600 * 1600 // SVG rasterization is bounded by this box.
		if !a.svg {
			cfg, _, e := image.DecodeConfig(bytes.NewReader(b))
			if e != nil {
				return ast.WalkContinue, nil
			}
			if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > maxMarkdownImagePixels/cfg.Height {
				return ast.WalkStop, fmt.Errorf("typeset Markdown exceeds 16 Mi decoded image pixels")
			}
			count = cfg.Width * cfg.Height
		}
		if count > maxMarkdownImagePixels-pixels {
			return ast.WalkStop, fmt.Errorf("typeset Markdown exceeds 16 Mi decoded image pixels")
		}
		pixels += count
		assets[dest] = a
		return ast.WalkContinue, nil
	})
	if err != nil {
		return nil, nil, nil, err
	}
	for _, im := range undrawn {
		restoreMermaidSource(im)
	}
	var animations markdownAnimations
	for _, dest := range destinations {
		a := assets[dest]
		if q.Context != nil {
			if err := q.Context.Err(); err != nil {
				return nil, nil, nil, err
			}
		}
		if a == nil || a.ready {
			continue
		}
		if a.svg {
			a.image, err = renderSVG(dest, a.data, 1600)
		} else if a.gif {
			a.image, a.animation, err = animations.decode(q.Context, a.data, q.Animate && !q.Preview)
		} else {
			a.image, err = decodeRaster(a.data)
		}
		a.data = nil
		if err != nil {
			assets[dest] = nil
		}
	}
	md := &Markdown{Source: []byte(cleanMarkdownSource(data))}
	animations.apply(md)
	return md, root, assets, nil
}
