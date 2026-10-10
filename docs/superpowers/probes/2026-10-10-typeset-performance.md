# Typeset viewer performance probe

Date: 2026-10-10. Branch `probe/typeset-performance`. The measurement tooling
named below was throwaway and was not merged; this report is the deliverable.

## Question

Scrolling and resizing the `--typeset` view of this repository's `README.md`
in Ghostty feels slower than it should. The README embeds the 944×664 demo GIF
(99 frames, mostly 80–90 ms apart, looping forever). Where does the time go?

## Method

1. Loader benchmarks (`internal/document/typeset_probe_*_test.go`) driving
   `Loader.Load` with a `TypesetView` at a 2000×1000-pixel viewport, the size a
   960-point measure produces at 150 DPI. Scenarios: first open, scroll steps at
   a fixed width, width changes deep in the document, with and without GIF
   animation. CPU and allocation profiles via `go test -cpuprofile/-memprofile`.
2. Micro-benchmarks of the per-frame pipeline downstream of layout on a real
   typeset frame: GIF compositing, the picture library's fit resample, PNG
   encoding and base64, raw RGBA and base64.
3. Session traces. `GLOSS_PROBE_TRACE=path` records a `runtime/trace` with
   regions around layout (`typeset.Load`), compositing (`animate.Next`) and the
   picture encode (`kitty.encode`), plus `input` log events for key presses and
   resizes. `experiments/traceprobe` summarizes a recording.

Machine: Apple M5 Max, Go 1.27.2, Ghostty.

## Pipeline

One scroll step or resize runs, in order: `Loader.Load` re-reads and hashes
the file, lays out blocks as far as the viewport needs, rasterizes the
viewport into a fresh RGBA; the picture library fits that image to the cell
grid (allocate, fill background, CatmullRom resample), PNG-encodes it at
BestSpeed, base64s it and writes a Kitty APC under a fresh image ID. While the
GIF is on screen, every GIF frame runs the same tail: composite a full
viewport image, fit, encode, transmit.

## Measurements

Per frame at 2000×1000 on `README.md`:

| Stage | Time | Allocation |
|---|---|---|
| Layout + raster, one scroll step | 10 ms | 32 MB |
| Layout + raster, width change | 12 ms | 31 MB |
| First open, GIF decode included | 75 ms | 111 MB |
| GIF composite onto the viewport (`CompositeSource.compose`) | 5 ms | 31 MB |
| Picture library fit resample at 1:1 (`containTo`) | 14 ms | 72 MB |
| PNG BestSpeed encode + base64 | 10 ms | 0.16 MB payload |
| Raw RGBA base64, for comparison | 2 ms | 10.7 MB payload |

Scroll-step CPU profile, loader only: raster 49 % (go-text glyph drawing 30 %,
GIF CatmullRom 6 %), GC returning memory to the OS (`madvise`) 21 %, file
re-read and SHA-256 11 %. Allocation per step: 43 % the viewport RGBA, 32 % a
fresh rasterizer per glyph run inside `DrawShapedRunAt`, 24 % the scaler's
temporary buffer.

Ghostty sessions, summarized by `experiments/traceprobe`:

| Session | Inputs | `typeset.Load` mean | p50 | max |
|---|---|---|---|---|
| Scrolling by wheel, 10.3 s, 216 steps | ~21 steps/s | 9.4 ms | 8.3 ms | 31.4 ms |
| Resizing, 6.8 s, 61 resize events, 14 reflows | debounce coalesces 4:1 | 21.6 ms | 20.0 ms | 29.6 ms |

