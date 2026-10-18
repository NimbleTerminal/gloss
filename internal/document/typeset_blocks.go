package document

import (
	"context"
	"image"

	"github.com/alecthomas/chroma/v2/styles"
	"github.com/yuin/goldmark/ast"
	"nimbleterminal.dev/nttypeset"
	ntgoldmark "nimbleterminal.dev/nttypeset/goldmark"
)

// buildBlocks walks a parsed Markdown tree into an nttypeset document. Images
// are looked up by destination; animated destinations are left for the viewer
// to composite. A cancelled ctx or a walk limit is returned as an error.
func buildBlocks(ctx context.Context, root ast.Node, source []byte, images map[string]image.Image, animated map[string]bool) (*nttypeset.Document, error) {
	supplied := make(map[string]nttypeset.Image, len(images)+len(animated))
	for dest, still := range images {
		supplied[dest] = nttypeset.Image{Key: dest, Still: still, Animated: animated[dest]}
	}
	for dest, on := range animated {
		if _, ok := supplied[dest]; !ok && on {
			supplied[dest] = nttypeset.Image{Key: dest, Animated: true}
		}
	}
	return ntgoldmark.Build(ctx, root, source, ntgoldmark.Options{
		Images: supplied,
		Bounds: func(im *ast.Image) nttypeset.Bounds {
			l := MarkdownImageLayout(im)
			return nttypeset.Bounds{Width: l.Width, Height: l.Height, Align: l.Align}
		},
		Syntax: styles.Get("github"),
	})
}
