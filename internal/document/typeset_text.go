package document

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"unicode/utf8"

	"nimbleterminal.dev/gloss/internal/theme"
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
	prep := &prepared{ctx: q.Context, lines: true}
	// One block per line bounds shaping and preserves blank lines and indentation.
	// Highlight the complete source first so multiline tokens retain their color.
	spans := []span{{text: strings.TrimSuffix(text, "\n")}}
	if language != "" {
		spans = highlightCodeContext(q.Context, strings.TrimSuffix(text, "\n"), language, codeTheme)
	}
	b := block{kind: "line", size: 25}
	for _, run := range spans {
		for i, line := range strings.Split(run.text, "\n") {
			if i > 0 {
				if len(b.spans) == 0 {
					b.spans = []span{{text: " "}}
				}
				prep.blocks = append(prep.blocks, b)
				b = block{kind: "line", size: 25}
			}
			part := run
			part.text = strings.ReplaceAll(line, "\t", "    ")
			if part.text != "" {
				b.spans = append(b.spans, part)
			}
		}
	}
	if len(b.spans) == 0 {
		b.spans = []span{{text: " "}}
	}
	prep.blocks = append(prep.blocks, b)
	if len(prep.blocks) > 8192 {
		return fail(fmt.Errorf("typeset text exceeds 8,192 lines"))
	}
	eng, err := newEngine(q.Font, q.Mono)
	if err != nil {
		return fail(err)
	}
	return l.startTypeset(q, data, file, kind, source, text, info, eng, prep)
}

func (l *Loader) startTypeset(q Request, data []byte, file []Field, kind string, source *Markdown, text string, info []Field, eng *engine, prep *prepared) Result {
	l.path, l.file = q.Path, file
	l.typesetAnimate = q.Animate && !q.Preview
	l.typeset, l.typesetBase, l.typesetHash = source, q.BaseDir, sha256.Sum256(data)
	l.typesetDark = theme.Dark(q.Theme)
	l.typesetType, l.typesetKind, l.typesetText, l.typesetInfo = q.Type, kind, text, info
	l.typesetEngine, l.typesetPrepared = eng, prep
	return l.renderTypeset(q)
}