CPU samples by pipeline stage in those sessions (gloss process only; the
terminal's decode is not visible here):

| Stage | Scrolling | Resizing |
|---|---|---|
| Picture library fit resample (`prepareSource`) | 27 % | 33 % |
| PNG encode | 17 % | 16 % |
| Glyph raster (go-text render) | 10 % | 3 % |
| Bubble Tea renderer (cell-width scanning) | 6 % | 4 % |
| Typeset raster, excluding glyphs | 4 % | 4 % |
| GIF decode, composite, APC build, file hash, GC | ~4 % | ~9 % |
| Runtime: goroutine wakeups, `madvise`, idle | ~30 % | ~28 % |

Top leaf functions in both sessions are the CatmullRom scaler's row and
column passes (24–31 % together), `madvise` (10–15 %) and PNG filtering.
The GIF composite shows up little because the GIF was rarely on screen.

The README is 4,563 pixels tall at this width and lays out as 30 blocks and
329 drawing operations; the GIF occupies rows 284–948.

## Findings, ranked

1. **The picture library resamples every frame even at 1:1.** Gloss uses the
   default contain fit, whose path always allocates a target image, fills the
   background and runs CatmullRom over 2 megapixels: 14 ms and 72 MB per frame
   for no visual change. The fill fit has a same-size shortcut, but only with a
   transparent background.
2. **Each GIF frame is a full new viewport image.** Compose copies the base,
   rescales the GIF 1:1, then the frame goes through fit, PNG and a new Kitty
   ID. About 29 ms CPU and over 100 MB per frame, twelve times a second while
   the GIF is visible: roughly a third of a core and 1.2 GB/s of allocation at
   idle, plus a 2-megapixel PNG decode per frame on Ghostty's side.
3. **Raster allocates everything fresh per step.** The viewport image, a go-text
   rasterizer per glyph run and a scaler buffer. Reusing buffers and caching
   rendered glyphs would roughly halve the 10 ms and remove most GC churn.
4. **The loader re-reads and hashes the file every step.** Ten percent of the
   scroll profile for a 6 KB file; grows with file size.
5. **PNG is the right transport for text.** It is 60× smaller than raw RGBA for
   this content. Shared memory is unavailable in releases (`CGO_ENABLED=0` on
   macOS) and is only an option on Linux.

Resize is not a layout problem: shaping is cached and relayout of the whole
README costs about 17 ms. What is felt on resize is the same per-frame tail as
scrolling, after a 60 ms debounce.

## Not measured

Ghostty's decode and upload time per frame, and how scroll frames queue behind
in-flight GIF frames. The hooks log inputs and mark the Kitty encode so the
summarizer can report input-to-frame latency for a recording made with the
final hooks.

## Recommendation

Fix findings 1 and 2 first, as separate bounded changes. Finding 1 is a fit or
background change in gloss plus a same-size shortcut in the picture library's
contain path. Finding 2 is a design question about Kitty placements (a second
image over the GIF rectangle, or Kitty animation frames) and deserves a short
design discussion. Finding 3 follows and is the only one that touches the
typeset layout code. Finding 4 is a small independent fix.

## Fix applied: finding 1

Two changes, one per repository:

- ntcharts, branch `fix/fit-same-size` (worktree `../ntcharts-fit`): all three
  fit modes draw a same-size source with a plain copy instead of CatmullRom,
  and an opaque source at exactly target size is returned as is. The fit of a
  2000×1000 frame drops from 14 ms and 72 MB to 3.6 ms and 8 MB for the copy,
  and to an opacity scan for the identity.
- gloss: under Kitty graphics the viewer names its picture's device size in
  `TypesetView.Pixels` and the typeset raster is exactly that size. Without
  this the raster is 2000 pixels wide at the 960-point cap, and a wider or
  higher-density terminal had it enlarged, blurrily, on every frame; a
  rounding pixel in height was enough to resample at 1:1.

Measured through the picture component on an opaque frame, fit plus PNG
encode: 10.7 ms and 2.5 MB per frame, from about 24 ms and over 80 MB. The
remaining per-frame cost is the PNG encode.

## Fix applied: the View frame

The 6 % the summarizer labelled "Bubble Tea renderer" was gloss's own View:
the body wrapped the picture's placeholder grid in a lipgloss frame of the
body's size, and the animator clipped it to the viewport, so every View
measured all cols×rows placeholder graphemes to pad and clip nothing. Bubble
Tea's own renderer was one sample. The animator now records each placement's
cell area, returns a presented grid as it is while the area still matches,
and reports that exactness; the body is then the grid itself.

| `BenchmarkPictureView`, 200×52 | Before | After |
|---|---|---|
| Time per View | 2.64 ms | 15.6 µs |
| Allocation | 1.8 MB, 86,145 allocs | 96 KB, 234 allocs |

## Fix applied: finding 2

Under Kitty graphics each GIF now plays in an overlay picture the size of its
cells, fed a window of the page (`CompositeSource.Window`), and the page
picture is not resent while it plays. Kitty native animation frames were
considered and deferred: Ghostty answered "unimplemented action" through
v1.3.1 and support is only reported for an unreleased 1.4.0.

| README GIF frame, composite + PNG | Whole viewport | GIF window |
|---|---|---|
| Time | 18.1 ms | 7.8 ms |
| Allocation | 12 MB | 6 MB |
| Pixels sent to the terminal | 2.0 Mpx | 0.63 Mpx |

A third Ghostty recording showed the composite still resampling each frame:
that terminal draws the page above the GIF's resolution, so the 1:1 copy did
not apply. The overlay is now prepared at the GIF's own resolution through the
picture's resolution factor and the terminal enlarges it. At 1.5× page scale:

| GIF frame, composite + PNG | At page scale | At GIF scale |
|---|---|---|
| Time | 32.3 ms | 7.8 ms |
| Allocation | 40 MB | 6 MB |
| Pixels sent to the terminal | 1.41 Mpx | 0.63 Mpx |

The picture truncates cell pixels times the factor to an integer, so with some
cell sizes the window's scale lands a few percent off the GIF's own size and
each frame was still resampled by CatmullRom. Frames within a factor of two of
their own size now use the bilinear scaler: 15.3 ms and 26 MB per frame down
to 10.3 ms and 6 MB in that case. In Ghostty, compositing went from 14.3 ms per
frame (trace 5) to 3.5 ms (trace 6) before this last change.

## Fix applied: finding 4, the reflow itself

Trace 7, a resize-heavy session, showed the page itself as the cost once the
frames were cheap: 17 ms per `typeset.Load`, a page encode per reflow, and
the stale grid measured again during each burst. Three changes:

- A page with animated regions in view is rastered once, without them; the
  composite draws their frames in. Before, it was rastered twice and the
  first raster resampled each GIF's first frame only to be thrown away. A
  composite with no regions in view no longer copies the page.
- Overlays are built only once the view has stood for 120 ms, so a scroll or
  resize burst costs the page alone; each GIF stands at its current frame,
  which the page carries.
- A stale grid shown during a burst is cut to the viewport by cell count and
  reported exact, so it skips the frame too.

| Reflow with the GIF in view (`BenchmarkProbeScroll`) | Before | After |
|---|---|---|
| At 1× | 12.6 ms, 51 MB | 6.6 ms, 25 MB |
| At 1.5× | 35.3 ms, 109 MB | 16.7 ms, 56 MB |

| `BenchmarkPictureViewStale`, 200×52 grid at 180×48 | Before | After |
|---|---|---|
| Time per View | ~2.6 ms | 68 µs |

## Where it ended: trace 8

A 25-second Ghostty session with 103 resize events and the GIF playing used
about 1.5 s of CPU in all, 6 % of one core. The resize burst coalesced into 12
reflows.

| Per event | Trace 5 | Trace 8 |
|---|---|---|
| `typeset.Load` mean | 11.6 ms | 5.5 ms |
| `animate.Next` mean | 14.3 ms | 4.2 ms |
| `kitty.encode` mean | 9.1 ms | 6.0 ms |
| Input to frame, median | 14.4 ms | 10.1 ms |

What remains is the PNG encode of each GIF frame's window (23 % of a small
total), goroutine wakeups in the command model (28 % of leaf samples, about
1.7 % of wall), and the terminal write. A paletted PNG for windows that are
all GIF would cut the encode and the payload several-fold, at the cost of
handling the page margins inside the window; not done.

