# Markdown layout comparison spike

An isolated experiment comparing gloss's goldmark-pdf → gopdf → PDFium path
with Goldmark → go-text shaping/wrapping → direct bitmap drawing. The direct renderer has since been promoted to the shipped viewer. This module
retains the original prototype and a frozen `pdfbaseline.go` adapter to keep
the PDF comparison reproducible; benchmark commands do not change viewer state. Its dependencies are pinned in
this separate module; the parent module is replaced with the local checkout.

Run from this directory:

```sh
GOWORK=off go test -race ./...
GOWORK=off go build -o /tmp/gloss-typeset-compare .
/tmp/gloss-typeset-compare -runs 5
```

The harness creates a fresh temporary results directory, prints its location,
and writes raw JSON measurements, a Markdown summary, and PNGs. Use
`-out /path/to/new-directory` to select a destination; it refuses existing
folders/files. `-fixture` accepts another local Markdown fixture within the
prototype's supported subset. The comparison always includes a generated,
original 100-section document as well. Nothing is fetched while rendering.

See [the recorded comparison](REPORT.md), [raw measurements](results/measurements.json),
and [the image comparison](results/index.html). To inspect the saved images
in gloss from the repository root:

```sh
./gloss --menu experiments/typeset/results/direct-800.png experiments/typeset/results/pdf-800.png experiments/typeset/results/direct-600.png experiments/typeset/results/pdf-600.png
```

## What is implemented

- The same eight repo-pinned Go TTFs as the PDF backend; body text at 25 px
  (12 pt at 150 DPI), heading 1 at 50 px, 50 px side margins.
- Headings, mixed bold/italic/monospace/strikethrough runs, paragraphs, hard
  breaks, quotes, lists/task markers, Chroma-highlighted code blocks, simple tables, local PNG/JPEG
  images and unavailable-image placeholders. Raw HTML does not get layout.
- Fenced code uses the existing pinned Chroma library and the PDF backend's
  GitHub light theme. Token colors, bold and italic become monospace runs;
  unknown/absent language tags and lexer failures fall back to plain code.
  Tokenization preserves source exactly and happens once during preparation.
  Colors participate in cache keys and survive wrapping and hard line breaks.
- Goldmark GFM parsing, go-text segmentation/shaping/line wrapping, and
  go-text/render rasterization. Explicit newlines split paragraphs before
  wrapping. Wrap inputs are copied because trimming can mutate glyph advances.
- Continuous vertical flow with a reusable display list. `ensure(bottom)`
  lays out blocks until the requested bottom; `viewport(top, height)` draws
  just that region. It does not allocate a whole-document canvas for viewing.
- Fonts and decoded images reused across widths, bounded shaping cache
  (32 MiB estimated entries; simple whole-cache eviction). No raster tile cache.
  Small per-run drawing buffers avoid viewport-sized rasterizer allocations.
- Preflight before image decode: 2 MiB UTF-8 source, 32 references, 16 Mi decoded
  pixels, 128 MiB combined encoded image input; nesting capped at 128 after
  parsing. Remote references are placeholders. No font downloading.

Preparation still parses the whole input and preflights/decodes its images.
A single paragraph or table is laid out as a unit, so an enormous first block
can delay first paint. Remaining layout is measured synchronously after first
paint in the harness: this demonstrates resumable layout, **not** background
scheduling or cancellation in the viewer.

## Comparison method

Each backend/fixture sample runs in a new subprocess of the same binary, with
backend order alternating. The first sample also saves PNGs after measurements;
encoding is excluded. Filesystem caches are not flushed. Runs are sequential;
this is a local comparison, not a universal performance guarantee.

First viewport is 800×900; resize is 600×900. The PDF path now uses the frozen adapter and the same PDFium renderer, 150 DPI
and widths 384/288 points, with normal page height. The recorded results predate
that extraction and used `document.Loader`; the shipped loader now draws directly.
It rasterizes the entire first page and crops to the same viewport. Cold PDF
numbers include PDFium initialization; resize uses the warm loader. Direct
resize reuses prepared input, fonts and shaped text after completing layout.
The direct layout covers all blocks without paper pagination. Different document styling and pagination make these comparable reading workloads,
not identical pixel workloads or equal feature coverage.

Allocation metrics use `runtime.MemStats`; allocation volume is not peak memory.
Live Go heap is sampled without forced GC. Process peak RSS is `getrusage` on
macOS/Linux (zero elsewhere), sampled after first paint, full remaining layout,
resize and repaint, before image artifact generation. RSS includes memory beyond
Go's heap. The harness executable links both paths; binary-size conclusions
cannot be drawn from it.

## Deliberate limitations

This is not a production Markdown renderer. There is no link interaction, search mapping, selection, bidi/font-fallback guarantee,
HTML/CSS, SVG input, inline image layout, alignment-aware tables, robust table
sizing, or real checkbox drawing. Inline images become labels; images alone in
a paragraph become blocks. Table widths use capped natural cell widths rather
than full min/max-content sizing. Long unbroken text, deeply nested lists,
entities/escapes, mixed-direction text and extreme tables need more conformance
work. It has no asynchronous cancellation, a total layout-time limit, or a
production budget on display-list allocations. It is for bounded local fixtures,
not exposed to uploaded input. There is no new terminal protocol, CLI feature,
or browser integration.

The tests cover fixture elements, progressive block layout, exact scrolled vs
full raster agreement, resize cache immutability, Unicode Latin shaping,
mixed styles, hard/empty lines, source/image budgets, and remote placeholders.
Highlighting tests cover Go/Python/JSON, unknown-language fallback, exact source
preservation, raster colors, resize consistency, and theme cache invalidation.
They do not establish terminal transmission latency or subjective readability.

## Dependencies and licensing

- `alecthomas/chroma/v2 v2.27.0`: reused from gloss for fenced-code tokens and
  the GitHub theme. No new production dependency.
- `go-text/typesetting v0.3.5`: shaping, segmentation and line wrapping; pure Go,
  used by Go UI toolkits. Upstream is actively maintained but explicitly keeps
  an evolving v0 API. [Source and maintenance policy](https://github.com/go-text/typesetting).
- `go-text/render v0.2.1`: a small raster drawing layer. Inspection found a fresh
  scanner/filler per `DrawShapedRunAt` sized to the target image, so this spike
  uses run-sized buffers. [Source](https://github.com/go-text/render).
- Both permit BSD-3-Clause or Unlicense; this experiment uses BSD-3-Clause.
  The renderer brings `srwiley/rasterx` (already in gloss's graph) and
  `srwiley/oksvg` (a separate upstream module from gloss's NimbleMarkets fork).
  These are portable Go implementations; no new CGO requirement.
- [Experiment notices](THIRD_PARTY_NOTICES.md) retain these licenses. Existing
  dependencies are covered by [gloss's notices](../../THIRD_PARTY_NOTICES.md);
  the unchanged font bytes retain their [font license](../../../ntfonts/LICENSE.gofont).

Before integrating, benchmark larger representative documents, preserve the
source/search model, add cancellation and strict total allocation/time budgets,
and validate width changes/scrolling through the real picture event loop.
