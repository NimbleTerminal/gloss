# Developing gloss

gloss is written in Go on [Bubble Tea](https://github.com/charmbracelet/bubbletea)
and [NTCharts](https://github.com/NimbleMarkets/ntcharts), with
[NTCharts SVG](https://github.com/NimbleMarkets/ntcharts-svg),
[NTCharts PDF](https://github.com/NimbleMarkets/ntcharts-pdf), and
[nt3d](https://github.com/NimbleMarkets/nt3d). The user
documentation is on the [documentation site](https://nimbleterminal.github.io/gloss/docs/);
this file is for working on gloss itself. [AGENTS.md](AGENTS.md) is the same
ground for coding agents.

## Go module path

The root module is `nimbleterminal.dev/gloss`. Self-imports and the separate
`examples/demo` and `experiments/typeset` modules use that prefix; their local
replacements still point at the root checkout. GitHub remains the source host.
Consumers moving from the GitHub module path must update their imports and use
a version published after the migration. Existing tags are unchanged.

## Build and run

Requires Go **1.26.9+** and [Task](https://taskfile.dev/) for the development
commands. Without Task, build with `go build -o gloss ./cmd/gloss`. Go's
automatic toolchain selection can download that version. Dependencies are
pinned in `go.mod`; sibling checkouts aren't needed. One dependency is
replaced: Bubble Tea's renderer, `github.com/charmbracelet/ultraviolet`, is
taken from the `perf/consolidated` branch of `github.com/neomantra/ultraviolet`
by a `replace` directive pinned to a commit, until that branch's renderer
changes are upstream (a Kitty placeholder grid frame costs about 40% less;
`BenchmarkTerminalFrame` in `internal/app` measures it). A module with a
`replace` cannot be installed with `go install …@latest`, so install from a
clone. Clone with `--recurse-submodules` (or run `git submodule update
--init`) to get the documentation theme.

```sh
task build                       # ./gloss
task run -- photo.png            # go run, with arguments
task install                     # installs gloss into your Go bin directory
./gloss examples/shapes.svg examples/tetrahedron.stl
go build -ldflags '-X main.version=0.1.0' -o gloss ./cmd/gloss
```

`task --list` lists the development commands.

## Test

```sh
task test
task fuzz                       # fuzz each loader, 60s apiece (FUZZTIME=5m, FUZZ=FuzzParseSTL)
task ci                         # formatting, modules, race tests, vet (also for Windows), build, notices, docs
```

Tests cover CLI validation, malformed files, STL geometry, PDF rendering and
navigation, SVG rasterization, viewport cropping, terminal-safe labels, and
stale asynchronous results. The example SVG and STL are small original
fixtures; `task gen-assets` regenerates the PNG, HEIC, JPEG XL, PDF, the block-letter
STL, and, with `python3`, the Grist document, which is written by hand to
Grist's layout rather than saved from Grist.

GitHub Actions runs `task ci` on Linux and macOS for pushes and pull requests. Windows is only
cross-compiled and vetted (`task cross-windows`), not tested.

## Recording the showcase

`scripts/gloss-demo.tape` is a VHS-language showcase recorded by
[ntrecord](https://github.com/NimbleTerminal/ntrecord), including actual Kitty
graphics, GIF playback, PDF paging, mesh rotation, Markdown search, tables, and
the URL QR overlay. It uses only the original fixtures in this repository and
does not fetch files or start a handoff server.

Run from the repository root:

```sh
GOWORK=off task build
env -u NO_COLOR ntrecord validate scripts/gloss-demo.tape
env -u NO_COLOR ntrecord scripts/gloss-demo.tape
# With the sibling development checkout, use ../ntrecord/bin/ntrecord instead.
```

The tape writes `dist/gloss-demo.gif`, a text transcript, and scene screenshots.
These generated assets are ignored and are not embedded in the binary. Recording
again replaces them, so copy aside any take you want to keep. Use realtime mode
(the default): deterministic mode does not advance application animation timers.
The tape waits for loaded views and hides startup/loading time. It uses Menlo,
included with macOS; elsewhere set `FontFamily` to an installed monospace font
with box-drawing glyphs, or to ntrecord's bundled `Go Mono`. Keep `NO_COLOR` unset
so Kitty placeholder colors survive. The mesh uses software rendering and does
not require a GPU.

The README uses `docs/assets/gloss-demo.gif`, a losslessly optimized copy of the
recording. After reviewing a new take, install `gifsicle` and update that asset:

```sh
gifsicle -O3 dist/gloss-demo.gif -o docs/assets/gloss-demo.gif
```

This preserves resolution, colors, and timing while storing changed regions
instead of whole frames. Keep the raw recording in `dist/`; only the optimized
README asset is tracked, and neither is embedded in gloss. Avoid lossy
optimization of terminal text and QR codes. For video, add an MP4 `Output` to
the tape and record directly rather than converting the already quantized GIF.

## Layout

The layout follows NTCharts' conventions, with one module for the CLI and a
separate browser-demo module:

- `cmd/gloss`: CLI flags, stdin handling, export orchestration, and the
  temporary server behind `--serve`.
- `web`: the demo site, the page `--serve` shows, and the experimental plain-JS
  `--pick-web` request page (`pick.html`, `pick.mjs`, `pick-api.mjs`).
- `internal/app`: terminal pager, selection menu, and Markdown layout.
- `internal/browse`: the file chooser behind the `o` browser, a Bubble Tea
  component of its own (see below); `internal/browse/browsetest` is its test
  harness.
- `internal/document`: bounded loaders, renderers, and vision image sizing.
  `parser_worker*.go` isolates SVG rasterization and PDF metadata/text parsing
  in the same executable's private worker entry point, or a disposable browser
  Web Worker. No external converter is installed. Parent-side deadlines stop
  and reap/terminate workers; two slots bound concurrency. Under the race
  detector the worker, which is the test binary run again, is several times
  slower, so `race_test.go` lengthens the deadline for the package's tests;
  tests of the deadline itself set their own. The worker protocol
  accepts bounded bytes, never paths to read or shell commands. PDFium rendering
  retains its existing backend after stream preflight. `pdf_budget.go` follows
  declared lengths, resolves unambiguous integer length objects, and budgets
  every supported filter stage before the PDF reader sees the input. Unsupported
  filters and indirect decoding parameters are explicit errors. Goldmark parses
  synchronously with block/inline/transform callbacks checking time and work;
  there are no abandoned timeout goroutines.
- `internal/app/qr.go`: table URL overlay using the external ntqrcode component.
- `examples`: small runnable fixtures.
- `scripts`: site building and fixture generation.
- `docs/hugo`: the documentation site, published at `/docs/` beside the demo.
- `skills/gloss`: the agent skill the binary carries (`gloss skill`).

## Document search

`internal/app/search.go` owns the viewer's `/` query editor, bounded search
commands, navigation, and ANSI-preserving highlighting. It uses Bubble Tea's
existing key/paste routing and event loop, without a clipboard dependency, so
the same component builds in the WASM demo. The folder browser's independent
`/` glob search is unchanged.

`internal/app/markdown_text.go` caches unwrapped Glamour text separately from
screen rows. Each row carries UTF-8 byte spans into logical text; wrapped spaces
and generated continuation indentation are accounted for during layout. Search
results retain logical offsets, and projection highlights every displayed
fragment while keeping the selected occurrence stable across resize. Markdown
table cells are rendered independently by Glamour and composed within bounded
column widths; this avoids unbounded natural-width table padding and preserves
cell boundaries, alignment, hyperlinks, and inline image placement. Source and
rendered caches are separate and immutable for the lifetime of the document.

Workers search these logical text snapshots or sheet rows and visible columns.
PDF searches own a separate text-only `document.Loader` and
close it on completion; raster loading remains independent. Queries are literal
RE2 expressions with Unicode case folding. Work is serialized per model, even
across canceled query replacements, to avoid accumulating concurrent PDF
parsers. Cancellation and owner/revision checks discard late replies. Layout
versions and column snapshots trigger reindexing after resize/source/visibility
changes. Reload, navigation away, and shutdown cancel outstanding work.

Bounds: 256 query runes, 1,000 text occurrences/cells/pages, 16 MiB searched text,
and a 30-second context checked between units. Existing PDF extraction deadlines
still bound an in-flight page; cancellation does not interrupt that parser.
Text matches cross soft wraps but not logical line/paragraph/cell boundaries.
PDFs navigate by page and
show excerpts, without raster highlights or OCR. The guide's Controls page
documents these scopes and the temporary `n`/`N` bindings.

## Typeset Markdown

`--typeset` and `t` use the direct renderer in
`internal/document/typeset_layout.go`, with go-text/typesetting v0.3.5 and
go-text/render v0.2.1 (BSD-3-Clause), Chroma, and the embedded Latin font subsets.
`typeset.go` preflights all local assets before decode; `typeset_render.go`
connects the cached layout to `Loader`. `typeset_text.go` sends literal text
directly to line blocks, with Chroma spans for source/JSON; only Markdown is
parsed as Markdown. Each format retains its original kind. `--markdown typeset`
is a compatibility alias; `--typeset` and `t` select the shared view mode.
The production graph no longer uses goldmark-pdf/gopdf; PDF inputs still use
PDFium. The [comparison experiment](experiments/typeset/README.md) retains a
frozen PDF adapter in its separate module and its recorded comparison images.

Standalone local images, including image-only links, are composited into the
typeset viewport; embedded GIF blocks animate when visible. Images mixed with prose
still appear as labels.

The loader serializes each font/shaping engine. It caches prepared blocks,
decoded images, shaped runs and the current width's display list. Copy glyphs
before wrapping: go-text's whitespace trimming mutates their advances. Chroma
runs during preparation, recording GitHub light and dark foreground colors,
plain fallback, and exact source preservation. Both colors participate in the
shaping cache key; font styles come from the light theme for stable metrics.
Rasterization draws each glyph from a cached mask, rendered once by go-text
at the glyph's size, raster scale and quarter-pixel position and trimmed to
its ink, blended in the run's colour; drawing every glyph from its outline on
every scroll step was the raster's largest cost. The mask cache has a 32 MiB
budget. A run holding a bitmap or SVG glyph is drawn by go-text on a tile.
There is no terminal graphics composition and no new graphics protocol.

`internal/theme` owns named palettes shared by Glamour and the direct renderer.
`--theme auto|light|dark|NAME|FILE.json` resolves to light for headless exports. The application
owns interactive detection: `tea.RequestBackgroundColor` from Init/focus and
`tea.BackgroundColorMsg` in Update, without blocking reads or extra probes.
Auto starts dark; explicit themes ignore terminal replies. Previews inherit
the parent theme and do not query. Theme changes invalidate Glamour's styled
text cache and reindex existing search; typeset requests carry the resolved
theme and reuse the font engine, shaped runs and display list. The rasterizer
selects semantic fill colors and syntax-token colors at paint time.
A theme mismatch schedules a cancellable viewport repaint and preserves scroll.
The normal generation/revision checks reject stale completions. Embedded
pictures, PDF pages and QR colors are not transformed.

`Request.TypesetView` supplies a continuous viewport in 150-DPI layout pixels;
nil requests a headless 1125-pixel slice. `--dpi` scales drawing, not wrapping.
Under Kitty graphics the viewer also names its picture's device size in
`TypesetView.Pixels`, and the raster is exactly that size, so the picture
component shows it as it is instead of resampling it; glyph rendering keeps
the layout-scale raster.
The viewer derives width from physical cell geometry, debounces width changes
for 30 ms, cancels obsolete work and applies only matching generation/revision
results. It keeps one job in flight and the old bitmap visible. A cancelled
request leaves the layout as it was before the block it was in (`layout.block`
restores it), and the next request takes it up from there; only a layout that
failed of itself is dropped. New widths
anchor the current block; scroll and height changes reuse layout. Preparation
still parses/validates all input, and layout is incremental by block. There is
no speculative background completion; scrolling advances the display list,
and End or headless export completes it within the limits documented in the
guide. Initial input waits for terminal geometry before layout.

`internal/app/typeset.go` owns scroll/zoom requests and cancellation; normal
picture image IDs, replacement and cleanup remain in `showPicture`. `s` opens
source; `/` also opens source and uses existing Markdown search/highlights.
Interactive export snapshots the displayed viewport; headless exports lay out
all content and return numbered slices, whose boundaries may cut text/images.
`t` resets search/position and selects the mode for later Markdown files too.

Tests cover no-PDF loading, incremental layout, cancellation, bounds, cache
invalidation, raster scale, source/search toggles, resize/scroll consistency,
Chroma colors, exports, and concurrent independent loaders. Actual Kitty/tmux
resize, scrolling and glyph fallback remain manual checks after renderer work.

## Animated GIFs and WebPs

`nimbleterminal.dev/ntpicture/animation` scans GIF blocks before `image/gif.DecodeAll` to
bound the logical canvas (32 MP), frame count (1,000), and aggregate paletted
frame pixels (64 Mi). The standard library decodes LZW and palettes. Static
loads decode only the first frame and composite it onto the logical canvas;
previews and headless exports do not enable animation.

The same library does the same for animated WebP. It walks the RIFF
container itself (VP8X flags, the ANIM background and loop count, each ANMF's
placement, duration, blend and dispose bits), checking declared chunk lengths
against the file and the frame rectangles against the canvas. Pixels are
decoded by `golang.org/x/image/webp`: each frame's own ALPH/VP8/VP8L chunks are
wrapped in a synthetic single-image file, with an extended header when alpha
is present. WebP frame X/Y offsets are stored halved. Because WebP frames are
usually full-canvas RGBA video, only the first is decoded on opening; the rest
are `ClipFrame.Load` closures over the file's bytes that the player runs as it
paints (`ClipFrame.paint`), so memory is the canvases plus one frame and the
128 MiB file limit bounds the rest. A frame that fails to decode, or decodes
to another size, is skipped; its delay and disposal still apply. Preflight
reports the canvas plus the complete retained compressed container. A 1080p lossy frame decodes
in about 25 ms. `animation.DecodeContext`, `PreflightWithLimits` and `Animated` dispatch on
the file signature. The document loader and both Markdown renderers pass their
request context through preflight and decoding; previews still decode only the
first frame but validate the complete container before allocation.

`animation.Player` holds immutable decoded frames and a playback position. `Next`
returns a fresh composed canvas, honoring transparency and the
[GIF89a disposal rules](https://www.w3.org/Graphics/GIF/spec-gif89a.txt).
It uses [Go's loop-count semantics](https://pkg.go.dev/image/gif#GIF): -1 plays
once, 0 repeats forever, and positive values count additional repetitions.
A disposal-previous snapshot is kept only while needed. Render and PNG export
commands can retain previous images without concurrent mutation. Sub-20-ms
delays use 100 ms to avoid busy playback; other frame delays are preserved.
The viewer applies its 0.25×–4× speed multiplier after that normalization,
keeping the minimum playback delay at 5 ms. A speed change cancels the current
timer/composition epoch and starts a full new delay for the current frame;
pause, finished playback, and Kitty transmission backpressure still apply.
Speed belongs to the loaded animation: resize/zoom and Space restart retain it;
reload or loading another file starts at 1×. Headless output is unaffected.

`animation.Player.Seek` accepts a one-based frame, clamps to the endpoints, resets
the repeat count, and checks cancellation between frames. Forward seeks copy
the current position; backward seeks replay from the first frame. Each seek
reuses at most two private canvases for disposal, without caching full frames.
`internal/app/scrubber.go` coalesces drag/key requests into one active seek and
the latest target. Completion messages identify both owner and job; cancelled
or removed work cannot replace the current picture. Seeking pauses playback,
preserves speed/zoom, and uses the existing Kitty replacement/cleanup path.
The frame bar uses the hint row with screen-relative hit testing, so prompts
and image placement retain their geometry. Arrow keys remain image panning.

`internal/animate.Animator` owns cancellable timers and off-loop composition;
`internal/app/animation.go` adapts the standalone controls to it.
Messages carry a playback owner and epoch, so pause, reload, navigation, and
quit invalidate late work. Hidden documents cancel timers. Kitty waits for
transmission before starting the next delay; it never builds a frame backlog.
The last transmitted Kitty picture stays visible while its replacement encodes.
Only after transmission does the view swap grids and delete the old image, so
picture's transitional glyph fallback never flashes between animation frames.
Still images, PDF page changes, typeset pages without composite playback,
previews, and thumbnail grids use `ntpicture.Picture` through
`internal/app/still.go`. Picture owns retained fronts, one active preparation,
latest-request coalescing, output ordering, retirement, and keep-screen cleanup.
Both Kitty and glyph preparation run through returned commands. The host retains
the last presented full-resolution source for export and forwards messages even
when an overlay hides the page. The existing terminal model supplies capability
and cell geometry; the existing block allocator is shared with animation, mesh,
and QR consumers to avoid overlapping IDs during migration.

The migration currently uses the sibling `../ntpicture` checkout. Its temporary
module replacement must be replaced with a published pin before releasing
this integration. Next steps are standalone playback and Markdown
regions, followed by a shared terminal session and single-ID allocator.
Animation and composite playback still use `internal/animate.Animator`, but
its players and decoders now come from `ntpicture/animation`. The remaining
compositor migration needs lightweight position snapshots: Gloss carries a
destination's position through offscreen layouts, while ntpicture's Composite
drops absent keys. Retaining full old page composites would retain large
rasters unnecessarily. Playback controller migration must also preserve crop
transforms, seek/export state, and late cleanup without public mutable state.
Window and font-size changes cancel pending composition and render into a fresh
placement at the new geometry. Until it is ready, the old placeholder grid is
cut to the viewport by cell count so it cannot wrap or displace the status bar.
A presented grid, exact or cut, is the body as it is, with no frame around it:
measuring its cells to pad nothing was the largest cost of each View. Obsolete
resize completions cannot present over the latest layout; pause is preserved.
At most a visible front picture and a pending replacement are retained; leaving
or explicitly choosing glyphs cleans up both. Keep-screen quit retains the
visible frame, and PNG export snapshots that frame even during encoding.
Kitty frames are sent as zlib-compressed raw pixels (`KittyFormatZlib`, RGB
when opaque): PNG's encoder tries every filter on every row and took three
times as long for the same bytes.
Fresh image IDs avoid ghostty-web's texture cache, consuming the slots inside
blocks already reserved by `nextKittyID`. Retired placements receive a second
ID-specific cleanup if an accepted transmission completes late. Glyphs use
the same picture pipeline as static images. `e` captures the immutable current
frame before its asynchronous export; it never reloads the first frame.

Tests cover composition, offsets, background/previous disposal, delays, finite
and infinite loops, immutable snapshots, pre-decode bounds, cancellation,
stale work, export, and Kitty pacing/cleanup. `examples/motion.gif` is an
original generated fixture; `scripts/gen-assets/gif.go` regenerates it, and
`gen-assets` converts it to `examples/motion.webp` with `gif2webp` (libwebp)
when that tool is installed. The WebP unit tests assemble containers from tiny
`cwebp` bitstreams embedded as hex, since Go has no WebP encoder.
`./gloss examples/readme.md` compares that GIF as a Markdown linked image and
an HTML `img` inside a centered `p`/`a` wrapper. Check visible animation, size
bounds, aspect ratio, alignment, and linked-image clicks; press `t` to repeat
in typeset mode, `s` to inspect source, and scroll away/back to resume playback.
The same file, GIF and WebP are embedded in the native/browser sample gallery.
Menu previews and headless exports intentionally show the first frame.
GIF lifecycle tests pause playback and drain finite render commands to completion;
they must not use the generic test pump's timer heuristic to discard slow renders.

The package has no document or application dependencies. `Clip` describes
immutable paletted/RGBA frames, frame rectangles, blend/disposal, delays and loop
counts; `NewPlayer` validates 64 MiB of decoded storage (RGBA costs four bytes
per pixel; frames with a `Load` hold none). `DecodeGIF` and `DecodeWebP` are the bounded producers. `Source.Clone` snapshots mutable
playback state before asynchronous composition. `PlayerSource` drives one clip;
`CompositeSource` drives independently timed regions over a page raster that
omits animated images, so transparent frames reveal the original page background.

Each inline Markdown picture owns an Animator; the host supplies capability,
cell geometry, visibility and `nextKittyID`. Hiding cancels the epoch and retires
placements without discarding the playback position. Typeset results carry full
region rectangles and a composite snapshot; reflow preserves positions by
asset destination, and only visible regions advance. Reload starts fresh.
Under Kitty graphics the page picture stays still and each group of cells the
regions cover gets an overlay: its own picture and Animator, fed
`CompositeSource.Window`, the page's base and regions cropped to those cells.
A frame is then the GIF's cells rather than the whole viewport, and the page
is not sent again while a GIF plays. A page drawn above a GIF's resolution gets
its overlay at the GIF's own (the picture's `KittyResolutionFactor`), the base
under it resampled once, so frames are copied in and the terminal enlarges
them. The view splices a presented overlay's
placeholders into the page grid (`spliceCells`); one still in flight leaves
the page's own pixels. A reflow rebuilds the overlays and carries their
positions through the request (`WithPositions`); leaving, reload and a switch
to glyphs stop them, and glyph rendering animates the composite page as a
whole. Overlays are built only once a view has stood for 120 ms
(`typesetSettleDelay`): a scroll or resize burst costs the page alone, each
GIF standing at its current frame, which the page carries. A page with
regions in view is rastered once, without them; the composite draws their
frames in. `e` composes the overlays' current frames onto the page.
Markdown loaders share an eight-destination/64 MiB animation budget and retain
still first frames when exceeded. The info field reports eligible/total GIFs.
Normal Markdown `e` snapshots the first visible GIF's full frame; typeset `e`
snapshots the displayed viewport. There are no inline playback controls.

## QR component

The reusable encoder and terminal component live in
[ntqrcode](https://github.com/NimbleTerminal/ntqrcode), imported as
`nimbleterminal.dev/ntqrcode/qrcode`. Gloss pins a tagged
release in `go.mod`; there is no local replacement or copied implementation.
The library owns module/image generation, quiet zones, Kitty and half-block
rendering, bounds, fit errors, and image cleanup. Its decoder and lifecycle
unit tests live with the component. See its README and DEVELOP for the API,
encoder assessment, and NTCharts exact-size rendering contract.

The pinned release (`v0.2.0`) uses `piglig/go-qr/v2 v2.6.0`. Gloss uses the
default options: medium error correction or higher, boosted when a stronger
level fits without increasing the symbol version. Numeric, alphanumeric, byte,
and Kanji segments are optimized to fit the content. Non-ASCII payloads include
UTF-8 ECI; ambiguous Kanji mappings stay in UTF-8 byte segments to preserve the
exact text. Both the native module and browser demo pin this release.

`internal/app/qr.go` owns the overlay, selected table URL, and export through
gloss's existing non-overwriting save hook. The app supplies `nextKittyID`,
its detected graphics mode and cell geometry, and space inside the overlay.
It forwards event-loop messages and executes commands from every setter,
`Update`, and `Close`. `internal/app/qr_test.go` retains the host integration
checks for keys, layout, removal/replacement cleanup, and PNG export decoding.

Try `./gloss examples/qr-links.csv`, select a URL (Down), and press `u`.
`e` exports `qr.png`. The compact overlay shows a single URL footer, ellipsized
when needed; closing it returns to the original table cell. The complete code
and four-module quiet zone are preserved. No URL is fetched by displaying it,
and localhost URLs do not become reachable from another device.

For joint local development with sibling checkouts, run:

```sh
go work init . ../ntqrcode
(cd examples/demo && go work init . ../.. ../../../ntqrcode)
```

If a workspace already exists, use `go work use` to add the same paths.
Both `go.work` files and their sums are ignored. The separate demo workspace
keeps its Bubble Tea WASM replacement out of the native build. Ordinary
`task build` and `task demo-check` then use the local library.
With an active workspace, `task build` always invokes Go so edits in sibling
modules cannot leave a stale binary. Go's own incremental cache still applies.
`task ci`, `task notices`, `task notices-check`, and `task release` always set
`GOWORK=off`: release checks and notices must describe the committed module
pins, not sibling checkouts. CI also forces a build so an earlier workspace
binary cannot be reused. GoReleaser disables workspaces as well.
Run `GOWORK=off task ci` and `GOWORK=off task demo-check` before tagging.
When updating the library, update both module pins (prefer a published tag),
regenerate notices, and validate with workspaces disabled. Do not commit a
filesystem `replace` into either module.

The standalone library's `examples/qrcode` demonstrates two components and
has native and WASM builds. Real terminal/font/tmux and phone-camera checks
remain manual acceptance checks; automated decoders do not replace them.

## The file browser

`internal/browse` is a component of its own: it knows nothing of gloss, and
takes what gloss adds (marks, sort, which files may be chosen) as options, over
any `fs.ReadDirFS`. `internal/app/opener.go` is the adapter. Folders are read
by commands and cached; a *layout* only draws that state (`list`, `columns`,
`places`), so keys, filter, and completion are the same in each. Paths are
slash-separated from the filesystem's root.

Its tests drive it through `internal/browse/browsetest` (see its README), which works for any
component with `Init`, `Update`, and `View`: an in-memory filesystem (with
latency, injected read errors, and read counts), a driver that sends keys and
clicks and settles the commands that follow, and scripts. A script is a file in
`internal/browse/testdata/scripts`: an `fs` part listing files and a `script`
part of commands (`press`, `type`, `click`, `snapshot`, `state`, ...; see
`browsetest.RunDir`). Snapshots are compared with the `.golden` file beside the
script, which is a readable screen:

```sh
task browse:test                                 # run them
task browse:screens                              # write the golden screens
```

Scripts understand the words of [VHS](https://github.com/charmbracelet/vhs)
tapes where the two overlap (`Type "re"`, `Down 2`, `Ctrl+L`, `Alt+Up`,
`Screenshot`; recording commands such as `Sleep` and `Set` are ignored), so a
tape reads as one. What VHS has no word for is ours: `fs`, `state`, `expect`,
`reads`, `click`. A script can also become a tape, to record a GIF of the
behavior it tests:

```sh
task browse:tape SCRIPT=columns     # dist/browse/columns.tape
task browse:gif SCRIPT=columns      # records dist/browse/columns.gif (needs vhs, ttyd, ffmpeg)
task browse:gifs                    # the showcase scripts
```

The tasks are the usual ones: `task browse:test`, `task browse:screens` (rewrite
the golden screens), and `task browse:bench`. `CMD`, `KEYS`, and `PAUSE` change
what the tape runs, the keys it substitutes, and its pace; the tool behind
them is `go run ./internal/browse/browsetest/tape` (see its `-h`).

The tape types the command, presses the keys, and leaves out what a recording
cannot do (checks, clicks, resizes, the `fs` part: it runs in a real folder).
VHS has no Home or End, and takes Alt only with a character, so a script's
`alt+up` is left as a comment unless `-keys alt+up=Ctrl+Up,alt+left=Ctrl+O`
stands in keys the browser also answers to. With `vhs` installed, a test
checks that every script makes a tape VHS accepts.

### Playing with it

The harness can also be driven by a person, on the same in-memory trees the
scripts use:

```sh
task browse:play                              # the chooser over a demo tree
task browse:play -- -from ~/Downloads         # over a copy of a real folder (names, sizes, dates)
task browse:play -- -script internal/browse/testdata/scripts/places.txt   # a script's tree and options
task browse:record NAME=thing                 # play, and keep what you do as a script
task browse:replay SCRIPT=places              # watch a script step by step, its checks shown
```

While recording, `F1` takes a snapshot, `F2` records checks of the state
(`state dir …`, `state current …`), `F3` leaves a note to edit; `Ctrl-C` ends and
writes the script, and `browse:record` then writes its golden screens. The
recording keeps the typing, keys, clicks, and wheel as the script words for them,
at a fixed size (90x20) so goldens stay small and alike, and puts the tree it
used in the script's `fs` part (look before sharing a `-from` copy: names and
sizes are real). In a replay `Space` does a step and then its checks, `p` plays
by itself (`+`/`-` change the pace), `r` starts again, `q` leaves; the status line
shows each check as it passes or fails, and the command exits non-zero if any did.

### Soundness checks

Every settled screen of a script is checked (`Screen.Problems`) for the faults
that mess up a terminal: more rows than there are; a row wider than the terminal
by **either** of two width tables, grapheme clusters and wcwidth, which disagree
about some emoji (a family of people joined with zero-width joiners is two cells
to one and six to the other), so a row that fits by one and overflows by the
other wraps on some terminals; and a control character in the text, as a file
name can carry. `col TEXT N` asserts the cell column something is drawn at, which
a wide character shifts from where letters would say, and `reject-raw TEXT`
asserts that nothing a name says reaches the terminal as an escape sequence.
`TestNoSizeOrNameMakesAnUnsoundScreen` sweeps every layout over 26 widths and 9
heights with awkward names (CJK, emoji, joined and selected sequences, combining
marks, very long names, bells, escapes) and fails on any unsound screen. Names
are drawn as names (`names.go`): control bytes as their symbols (`␇`, `␛`), and a
cluster the two tables count differently as its first character.

To try an idea, add a script, run it with `-update-screens`, and read the
golden file; then keep it. After changing how anything is drawn, the diff of
the golden files is the review. `go test -bench . ./internal/browse` times
typing and drawing in a folder of 100,000 files.

## Documentation

The documentation is written from the code where it can be, so it cannot say
what gloss does not do. gloss has one default command, `view` (what `gloss FILE`
runs), and two small ones, `skill` and `help`; `view`'s options are grouped into
*domains* (opening and viewing, documents, meshes, exporting, text and details,
handing files over, agents), and `cmd/gloss/domains.go` is where an option is
given its domain. A test fails for an option that has none.

```sh
task docs                       # the gloss(1) man page, shell completions, the command reference, two guide pages
task docs:hugo:serve            # the site, while you edit it (needs hugo, extended)
task docs:hugo:build            # the site, in docs/hugo/public
```

- The **command reference**, the **man page**, and the **shell completions** are
  generated by gloss itself (`--docs-markdown`, `--docs-man`, `--docs-completions`,
  hidden from `--help`) from its options. What an option's value may be (`--type`,
  `--view`, and so on) is listed in `cmd/gloss/completions.go`; a test holds each
  list to the option's usage text.
- The **guide** (`docs/hugo/content/guide`) is written by hand, except two pages
  that `internal/tools/docsite` makes: *For LLMs*, from `skills/gloss/SKILL.md`,
  and *Development*, from this file. Edit those sources, not the generated pages.
- The theme, [hugo-book](https://github.com/alex-shpak/hugo-book), is a Git
  submodule. The site wears the Nimble brand from `docs/hugo/assets/_custom.scss`
  (the palette in both color modes, and the same Open Sans fonts as the demo,
  mounted from `web/fonts`), with a wordmark partial in `docs/hugo/layouts`.
- The man page and the completions ship in the release archives, the Debian
  package, and the Homebrew cask (`manpages` and `completions` in
  `.goreleaser.yaml`).

## Releases

Pushing a `v*` tag runs checks, and [GoReleaser](https://goreleaser.com)
packages macOS, Linux, and Windows amd64 and arm64 binaries: archives, `.deb`
packages, and SHA-256 checksums go to a GitHub Release, and a cask to the
[Homebrew tap](https://github.com/NimbleTerminal/homebrew-tap). Locally,
`task release` produces the same in `dist/` as a snapshot, without publishing.
Release binaries use software STL rendering when native GPU support is
unavailable. The macOS binaries are signed and notarized with the hardened
runtime, which forbids executing memory the process wrote itself unless an
entitlement allows it; `.goreleaser.entitlements.plist` grants
`com.apple.security.cs.allow-unsigned-executable-memory` for PDFium's
compiled WebAssembly (wazero maps code without `MAP_JIT`, so `allow-jit` alone
does not suffice). Without it, macOS kills the process with `Code Signature
Invalid` on its first PDF; a test in `internal/tools/releasecheck` keeps the
file and the entitlement in the configuration. Builds from source are
ad-hoc signed without the hardened runtime and never needed it.

## Plain web picker

The full `--serve` page distinguishes show-only and pick chrome. Its
`viewerLifetime` tracks either an explicit hard deadline or, for detached
show-only sessions, a renewable idle deadline. Token-guarded, same-origin POSTs
record trusted browser input; long-poll GETs report warnings and settlement
without renewing anything. Explicit viewer quit uses `Options.OnQuit` to
distinguish completion from transport loss. Graceful HTTP shutdown lets a
pending session poll receive the reason before connections close.
The idle deadline is persisted in `viewer-lease.json` inside the existing
private session directory. Read-only status/resume observation and pruning
consult it; renewal never recreates the state file removed by cancellation.
Startup/status expose an additive `idle_timeout_seconds` field. Picker defaults
and positive `--timeout` values remain fixed deadlines.

`gloss --pick-web --prompt "Choose a receipt" --timeout 10m` serves the
experimental HTML/JS upload page, without starting Booba or a terminal model.
Off a terminal it detaches as usual; open the returned `url`, then use
`gloss --resume TOKEN` to retrieve the confirmed paths. Localhost is the default;
only this mode supports opt-in network binding.

`cmd/gloss/pick_network.go` separates `--listen IP:port` (default
`127.0.0.1:0`) from `--advertise-host IP-or-DNS-name`. Validation happens before
detaching and again at server creation. Listeners use explicit `tcp4`/`tcp6`
families, so wildcard behavior is consistent across operating systems. A
wildcard requires a non-loopback advertised host; a specific IP supplies its
own default.
DNS names are advertised aliases, never resolved for binding or authorization.
Interface names, scoped/link-local addresses, and reverse proxies are deferred.

After the socket opens, an immutable host policy captures the actual port,
advertised host, specific bind IP if any, and `localhost` for loopback binds.
Wildcard listeners accept only the advertised authority, not arbitrary local
IPs or DNS names. IP literals, DNS case, and HTTP's default port are normalized;
the API requires Origin to match the requesting authority when present.
Forwarded headers cannot override either check. Token checking precedes all
page/API access, and the full viewer retains its localhost policy. Advertising
a Tailscale name is not an interface or client access restriction. HTTPS proxy
support needs a separate explicit origin/trust design.

The same URL is announced on stderr and persisted for detached startup JSON.
No protocol fields or exit codes change. Tests cover wildcard and IPv6 socket
binding, advertised DNS without external resolution, denied hosts/origins,
port failures, and default/wildcard detached upload-and-message round trips.
They cannot prove another device can route to the address: check real LAN and
Tailscale access manually, including client isolation and firewall policies.

Foreground network picks show a QR on terminal stderr automatically.
`cmd/gloss/pick_qr.go` selects this only with terminal stdin/stderr, no detached
token, and a non-loopback listener/advertised host. It runs the server wait
alongside `internal/app/handoff.go`, a small Bubble Tea screen using the same
`terminalPicture` setup and update helpers as the pager. The existing QR
component receives capability/geometry updates and uses the shared image-ID
allocator. There are no synchronous terminal probes or separate input readers.
The server owns settlement; its completion closes the screen through Tea so
ID-specific graphics cleanup runs before quitting. Terminal cancellation
cancels the server context. OS signals remain owned by `served`, not a second
Bubble Tea signal handler. Stdout never carries the screen. Tests decode its
half-block output independently and exercise resizing, graphics toggling, and
settlement cleanup. The detached startup protocol stays unchanged.

`cmd/gloss/pick_web.go` owns the request state. Under the token URL, `GET files`
lists completed uploads as `{state, files: [{id, name, size}]}`; `POST files`
accepts multipart uploads and returns the new file entries; `DELETE files/ID`
removes one; `POST confirm` takes `{ids: [...], message?: "..."}`;
`POST decline` declines. Message text is bounded to 2,000 Unicode code points,
with a 32 KiB confirmation-body cap allowing JSON escapes and 200 upload IDs.
Whitespace-only text becomes absent; other whitespace and Unicode are preserved.
An accepted confirmation can only be retried with the same paths and message.
Only IDs name uploads across this boundary. The page cannot read host paths or
download files. `pick-api.mjs` is the frontend boundary a hosted prototype could
replace; no hosted service or deployment is included here.

API operations serialize upload, removal, and confirmation. The existing
bounded receiver stores files; settlement drains the HTTP response before
shutdown and removes unconfirmed uploads. Refresh recovers completed uploads;
the final result remains in detached status/resume state, not at the page URL.
The page keeps the unsent message in session storage scoped to the token URL,
clearing it on Send/Cancel. Storage failures do not prevent sending. Accepted
messages are persisted with the detached answer and exposed only in JSON;
ordinary stdout stays paths-only. The protocol remains version 1: the optional
`message` field is an additive change. The reflected schema documents it.
There is no durable browser receipt or interrupted-upload resumption in this
picker. Request tests cover the detached round trip, token/origin checks,
confirmation IDs, CSP and other security headers, 200/201-file boundaries,
255-byte filename limits, concurrent uploads and removal/confirmation races,
and QR wait exit codes. `task ci` exercises these under the race detector;
`task web-check` tests the JS adapter.

Upload-only `--pick-web` requests have a fixed two-minute total deadline in both `http.Server.ReadTimeout`
and `web/pick-api.mjs`, not an idle timeout. A 128 MiB file needs about 9 Mbps
of uplink before overhead; slower links must send smaller files. `--timeout`
controls the session lifetime and does not extend this per-upload deadline.
Ordinary viewer uploads use a 30-second read-progress deadline instead.
The shared receiver caps request bytes (session limit plus 1 MiB), all multipart
fields (256), and concurrency (four). `uploadQuotaReader` reserves each chunk
with a CAS before returning it to the file writer; errors refund all charged
bytes and delete partial files. Reservations share the counter used by exports.

## The browser demo

```sh
task demo                         # embedded gallery in your terminal
task demo -- --sample landscape.heic
task serve-wasm-site               # http://localhost:8000
task build-wasm-site               # static site in web/dist
task web-check                     # browser helper tests; Node 18+
```

`examples/demo` pins the same Bubble Tea WASM fork used by the NTCharts demos;
this replacement does not affect the native CLI. Samples are compiled into the
app with `go:embed` and read through the same document loaders. User files are never uploaded to the public site. URL drops and app `?src=`
links fetch directly in the browser with CORS, credentials omitted, no referrer,
a one-minute deadline, and a streamed byte limit. `--accept` (or `?accept=`
in the public app) validates content using the Go document code before adding it.
There is no public fetch proxy. Browser PDF rendering uses the NTCharts PDFium bridge,
which loads PDFium from the `@embedpdf/pdfium` npm package. The generated shim
points at a CDN, so `scripts/build-site.sh` instead downloads that exact version
from the npm registry, checks it against a pinned SHA-512, serves it from
`vendor/embedpdf-pdfium` on the site, and rewrites the shim to match (and fails
if a CDN address is left). To move to another version, change the version and
hash together in that script. Every page then runs only code from its own
origin, which its Content-Security-Policy (a `<meta>` tag in each page) enforces;
requests to other hosts are limited to document addresses a visitor asks to open. Other
runtime assets are served alongside the site. Meshes are drawn with WebGPU where the browser
has it, and by the software renderer where it does not.

Mesh charts request Kitty shared-memory transport, with direct transmission
when it is unavailable. Their Kitty placeholder grids already fill the viewport;
only the short header and footer need padding. `BenchmarkMeshView` measures this
path, and `TestMeshBodyIsUnframed` checks it across resizes. A browser image cache
must also distinguish graphics revisions: retransmission can reuse the same
WASM pixel address and length, so allocation identity alone does not identify
unchanged image contents.
The browser terminal holds painting during synchronized output (DEC mode
2026), with a one-second recovery deadline. Valid image placeholders awaiting
their placement paint background only, rather than a font's missing-glyph box.
Canvas2D reads one viewport and each image's metadata once per frame; backing
canvas dimensions use the same integer rounding during resize and comparison.
Use `?renderer=canvas2d` (or `webgl`) on the terminal demo to compare display
backends independently of the mesh renderer.

Embed the standalone terminal on another site:

```html
<iframe src="https://nimbleterminal.github.io/gloss/demo.html?sample=field-guide.pdf"
        title="gloss live terminal" width="100%" height="560"
        style="border:0" loading="lazy"></iframe>
```

Omit `sample` to start in the file menu; accepted filenames are listed in
`examples/assets.go`. The native keys work in the demo; quitting offers a
restart button. Choosing another format restarts the embedded terminal at that
sample; clicking the active format preserves the session. Restart explicitly
reloads it. The loading screen reports received bytes and compilation and
startup stages.

The Pages workflow builds for pull requests and deploys pushes to `main`. Set
repository **Settings → Pages → Source → GitHub Actions** to enable hosting.

## License

MIT; see [LICENSE](LICENSE). The licenses of the modules gloss links
are in [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).


## Markdown link activation

Normal Markdown keeps OSC 8 sequences through wrapping and search highlighting.
`internal/app/links.go` hit-tests those sequences in terminal cells; linked image
blocks retain their parent link. Typeset spans carry destinations into drawing
operations and `Result.Links` reports clipped raster-pixel rectangles, including
linked images. Link destinations stay out of the shaping cache. Hit testing
inverts picture's integer FitContain mapping and rejects stale/pending viewports.
Typeset views with links request all-motion mouse reporting and use the same
hit test for a status-bar URL preview. Hover does not need a host opener and
never fetches a destination. Normal Markdown retains cell-motion reporting.

A left-button press/release on the same cell and URL calls `Options.OpenLink`
outside the event loop. Movement, scrolling, source view and overlays cancel or
suppress activation. The native CLI supplies its browser launcher; `--serve` and
browser embeddings leave the callback nil so remote clicks never launch a browser
on the server. Only absolute HTTP(S) URLs without credentials/control characters
reach the callback. This action does not use document fetching or `--fetch`.

## Typeset text selection

A typeset page is a picture, so the terminal cannot select from it, while the
text screens keep the mouse only for the wheel and clicks and leave selection
to the terminal's Shift override. `internal/app/typeset_select.go` follows a
plain left drag over the page: the press is a page point, each motion asks
for a reflow with `Request.TypesetSelect` holding the two ends, and the
result's picture has the text between them highlighted behind the glyphs
(`layout.rasterTo`, `Palette.Selection`). `internal/document/typeset_select.go`
resolves an end to a caret, a text operation and a rune offset, from a point
(nearest line, then the glyph cluster's nearer edge, from the run's glyph
advances) or from a place. Every text operation carries its paragraph's
runes and which paragraph of which block it is, which is the same at every
width, so the result gives both ends back as `TypesetPlace`s and the viewer
keeps those: a scroll, zoom, reflow or theme change re-highlights the same
text. A drag does not cancel the job in flight, as a scroll does: a finished
job for an earlier point of the drag shows the right part of the page with
the selection behind, so it is shown and the current selection asked for
(`typesetReflowed`); its placed ends are adopted only when `TypesetAsked` is
the selection as it is now. `Result.TypesetSelected` is the text in reading order: a wrapped line
is a space, a paragraph a line break, a table cell a tab, a block a blank
line. `y` sends it with `tea.SetClipboard`, OSC 52, which works through SSH
and in the browser build and needs no clipboard package; `Esc`, a click or a
new drag clears it. Shift and Alt drags are not taken. The glyph fallback
maps cells through the same fitted geometry as links do.

Theme catalog and selection adapt go-thinkt (see `internal/theme/README.md` and
`LICENSE.thinkt`). User JSON is resolved into immutable, content-addressed
catalog entries at the host boundary. Rendering receives a reference to the
snapshot, never a mutable global active theme. `Options.ThemeChoices` and
`Options.SaveTheme` let native hosts supply local preferences; web hosts keep
choices in memory. Syntax tokens survive shaping so palette changes reuse layout.

### Typeset font pairs

`internal/document/typeset_fonts.go` defines four paired looks. Requests carry
`Font`/`Mono`, results carry `FontKey`, and stale viewer results must match it.
The loader keeps prepared content/assets across changes, clears shaping/glyph
caches when faces change, and reuses the old block starts to anchor the new
layout. Face fallback is resolved through go-text's segmenter; warnings are
bounded to 32 code points. Fonts are parsed per engine, never globally mutated.

`internal/app/font_picker.go` renders bounded samples asynchronously via the
same typesetter, with a fresh picture ID, owner/revision checks, cancellation,
and cleanup on close/resize/quit. No terminal probes are added. Native font
preferences are host callbacks; previews and headless exports do not read them.
The source pins, licenses, subset recipe and checksums are documented in
`internal/document/fonts/README.md`. Regeneration needs FontTools 4.60.2;
building and using gloss need only the embedded subsets.

## Line numbers, go to line, and chopped lines

Line numbers are file lines, shown only where a view's rows are file lines
(`internal/app/line_numbers.go` has the keys, the prompt and the status
detail). The text view (`markdownView.lineMap`) knows them in two cases: the
source view, where every unit is one line; and a file shown as one fenced code
block, as plain text, source and JSON are, where the file's lines are the units
between Glamour's margin rows. The margin is measured once per theme by
rendering a block of one known line (`fenceMargin`), and the rows are compared
with the file's lines, whitespace aside, before they are numbered: a mismatch
means no numbers rather than wrong ones. Rendered Markdown prose and JSON
records have no mapping; `#` and `:` then say to use the source view. The
gutter is prepended after search highlighting, with the wrap width reduced by
its columns, and a wrapped line is numbered on its first row only.

A typeset text, source or JSON document already has one block per line
(`prepared.lines`), so with `Request.TypesetLineNumbers` the layout lays a
monospace number left of each block (`layout.number`) and moves the text right
by the width of the widest number (`layout.gutterWidth`). Numbers are
operations flagged `gutter`: drawn in the muted color, skipped by caret
placement, selection spans and the selected text, and given paragraph −1 so
places in the text are the same with or without them. Numbers on or off is
part of the layout's identity, like its width, so toggling relays out and the
visible block is anchored as on a reflow. `TypesetViewport.Line` on a request
asks for a line at the top, which the loader lays out to and answers as it
answers `End`; on a result `Line` is the line at the reading top (the block
under the page margin, `layout.lineAt`) and `Lines` the count, zero for a
Markdown page. The viewer clears its pending line when a result arrives and
counts a pending line or a different gutter as a view that does not match.

Chopping (`S`, `--chop-long-lines`) replaces wrapping, as `less -S` does. In
the text view the layout wraps at an unbounded width, so a logical line is one
row holding the whole styled line, and `view` cuts the window `left` columns
in with `ansi.Cut`, after search highlighting and before the gutter, so styles
and highlights survive and the gutter stays put. `scrollX` moves by a quarter
of the width and stops at the widest row; `reveal` brings a search match that
is off the edge a third of the way in. Tables are laid out within the width
as before. A typeset text page lays each line out at an unbounded width
(`layout.chop`), tracks the right edge of its widest operation (`extentX`),
and the pan clamp in `renderTypeset` takes the greater of the page width and
that edge. Chop is part of the layout's identity with width and numbers, and
of the viewer's match test through `TypesetViewport.Chop`; a Markdown page
ignores it, since its blocks are not lines.


## Native app handoff

`internal/openwith` separates portable Host/Target/Choice values from native
catalog, preferences and process code (`!js`). `Options.OpenWith` is installed
only in the direct native pager. The app chooser snapshots the target and uses
commands/results tagged by operation and document generation; it intercepts keys
before the browser filter and owns an opaque modal body while open. Underlying
view state is retained, animation is paused and no image placeholders are drawn
through the chooser. `browse.OpenWithMsg` carries intent without selecting a file.
The harness records emitted messages and browsecfg exposes the intent to scripts.

The catalog/probe, trusted-ID preference and nonblocking launch design follows
`go-thinkt/internal/config/apps.go` and its platform catalogs; no thinkt code or
dependency is imported. Launch uses argv, never a shell, and reaps the child in
a goroutine. Preferences are limited to 64 KiB and atomically replaced; malformed
files cannot be silently overwritten. Tests inject probes and fake child
processes, and exercise picker keys through the event loop. The native CLI owns
stdin/download provenance. `--apps` reads preferences for discovery but never
launches or loads a document; exports do not read these preferences.

### Native app visibility

The Open with chooser's `m` checklist uses the optional `openwith.Manager`
host interface. Lists include installed apps even when hidden; System default
and Reveal are excluded. Saves use the same bounded atomic preferences as
per-format defaults, and failed writes leave in-memory preferences unchanged.
The UI loads and saves through tagged messages in the existing event loop.
The checklist interaction (arrows, Space to toggle) follows
[thinkt's setup app chooser](https://github.com/wethinkt/go-thinkt/blob/main/internal/tui/setup/steps_apps.go);
the implementation is native to gloss and adds no dependency.

### Terminal editors

The optional native `Editor(Target)` capability prepares an `exec.Cmd` from
VISUAL/EDITOR using Google's shlex parser. No shell is inserted. Only the pager
calls `tea.ExecProcess`; the asynchronous GUI `Launch` path refuses this action.
The app uses tagged preparation and completion messages, rejects stale requests,
and clears Kitty placements and animation work before yielding the terminal.
After completion it reloads the document or refreshes the current menu/browser.
Tests run a child process through Bubble Tea to verify writes and nonzero exits,
and cover source/scroll restoration and typeset reload after atomic replacement.
Actual editor interaction and Kitty/tmux restoration still require terminal tests.