## Finding 5: the page encode, from a text page

Trace 9, a plain text page (`README.txt`) with 990 wheel and 281 resize
events in 19 s: 380 reflows at 5 ms, 256 encodes at 10 ms, and the PNG
encode at 27.5 % of samples, half of it in the filter trials Go's encoder
runs on every row even at BestSpeed. `madvise` at 14 % is the collector
returning each 12 MB raster to the OS; the terminal write is 11 %.

Kitty takes raw pixels compressed with zlib (`o=z`). ntcharts gained
`KittyFormatZlib` (RGB when opaque, RGBA otherwise) and gloss sends every
Kitty frame that way:

| Text page encode + base64 | PNG BestSpeed | RGBA + zlib | RGB + zlib |
|---|---|---|---|
| 2000×1000 | 16.4 ms, 0.27 MB | 5.7 ms, 0.26 MB | 4.5 ms, 0.23 MB |
| 3000×1500 | 32.9 ms, 0.46 MB | 11.6 ms, 0.45 MB | 9.1 ms, 0.39 MB |

Through the picture component the frame path fell from 10.7 ms to 5.8 ms.
In Ghostty on the text page, 454 wheel events in ten seconds:

| Per event | PNG | zlib |
|---|---|---|
| Kitty encode mean | 15.2 ms | 5.3 ms |
| Input to frame, median | 9.8 ms | 13.4 ms |
| Input to frame, 90th percentile | 66.4 ms | 15.9 ms |

