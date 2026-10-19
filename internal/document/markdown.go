package document

import (
	"context"
	"fmt"
	"image"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/yuin/goldmark/ast"
	"nimbleterminal.dev/ntpicture/animation"
)

const MaxMarkdownBytes = 2 << 20
const MaxMarkdownImages = 32
const maxMarkdownImagePixels = 16 << 20

type Markdown struct {
	Animations, Animated, AnimationOverBudget int // GIFs and animated WebPs: seen, playing, and left on their first frame.
	Source                                    []byte
	Images                                    []MarkdownImage
	// The pictures are inside the file, as a notebook's outputs and a Word
	// document's media are: their links name no file a reader can open.
	Packaged bool
	// Diagrams counts Mermaid diagrams, drawn in the palette of the theme the
	// document was loaded with; a change between light and dark reloads it.
	Diagrams int
}

// Picture is one picture of packaged Markdown, as a file beside its text
// would hold it.
type Picture struct {
	Destination string // The link as the Markdown has it.
	Name        string // A file name for it, unique among the pictures, ending .png.
	Image       image.Image
}

// Pictures lists the pictures of packaged Markdown that could be read, each
// once; Markdown whose pictures are files beside it has none to list.
func (md *Markdown) Pictures() []Picture {
	if md == nil || !md.Packaged {
		return nil
	}
	var out []Picture
	seen, taken := map[string]bool{}, map[string]bool{}
	for _, img := range md.Images {
		if img.Image == nil || seen[img.Destination] {
			continue
		}
		seen[img.Destination] = true
		stem := strings.TrimSuffix(path.Base(img.Destination), path.Ext(img.Destination))
		name := stem + ".png"
		for n := 2; taken[name]; n++ {
			name = fmt.Sprintf("%s-%d.png", stem, n)
		}
		taken[name] = true
		out = append(out, Picture{Destination: img.Destination, Name: name, Image: img.Image})
	}
	return out
}

// Relink points the links of the Markdown's pictures elsewhere: to, by
// destination, gives where.
func (md *Markdown) Relink(to map[string]string) []byte {
	pairs := make([]string, 0, 2*len(to))
	for from, where := range to {
		pairs = append(pairs, "]("+from, "]("+where)
	}
	return []byte(strings.NewReplacer(pairs...).Replace(string(md.Source)))
}

type MarkdownImage struct {
	Layout           ImageLayout
	Destination, Alt string
	Image            image.Image
	Animation        *animation.Player
	Err              error
}

// MarkdownTree uses the same dialect for asset discovery and Glamour layout.
func MarkdownTree(source []byte) ast.Node {
	root, err := readMarkdownTree(source)
	if err != nil {
		return ast.NewDocument()
	}
	return root
}

func loadMarkdown(path string, data []byte, baseDir string) (*Markdown, error) {
	return loadMarkdownFrom(path, data, baseDir, nil)
}

func loadMarkdownFrom(path string, data []byte, baseDir string, files fs.FS, playback ...bool) (*Markdown, error) {
	return loadMarkdownContext(context.Background(), path, data, baseDir, files, len(playback) > 0 && playback[0], false)
}

