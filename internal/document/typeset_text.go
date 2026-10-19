package document

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/alecthomas/chroma/v2/styles"
	"nimbleterminal.dev/gloss/internal/theme"
	"nimbleterminal.dev/nttypeset"
)

// Plain input goes directly to styled spans, never through Markdown parsing.
// The fenced representation is only for the existing terminal text view.
func (l *Loader) loadTypesetText(q Request, data []byte, file []Field, kind string) Result {
	fail := func(err error) Result { return Result{Generation: q.Generation, Kind: kind, Err: err} }
	if len(data) > MaxMarkdownBytes || !utf8.Valid(data) {
		return fail(fmt.Errorf("typeset text requires at most 2 MiB of UTF-8 input"))
	}
	var text, language string
	var source *Markdown
	var info []Field
	if kind == "json" {
		doc, err := ReadJSON(q.Path, data)
		if err != nil {
			return fail(err)
		}
		if doc.cut || (doc.Lines && doc.Shown < doc.Records) {
			return fail(fmt.Errorf("typeset JSON exceeds the formatted text budget"))
		}
		text, language, source, info = doc.Plain, "json", &Markdown{Source: doc.Markdown}, doc.fields()
	} else {
		doc, err := ReadText(q.Path, data)
		if err != nil {
			return fail(err)
		}
		if len(strings.TrimSuffix(doc.Plain, "\n")) > MaxMarkdownBytes-4096 || doc.shown > 0 || len(doc.Markdown) > MaxMarkdownBytes {
			return fail(fmt.Errorf("typeset text exceeds the source view budget"))
		}
		text, language, source, info = doc.Plain, doc.language, &Markdown{Source: doc.Markdown}, doc.fields()
	}
	if len(text) > MaxMarkdownBytes {
		return fail(fmt.Errorf("typeset text exceeds 2 MiB"))
	}
	if strings.Count(strings.TrimSuffix(text, "\n"), "\n")+1 > 8192 {
		return fail(fmt.Errorf("typeset text exceeds 8,192 lines"))
	}
	doc, err := nttypeset.Lines(q.Context, text, language, styles.Get("github"))
	if err != nil {
		return fail(err)
	}
	eng, err := newTypesetEngine(q.Font, q.Mono)
	if err != nil {
		return fail(err)
	}
	return l.startTypeset(q, data, file, kind, source, text, info, eng, doc)
}

func (l *Loader) startTypeset(q Request, data []byte, file []Field, kind string, source *Markdown, text string, info []Field, eng *nttypeset.Engine, doc *nttypeset.Document) Result {
	l.path, l.file = q.Path, file
	l.typesetAnimate = q.Animate && !q.Preview
	l.typeset, l.typesetBase, l.typesetHash = source, q.BaseDir, sha256.Sum256(data)
	l.typesetDark = theme.Dark(q.Theme)
	l.typesetType, l.typesetKind, l.typesetText, l.typesetInfo = q.Type, kind, text, info
	l.typesetEngine, l.typesetDocument = eng, doc
	return l.renderTypeset(q)
}
