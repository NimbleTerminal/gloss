package document

import (
	"context"
	"fmt"

	"github.com/alecthomas/chroma/v2/styles"
	"nimbleterminal.dev/nttypeset"
)

// Large interactive text is a lazy sequence of bounded AST documents. The
// shared chunk cache also accepts semantic AST sources (e.g. chat messages);
// neither the parser's whole-file limits nor Layout's limits are raised.
func (l *Loader) loadTypesetChunks(q Request, data []byte, file []Field) Result {
	fail := func(err error) Result { return Result{Generation: q.Generation, Kind: "text", Err: err} }
	doc, err := ReadText(q.Path, data)
	if err != nil {
		return fail(err)
	}
	// Apply the same terminal-control normalization as the regular source view.
	text := cleanMarkdownSource([]byte(doc.Plain))
	source, err := nttypeset.NewLineChunks(q.Context, text, doc.language, styles.Get("github"))
	if err != nil {
		return fail(err)
	}
	eng, err := newTypesetEngine(q.Font, q.Mono)
	if err != nil {
		return fail(err)
	}
	chunks, err := eng.NewChunked(source)
	if err != nil {
		return fail(err)
	}
	l.typesetLineChunks, l.typesetChunks = source, chunks
	// The text-mode representation remains bounded; typeset traversal includes
	// every line, so its info panel must not claim the text-mode prefix is all.
	doc.shown = 0
	return l.startTypeset(q, data, file, "text", &Markdown{Source: doc.Markdown}, text, doc.fields(q.Locale), eng, nil)
}

func (l *Loader) renderTypesetChunks(q Request) Result {
	chunks, source := l.typesetChunks, l.typesetLineChunks
	fail := func(err error) Result { return Result{Generation: q.Generation, Kind: l.typesetKind, Err: err} }
	if q.TypesetView == nil {
		return fail(fmt.Errorf("large typeset text requires an interactive viewport"))
	}
	if q.Context == nil {
		q.Context = context.Background()
	}
	if err := q.Context.Err(); err != nil {
		return fail(err)
	}
	if _, err := setTypesetFonts(l.typesetEngine, q.Font, q.Mono); err != nil {
		return fail(err)
	}
	v := *q.TypesetView
	index := max(0, min(v.Chunk, chunks.Chunks()-1))
	if v.End {
		index = chunks.Chunks() - 1
	}
	globalLine := v.Line
	if globalLine > 0 {
		index = source.ChunkForLine(globalLine)
	}
	if v.Top < 0 && index > 0 && !v.End && globalLine == 0 {
		index--
		v.End = true
	}
	width := typesetLayoutWidth(q)
	// At most one boundary is crossed per request. Even a large scroll delta
	// cannot force a pass over the whole history before producing a viewport.
	for attempt := 0; attempt < 2; attempt++ {
		first, _ := source.LineRange(index)
		options := nttypeset.LayoutOptions{Numbers: q.TypesetLineNumbers, Chop: q.TypesetChop, LineOffset: first - 1, LineCount: source.Lines()}
		chunk, err := chunks.At(q.Context, index, width, options)
		if err != nil {
			return fail(err)
		}
		// Preserve the visible block across font, gutter, chop and width changes.
		old := l.direct
		previous := l.typesetChunkShown
		if previous != nil && previous.Chunk == index && old != nil && old != chunk.Layout && v.Top > 0 && !v.End && globalLine == 0 {
			was := old.Starts()
			block := old.LineAt(v.Top) - 1
			if err := chunk.Layout.EnsureBlock(q.Context, block); err != nil {
				return fail(err)
			}
			if starts := chunk.Layout.Starts(); block < len(starts) && block < len(was) {
				// LineAt includes the reading margin; a negative offset keeps
				// a go-to-line target below that margin after reflow.
				offset := v.Top - was[block]
				bounds := chunk.Layout.Bounds()
				if block < len(bounds) && !bounds[block].Empty() {
					offset = min(offset, max(0, bounds[block].Max.Y-starts[block]-1))
				}
				v.Top = max(0, starts[block]+offset)
			}
		}
		l.typesetDocument, l.direct, l.directOptions = chunk.Document, chunk.Layout, options
		l.typesetLineOffset, l.typesetLineCount = first-1, source.Lines()
		local := q
		local.TypesetView = &v
		if globalLine > 0 {
			v.Line = max(1, min(globalLine-first+1, len(chunk.Document.Blocks)))
		}
		if index != q.TypesetView.Chunk {
			// A selection's places and picture rectangles belong to the displayed
			// chunk. Never reinterpret them as places in a neighboring AST.
			local.TypesetSelect, local.TypesetFocus, local.TypesetMarks = nil, nil, nil
		}
		// Reuse the normal renderer with this bounded document and shared engine.
		result := l.renderTypesetPage(local)
		if result.Err != nil {
			return result
		}
		shown := result.TypesetView
		if shown == nil {
			return fail(fmt.Errorf("typeset chunk returned no viewport"))
		}
		bottom := max(0, result.TypesetExtent-shown.Height)
		atBottom := previous != nil && previous.Chunk == index && previous.Top >= bottom
		sameLayout := old == chunk.Layout && previous != nil && previous.Height == shown.Height && previous.Zoom == shown.Zoom
		if attempt == 0 && atBottom && sameLayout && !v.End && globalLine == 0 && result.TypesetDone && v.Top > bottom && index+1 < chunks.Chunks() {
			index++
			v.Top, v.Left, v.Line = 0, 0, 0
			continue
		}
		shown.Chunk, shown.Chunks = index, chunks.Chunks()
		shown.Line += first - 1
		shown.Lines = source.Lines()
		if index != q.TypesetView.Chunk && q.TypesetSelect != nil {
			asked := *q.TypesetSelect
			result.TypesetAsked = &asked
		}
		saved := *shown
		l.typesetChunkShown = &saved
		// Chunk-local layout completion is not the end of the whole history.
		result.TypesetDone = result.TypesetDone && index == chunks.Chunks()-1
		return result
	}
	return fail(fmt.Errorf("typeset chunk navigation did not settle"))
}