What remains on a text page: glyph rasterization at 14.5 % (every step
draws every glyph from its outline; a mask cache is the original finding 3),
the collector returning memory at 14 % (`madvise`; try `GOGC=400` in a
session before changing code), and the terminal write at 12 %.

## Fix applied: finding 3, glyph masks

Every scroll step drew every glyph from its outline through go-text's
rasterizer, with a rasterizer allocated per run: 14.5 % of the text-page
session. Glyphs are now drawn from cached masks, rendered once per glyph,
size, raster scale and quarter-pixel position, trimmed to their ink and
blended in the run's colour in one draw. The cache has a 32 MiB budget.

| Reflow, `BenchmarkProbeScroll` | Before | After |
|---|---|---|
| Still page at 1× | 8.8 ms, 33 MB | 3.1 ms, 22 MB |
| GIF in view at 1× | 6.6 ms, 25 MB | 1.4 ms, 14 MB |
| GIF in view at 1.5× | 16.7 ms, 56 MB | 6.0 ms, 31 MB |

## Where it ended: trace 11, text page

A text page, 316 wheel and 190 resize events in 10.8 s, 142 CPU samples in
all (about 13 % of one core while being driven hard):

| Per event | Trace 9 | Trace 11 |
|---|---|---|
| `typeset.Load` mean | 5.0 ms | 1.3 ms |
| `kitty.encode` mean | 10.1 ms | 6.1 ms |
| Input to frame, median | 12.8 ms | 8.3 ms |

What remains: the zlib encode and base64 (25 %, of which Adler-32 alone is
8 %), the terminal write (20 %, the pty blocking while the terminal
ingests), Bubble Tea's own frame diff over the placeholder grid (15 %), and
scheduling. The 90th-percentile latency of 65 ms is the 60 ms resize
debounce, set when a reflow cost ten times what it does now.

## Where it ended: trace 12, Markdown page under resize

The resize debounce dropped from 60 ms to 30 ms once a reflow cost a tenth
of what it did. 435 resize events in 21.7 s on `README.md`:

| Per event | Trace 7 | Trace 12 |
|---|---|---|
| Input to frame, median | 17.8 ms | 5.4 ms |
| Input to frame, 90th percentile | 31.8 ms | 35.0 ms |
| `typeset.Load` mean | 17.1 ms | 3.4 ms |
| `kitty.encode` mean | 7.0 ms | 4.9 ms |

The GIF advanced twice: a continuous drag never settles, by design. The top
of the profile is now the terminal write (23 %) and scheduling.

## Reproducing

```
go test ./internal/document -run xxx -bench Probe -benchmem
GLOSS_PROBE_TRACE=/tmp/gloss.trace gloss --typeset README.md
(cd experiments/traceprobe && GOWORK=off go run . /tmp/gloss.trace)
GLOSS_PROBE_KITTY=rgba gloss --typeset README.md   # or shm
```
