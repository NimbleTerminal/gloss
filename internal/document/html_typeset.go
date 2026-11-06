package document

import (
	"io/fs"

	"github.com/alecthomas/chroma/v2/styles"
	htmltypeset "nimbleterminal.dev/nthtml/typeset"
)

// typesetHTML exports the reading subset through the same layout, fonts,
// pagination and raster path as Markdown and text. It never converts HTML to
// Markdown. Interactive HTML keeps its CSS page layout and source navigation.
func typesetHTML(q Request, data []byte, base string, files fs.FS, file []Field) Result {
	fail := func(err error) Result { return Result{Kind: "html", Err: err, Generation: q.Generation} }
	doc, opts, err := prepareHTML(q.Context, q, data, htmlLayoutWidthFor(q.TypesetWidth), base, files)
	if err != nil {
		return fail(err)
	}
	reading, sources, err := htmltypeset.BuildWithSources(q.Context, doc, htmltypeset.Options{Document: opts, Syntax: styles.Get("github")})
	if err != nil {
		return fail(err)
	}
	eng, err := newTypesetEngine(q.Font, q.Mono)
	if err != nil {
		return fail(err)
	}
	q.Theme.Palette = sources.Palette()
	// A private loader keeps export state separate from the CSS viewer and its
	// text/search fallback. Subsequent exports may use another theme or width.
	l := &Loader{}
	defer l.Close()
	return l.startTypeset(q, data, file, "html", &Markdown{}, "", Section("Page", Field{"Format", "HTML"}), eng, reading)
}