// loadMarkdownContext loads Markdown and its images; dark draws Mermaid
// diagrams for a dark theme.
func loadMarkdownContext(ctx context.Context, path string, data []byte, baseDir string, files fs.FS, play, dark bool) (*Markdown, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if len(data) > MaxMarkdownBytes {
		return nil, fmt.Errorf("Markdown exceeds 2 MiB")
	}
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("Markdown must be UTF-8")
	}
	source := cleanMarkdownSource(data)
	if err := checkMarkdownNesting(source); err != nil {
		return nil, err
	}
	md := &Markdown{Source: []byte(source)}
	if baseDir == "" {
		baseDir = filepath.Dir(path)
	}
	cache := make(map[string]MarkdownImage)
	pixels, encodedBytes := 0, 0
	var animations markdownAnimations
	root, err := readMarkdownTree(md.Source)
	if err != nil {
		return nil, err
	}
	_ = ast.Walk(root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if err := ctx.Err(); err != nil {
			return ast.WalkStop, err
		}
		img, ok := n.(*ast.Image)
		if !entering || !ok {
			return ast.WalkContinue, nil
		}
		if len(md.Images) >= MaxMarkdownImages {
			return ast.WalkSkipChildren, nil
		}
		dest := string(img.Destination)
		if code, ok := mermaidSource(img); ok {
			md.Diagrams++
			asset, cached := cache[dest]
			if !cached {
				asset.Destination = dest
				asset.Image, asset.Err = renderMermaidImage(img, code, dark)
				if asset.Err == nil {
					area := asset.Image.Bounds().Dx() * asset.Image.Bounds().Dy()
					if pixels+area > maxMarkdownImagePixels {
						asset.Image, asset.Err = nil, fmt.Errorf("document image budget reached")
					} else {
						pixels += area
					}
				}
				asset.Layout = MarkdownImageLayout(img)
				cache[dest] = asset
			}
			asset.Alt = inlineText(img, md.Source)
			md.Images = append(md.Images, asset)
			return ast.WalkSkipChildren, nil
		}
		asset, cached := cache[dest]
		if !cached {
			asset.Destination = dest
			resolved, err := localMarkdownPathFrom(baseDir, dest, files)
			asset.Err = err
			if err == nil && pixels >= maxMarkdownImagePixels {
				asset.Err = fmt.Errorf("document image budget reached")
			}
			if asset.Err == nil {
				// Reuse the same bounded image/SVG decoders as standalone files.
				data, err := readFileFrom(files, resolved)
				asset.Err = err
				encodedBytes += len(data)
				if encodedBytes > MaxFileBytes {
					asset.Err = ErrTooLarge
					err = ErrTooLarge
				}
				if err == nil {
					kind, err := Detect(resolved, data, "")
					asset.Err = err
					if err == nil {
						switch kind {
						case "svg":
							asset.Image, asset.Err = renderSVG(resolved, data, 1600)
						case "gif", "webp":
							asset.Image, asset.Animation, asset.Err = animations.decode(ctx, data, play)
						case "heic", "jxl", "image", "png", "jpeg", "bmp", "tiff":
							asset.Image, asset.Err = decodeRaster(data)
						default:
							asset.Err = fmt.Errorf("embedded format %s is not an image or SVG", kind)
						}
					}
				}
				if asset.Err == nil {
					asset.Image, asset.Err = ExportImage(Result{Image: asset.Image}, 1600)
					if asset.Err == nil {
						area := asset.Image.Bounds().Dx() * asset.Image.Bounds().Dy()
						if pixels+area > maxMarkdownImagePixels {
							asset.Image = nil
							asset.Err = fmt.Errorf("document image budget reached")
						} else {
							pixels += area
						}
					}
				}
			}
			if asset.Err != nil {
				asset.Animation = nil
			}
			cache[dest] = asset
		}
		asset.Layout = MarkdownImageLayout(img)
		asset.Alt = inlineText(img, md.Source)
		md.Images = append(md.Images, asset)
		return ast.WalkSkipChildren, nil
	})
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	animations.apply(md)
	return md, nil
}

// inlineText is the text of a node's inline children, what the deprecated
// ast.BaseNode.Text gathered.
func inlineText(n ast.Node, source []byte) string {
	var b strings.Builder
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		switch t := c.(type) {
		case *ast.Text:
			b.Write(t.Value(source))
			if t.SoftLineBreak() {
				b.WriteByte('\n')
			}
		case *ast.String:
			b.Write(t.Value)
		default:
			b.WriteString(inlineText(c, source))
		}
	}
	return b.String()
}

func localMarkdownPathFrom(base, destination string, files fs.FS) (string, error) {
	u, err := url.Parse(destination)
	if err != nil {
		return "", fmt.Errorf("invalid image path: %w", err)
	}
	if (u.Scheme != "" && u.Scheme != "file") || (u.Host != "" && !(u.Scheme == "file" && u.Host == "localhost")) {
		return "", fmt.Errorf("remote and data images are not loaded")
	}
	if u.Path == "" {
		return "", fmt.Errorf("empty image path")
	}
	path := filepath.FromSlash(u.Path)
	if !filepath.IsAbs(path) {
		path = filepath.Join(base, path)
	}
	var info fs.FileInfo
	if files == nil {
		info, err = os.Stat(path)
	} else {
		info, err = fs.Stat(files, filepath.ToSlash(path))
	}
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("image is not a regular file")
	}
	if files != nil {
		path = filepath.ToSlash(path)
	}
	return path, nil
}

// cleanMarkdownSource keeps terminal controls out of the source viewer.
func cleanMarkdownSource(data []byte) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return r
		}
		if r == '\r' {
			return -1
		}
		// Preserve joiners used by emoji and shaped text while still
		// excluding terminal controls and bidirectional overrides.
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) && r != '\u200c' && r != '\u200d' {
			return '\ufffd'
		}
		return r
	}, string(data))
}
