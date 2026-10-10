# Direct Markdown layout: comparison findings

Historical comparison: the direct renderer has since been promoted into gloss.
The original measurements below are unchanged; the comparison module now keeps
a frozen PDF adapter because the production loader no longer generates PDFs.

The direct renderer is worth pursuing as an experimental viewer backend. It
produces a more readable table and a continuous reading surface, starts much
sooner in a fresh process, and avoids generating a PDF on width changes.
This is evidence for a next integration experiment, not feature parity or a
recommendation to replace the current backend immediately.

Measured 2026-10-09 on this macOS arm64 machine, Go 1.27.1; five fresh subprocesses
per backend/fixture, alternating backend order, medians. Updated with Chroma
highlighting enabled in the direct backend; the harness was built with CGO disabled. Same fonts, source,
body size, viewport and side margins. No other tests/builds ran during this final
measurement. Filesystem caches were warm; PDFium starts fresh in each process.

| Workload | Direct go-text | Current PDF path |
|---|---:|---:|
| Small fixture: first viewport, cold renderer | 9.47 ms | 1,108.94 ms |
| Small fixture: resize 800 → 600 | 5.99 ms | 14.83 ms |
| Long fixture: first viewport, cold renderer | 11.51 ms | 1,136.90 ms |
| Long fixture: resize 800 → 600 | 5.54 ms | 40.54 ms |
| Small fixture: peak process RSS across workload | 60.03 MiB | 331.67 MiB |
| Long fixture: peak process RSS across workload | 56.45 MiB | 340.20 MiB |

The cold-start gap includes PDFium initialization; it does **not** imply PDF
layout alone takes a second or that every later document would pay that cost.
The warm resize comparison is more relevant to the reported resizing experience.
PDF repaint remains faster: about 0.9–1.0 ms to copy a cached page raster versus
5.1–5.8 ms for direct text rasterization. Neither number includes Kitty encoding,
terminal transmission, terminal rendering, event-loop latency, or tmux.

See [raw samples](results/measurements.json) and the
[generated summary](results/RESULTS.md), including allocation volume and sampled
Go heap. Peak RSS includes first paint, finishing remaining layout, resize and
repaint, before saving image artifacts. GC timing makes the sampled Go heap
variable; the process RSS comparison is more useful for whole-renderer memory.

## First screen and later layout

- `typeset.md`: preparation 3.76 ms, first layout 0.32 ms, raster 5.42 ms. First paint reached 8 of 9 blocks; remaining layout took 0.03 ms.
- `long.md`: preparation 3.19 ms, first layout 0.21 ms, raster 8.11 ms. First paint reached 5 of 201 blocks; remaining layout took 4.15 ms.

Preparation is still whole-input parsing and image validation/decoding. The
long fixture is 100 sections with mixed inline styles; it is not an image-heavy
stress test. Progress is block-granular: one enormous paragraph/table can still
block the first viewport. The harness explicitly calls the continuation after
first paint; scheduling it in the background and cancelling stale work are not
implemented.

## Visual comparison

Open [the side-by-side gallery](results/index.html), or view the PNGs in gloss
using the command in [README](README.md).

- The direct table uses padded cells, a distinct header and full reading width.
  The current PDF table is cramped and uses only part of that width.
- The direct quote has a margin rule; its code block has consistent inset and
  spacing. Both now use the existing Chroma highlighter and GitHub light theme;
  token foregrounds, bold and italic survive wrapping and resizing.
- Direct flow keeps the picture adjacent to the preceding list. The PDF backend
  puts the picture on its next paper page, leaving a large blank area. The
  overview image for PDF is **page 1 only**; the picture has not been dropped.
- Accented Latin and mixed emphasis are visible in both. This does not establish
  broad Unicode, font fallback or bidi correctness. The direct H1 is bold while
  the PDF H1 is regular: these are deliberately different styles using the same
  font family and size.
- Width 600 inspection caught missing spaces from wrapping's mutation of cached
  glyph advances. Inputs now copy their glyphs before wrapping, and a regression
  test compares resized output with a fresh renderer pixel-for-pixel.

## The performance fix inside the spike

The first direct implementation drew every styled run into the viewport. The
renderer creates a scanner/filler sized to its destination for each run. That
version allocated about 200 MiB on the small fixture and took roughly 69 ms for
first paint and 49 ms on resize. Rendering each run to a small transparent buffer
before compositing reduced measured first-paint allocation to about 25 MiB with highlighting enabled.
This is still allocation-heavy enough to justify a bounded glyph/line raster
cache in a future iteration; the current spike deliberately has none.

## Recommendation and next experiment

Keep the current PDF backend while trying this behind a distinct experimental
viewer mode. Reuse the existing single-picture output and source toggle. Add
cancellable layout jobs and preserve the current reading position across reflow
using a source/block anchor. Add a bounded raster cache so scrolling can reuse
text pixels. Then measure end-to-end Kitty and tmux interaction rather than only
CPU-side rendering. Search/link positions can eventually come from the display
list, but the spike does not yet retain a complete source mapping.

Before exposing uploads: share the production asset preflight, add total
layout-time/display-list allocation budgets, and improve Markdown conformance,
table sizing, inline images and cancellation. See the README's explicit subset
and limitations. No production dependency, mode, graphics protocol, or default
behavior changed in this experiment.

## Validation

- Separate module: `GOWORK=off go test -race ./...` passed, including scrolling,
  cache immutability across resize, mixed Unicode/styles, hard lines, progressive
  blocks, missing/remote images and budget rejection before image decoding. Highlighting
  tests cover Go/Python/JSON, plain fallback, source preservation, raster colors,
  and theme-aware cache keys.
- Separate-module vet, `CGO_ENABLED=0` build, and `go mod tidy -diff` passed.
- PNGs inspected at 800 and 600 px, including the full direct fixture.
- Repository `task ci` passed format/tidy/verification, race tests, vet, Windows
  checks and staticcheck, then failed at the existing Go 1.27.1 vulnerability
  advisories (the reported standard-library fixes require 1.27.2).
- Root build, notices check and docs check passed separately after the CI stop.
- Actual terminal interaction tests were not part of this renderer comparison.
