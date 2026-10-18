# nttypeset Extraction Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move gloss's direct typesetter into the standalone module `nimbleterminal.dev/nttypeset` without changing a pixel of gloss's typeset output.

**Architecture:** First make the engine theme- and animation-free inside gloss (roles resolved from a palette at paint time, animated images as reserved rectangles, explicit contexts). Then move the model, engine, layout, rasterizer, selection, highlighter and line builder into the new module behind the API in the spec, with gloss's `internal/document` reduced to parsing, asset loading, viewport policy and compositing. Finally create `nimbleterminal.dev/ntfonts` for the embedded OFL subsets, move the goldmark adapter if open question 1 says so, tag both modules `v0.1.0`, and pin them.

**Tech Stack:** Go 1.26 (CI 1.27.2), go-text/typesetting v0.3.5, go-text/render v0.2.1, chroma v2.27.0, x/image v0.47.0; goldmark v1.8.2 and goldmark-emoji v1.0.5 in the adapter package only. Go Task, GitHub Actions.

**Spec:** `docs/superpowers/specs/2026-10-18-nttypeset-extraction-design.md` (in the gloss repo). Read it first; this plan uses its names and argues from it.

**Repositories:** gloss at `~/projects/gloss` (branch off `main`); nttypeset at `~/projects/nttypeset` and ntfonts at `~/projects/ntfonts` (both empty clones of `github.com/NimbleTerminal/<name>`; the first push of each creates `main`; both vanity imports already resolve). Pattern repo: `~/projects/ntpicture`.

## Global Constraints

- Module paths `nimbleterminal.dev/nttypeset` and `nimbleterminal.dev/ntfonts`; `go 1.26` in both go.mod files; CI `go-version: '1.27.2'`.
- nttypeset's core package imports only go-text (`typesetting`, `render`), chroma and x/image beyond the standard library. No goldmark, ntpicture, Bubble Tea, ntfonts or `nimbleterminal.dev/gloss/...` anywhere in the module, tests included (tests use `golang.org/x/image/font/gofont`). ntfonts imports only the standard library and never nttypeset; the two meet through the string-typed `FaceSource`.
- No panic leaves the module: every exported method recovers and returns an error; `context` errors pass through unwrapped (`errors.Is(err, context.Canceled)`); limits return `*LimitError` with `errors.Is(err, ErrLimit)` and today's message text.
- Default limits: 32,768 blocks; 32,768 style runs per block; 16,384 runes per paragraph; 262,144 glyphs; 32,768 display-list entries; 11,250,000 layout pixels height; 32 Mi raster pixels; 15 s cumulative layout time; 32 MiB shaped-run cache; 32 MiB glyph-mask cache. A `Limits` value may lower, never raise.
- `Margin = 50`; heading sizes 50, 46, 42, 38, 33, 29 by level; default size 25; indent step 24; code inset 16; gap after a non-Line block 16; gap before a heading 16; image gap 20; rule 2 px high then 20 gap; gutter width is the widest number plus 20.
- Every phase ends with gloss `GOWORK=off task ci` green and the pixel baseline (Task 1) unchanged; from Task 7 on, nttypeset `task ci` green too.
- Gloss's `go.mod` names resolvable nttypeset and ntfonts versions at every commit: pushed pseudo-versions until Task 21 pins `v0.1.0` of each. `go.work` (ignored) carries `../nttypeset` and `../ntfonts` locally. Never commit a filesystem `replace`.
- Tests move with their code and keep their assertions (two change form, per the spec's Tests section). New tests use plain `testing`, no assertion library.
- Commit messages follow gloss's style (`Pin NT libraries to published releases`, `fix: ...`): imperative, one line, no trailing period.
- The spec was approved 2026-10-18 with all three open questions settled: the goldmark adapter ships as `nttypeset/goldmark` (Task 19 runs), the fonts move to ntfonts (Task 18 runs), and `experiments/typeset` takes the one-line fonts-path change. The "if no" branches in Tasks 18–19 are dead; Task 17 no longer stops for answers.

## Review Focus

Failure modes the spec implies but did not pin to a test; each is added to the owning task below.

1. A `FaceSource` returning bytes that are not a font: `NewEngine`/`SetFonts` must return an error, not panic (Task 10).
2. A layout width below the margins (width 1): layout terminates, every line gets at least 32 pixels, no infinite loop (Task 12).
3. An empty `Document`: `Raster` returns a background-filled image of the view size, `Done()` is true, `Extent()` is `2*Margin` (Task 12).
4. A `View` with `Pixels` of zero or negative width and nonzero height, or one over `Limits.Pixels`: an error, never a panic or a zero-size image (Task 12).
5. `Select` with a `Placed` end whose `Block` is past the document: `nil, nil`, not a panic or an endless `Ensure` (Task 13).

---

## Phase 0: the pixel oracle

### Task 1: Typeset export baseline script

**Files:**
- Create: `scripts/typeset-baseline.sh` (gloss)
- Modify: `.gitignore` (gloss), only if `dist/` is not already ignored

**Interfaces:**
- Produces: `scripts/typeset-baseline.sh record|check` writing and comparing `dist/typeset-baseline/MANIFEST.sha256`.

- [ ] **Step 1: Write the script**

`scripts/typeset-baseline.sh` builds gloss (`GOWORK=off task build`), then for each of `examples/typeset.md`, `examples/notes.txt`, `examples/batch.jsonl`, for each `--font` in `readable editorial technical compact`, for each `--theme` in `light dark`, exports with `./gloss --typeset --page all --dpi 150 --font F --theme T --output-dir dist/typeset-baseline/<file>-<F>-<T>/ <input>`; and for the two text files additionally `--line-numbers` and `--chop-long-lines` variants (suffix `-N`, `-S`). `record` then writes `shasum -a 256` of every PNG, sorted by path, to `dist/typeset-baseline/MANIFEST.sha256`. `check` exports into `dist/typeset-check/` the same way and fails if any hash differs or any file is missing or extra, printing the differing paths. Exit nonzero on any difference.

- [ ] **Step 2: Record the baseline at the pre-extraction commit**

Run: `scripts/typeset-baseline.sh record && wc -l dist/typeset-baseline/MANIFEST.sha256`
Expected: a manifest with more than 40 lines, no errors.

- [ ] **Step 3: Verify the check is stable**

Run: `scripts/typeset-baseline.sh check`
Expected: `baseline unchanged` (or the script's equivalent), exit 0. If two runs differ at the same commit, stop: the export is nondeterministic and the oracle needs fixing before anything moves.

- [ ] **Step 4: Confirm `dist/` is ignored, then commit**

```bash
git check-ignore dist/typeset-baseline/MANIFEST.sha256   # prints the path
git add scripts/typeset-baseline.sh
git commit -m "Add a typeset export baseline script for the nttypeset extraction"
```

## Phase 1: theme- and animation-free engine inside gloss

### Task 2: Roles and a paint-time palette replace sentinel colors

**Files:**
- Modify: `internal/document/typeset_layout.go` (`span`, `operation`, `inline`, `walk`, `rasterTo`, `typesetFill`, `rect`)
- Modify: `internal/document/typeset_highlight.go`
- Modify: `internal/document/typeset_render.go` (build the palette per request)
- Modify: `internal/document/font_preview.go` (palette instead of `theme.Get`)
- Modify tests: `typeset_theme_test.go` (`TestTypesetDarkSyntaxAndFills`), `typeset_highlight_test.go`, `typeset_glyph_test.go`

**Interfaces:**
- Produces, inside package `document`: `type role uint8` (`roleText`, `roleAccent`); `type fillRole uint8` (`fillBorder`, `fillCode`, `fillHeader`, `fillMuted`); `type palette struct { background, foreground, muted, border, code, header, accent, selection color.RGBA; syntax *chroma.Style }`; `func paletteFor(theme string) palette` (from `theme.Get(theme)` and `styles.Get(theme.Syntax(theme))`); `span.role role` replacing `span.darkInk`; `operation.fill fillRole` replacing `operation.fill color.Color`; `(l *layout) rasterTo(..., p palette, base bool)`.

- [ ] **Step 1: Rewrite `TestTypesetDarkSyntaxAndFills`**

It asserts: `highlightCode("func hi() string { return \"hello\" }", "go", codeTheme)` yields at least two spans with distinct nonzero `token`; rastering a `Code` block through `rasterTo` with `paletteFor("dark")` paints the code background pixel at the block's inset corner `theme.Get("dark").Code`, and the first keyword glyph's ink is `theme.Ink("dark", chroma.Keyword)` (check one pixel the test finds by scanning the first text run's rect for the darkest-against-background pixel). Keep the test name.

- [ ] **Step 2: Rewrite the two highlight assertions**

In `TestHighlightCode`, replace `colors[s.ink]` with `tokens[s.token]` and assert `len(tokens) >= 2`. In `TestHighlightedRasterResizeAndThemeCache`, keep the raster-color check (it reads pixels), and replace the monokai shaping-collision check with: two span slices equal except one has `ink: color.RGBA{1,2,3,255}` shape to different `*shaped` pointers.

- [ ] **Step 3: Run the three tests to see them fail**

Run: `cd ~/projects/gloss && GOWORK=off go test ./internal/document -run 'TestTypesetDarkSyntaxAndFills|TestHighlightCode|TestHighlightedRasterResizeAndThemeCache'`
Expected: FAIL (compile errors on `role`/`paletteFor`).

- [ ] **Step 4: Implement roles and the palette**

In `typeset_layout.go`: `span` loses `darkInk`, gains `role`; `inline` and `walk` set `role: roleAccent` on links, autolinks and heading spans instead of the two `theme.Get` inks; `rect` takes a `fillRole`; the code background, table header, rule and quote-bar fills use `fillCode`, `fillHeader`, `fillBorder`, `fillMuted`; `rasterTo` takes `p palette` and resolves ink as: `p.foreground`; `roleAccent` → `p.accent`; `op.ink.A != 0` → `op.ink`; `op.token != 0` → `p.syntax.Get(op.token).Colour` if set, else `p.foreground`; gutter → `p.muted`; selection → `p.selection`; background → `p.background`. Delete `typesetFill`. The shaping cache key includes `role` where it included `darkInk`. In `typeset_highlight.go`: set `token` and bold/italic from `theme`; set `ink` only when `theme != codeTheme` (keeps the test-only path), never `darkInk`. In `typeset_render.go`: `l.typesetEngine.theme` is replaced by passing `paletteFor(out.Theme)` to `rasterTo`; `font_preview.go` likewise. Remove the `theme` import from `typeset_layout.go`.

- [ ] **Step 5: Run the document tests and the baseline**

Run: `GOWORK=off go test ./internal/document && scripts/typeset-baseline.sh check`
Expected: PASS; `baseline unchanged`.

- [ ] **Step 6: Commit**

```bash
git add internal/document
git commit -m "Resolve typeset colors from a palette at paint time"
```

### Task 3: Animated images become reserved rectangles

**Files:**
- Modify: `internal/document/typeset_layout.go` (`block`, `prepared`, `rasterTo`), `typeset_render.go`, `typeset.go`, `loader.go`
- Modify tests: `typeset_theme_test.go` (`TestTypesetThemePreservesImage` reads `l.typesetPrepared.images`), `typeset_linked_image_test.go`

**Interfaces:**
- Produces: `block.animated bool`; `prepared` loses `animations`; `Loader.typesetAnimations map[string]*animation.Player` keyed by destination; `(l *layout) images(top, left int, scale float64, viewport image.Rectangle) []placedImage` with `placedImage{rect image.Rectangle; destination string}`; `rasterTo(..., omitAnimated bool)` replacing `base`.

- [ ] **Step 1: Add a test that animated images are omitted from the base and reported**

In `typeset_linked_image_test.go`, `TestTypesetAnimatedImageIsReservedRectangle`: a two-frame GIF `anim.gif` and `![a](anim.gif)` loaded with `Animate: true`; `r.AnimationBase` at the image rect's center equals the light background; `r.Animated` has one region whose `Destination` is `anim.gif`; without `Animate`, `r.Image` at the same point is the GIF's first-frame color.

- [ ] **Step 2: Run it to see it fail**

Run: `GOWORK=off go test ./internal/document -run TestTypesetAnimatedImageIsReservedRectangle`
Expected: FAIL at the `AnimationBase` assertion or compile (`animated` field missing). If it passes already, keep it as the pin and continue.

- [ ] **Step 3: Move the players out of the layout**

`walk` sets `block.animated = p.animated[dest]` from a `map[string]bool` the loader fills from the asset list; `rasterTo` skips `op.asset` when `omitAnimated && op.animated` (operation gains `animated bool`); `renderTypeset` builds `out.Animated` from `lay.images(...)` filtered by `l.typesetAnimations[dest] != nil`, and `rasterTo(..., len(out.Animated) > 0)`. `Loader.closePDF` clears `typesetAnimations`. `TestTypesetThemePreservesImage` keeps reading the still through `l.typesetPrepared.images`.

- [ ] **Step 4: Run the tests and the baseline**

Run: `GOWORK=off go test ./internal/document && scripts/typeset-baseline.sh check`
Expected: PASS; `baseline unchanged`.

- [ ] **Step 5: Commit**

```bash
git add internal/document
git commit -m "Keep animation players out of the typeset layout"
```

### Task 4: Explicit contexts on layout entry points

**Files:**
- Modify: `internal/document/typeset_layout.go` (`engine.check`, `ensure`, `rasterTo`, `text`, `table`), `typeset_render.go`, `typeset_select.go` (`place` calls `ensure`), `font_preview.go`
- Modify test: `typeset_cancel_test.go` (sets `lay.e.ctx` directly)

**Interfaces:**
- Produces: `(l *layout) ensure(ctx context.Context, bottom int)`, `(l *layout) rasterTo(ctx, ...)`, `(l *layout) resolve(ctx, s TypesetSelection)`; `engine.ctx` is set and cleared by these entry points (`defer`), never by the loader.

- [ ] **Step 1: Change the cancel test to pass the countdown context**

`TestTypesetCancelledRequestKeepsLayout` calls `lay.ensure(countdown{...}, 1125*MaxPages)` instead of assigning `lay.e.ctx`.

- [ ] **Step 2: Run it to see it fail to compile**

Run: `GOWORK=off go test ./internal/document -run TestTypesetCancelledRequestKeepsLayout`
Expected: FAIL (wrong argument count).

- [ ] **Step 3: Thread ctx through**

Entry points set `l.e.ctx = ctx` and `defer func() { l.e.ctx = nil }()`; internal callers (`text`, `table`, `block`, `number`) keep calling `l.e.check()`. `renderTypeset` and `FontPreview` stop assigning `engine.ctx`.

- [ ] **Step 4: Run the tests, race detector on, and the baseline**

Run: `GOWORK=off go test -race ./internal/document && scripts/typeset-baseline.sh check`
Expected: PASS; `baseline unchanged`.

- [ ] **Step 5: Commit**

```bash
git add internal/document
git commit -m "Pass the request context into typeset layout entry points"
```

### Task 5: A self-contained font sample routine

**Files:**
- Modify: `internal/document/font_preview.go`
- Test: `internal/document/typeset_fonts_test.go` (`TestFontPreviewBoundedAndCancelled`, plus one new assertion)

**Interfaces:**
- Produces: `func previewRows(ctx context.Context, looks []FontLook, override, body, code string, width, height int, p palette, newEngine func(name, override string) (*engine, error)) (*image.RGBA, error)` — the drawing half of `FontPreview`, which becomes sniffing plus one call. This is the function Task 14 moves as `Preview`.

- [ ] **Step 1: Add an assertion**

In `TestFontPreviewBoundedAndCancelled`: the returned image, split into four equal rows, has ink (a pixel not equal to the background) in every row.

- [ ] **Step 2: Run it**

Run: `GOWORK=off go test ./internal/document -run TestFontPreviewBoundedAndCancelled`
Expected: PASS already (it pins behavior before the refactor).

- [ ] **Step 3: Refactor `FontPreview` into sniffing plus `previewRows`**

`previewRows` builds, per look, an engine, an empty `prepared`, lays the three sample strings with `l.text` keeping first-baseline runs as today, and rasters one row. Row height `height/4`, size `max(10, min(24, rowHeight/5))`, `y` starts at 4 and advances `size*3/2 + 2`, text at `x = 10` width `width-20`. Bounds check 160–1200 stays in `FontPreview`.

- [ ] **Step 4: Run the tests and commit**

Run: `GOWORK=off go test ./internal/document`
Expected: PASS.

```bash
git add internal/document/font_preview.go internal/document/typeset_fonts_test.go
git commit -m "Split the font preview into Markdown sniffing and sample rows"
```

### Task 6: Phase 1 gate

- [ ] **Step 1: Full gate and baseline**

Run: `cd ~/projects/gloss && GOWORK=off task ci && scripts/typeset-baseline.sh check`
Expected: every task passes; `baseline unchanged`.

- [ ] **Step 2: Nothing to commit; note the commit hash in the PR description or branch notes as "phase 1 complete".**

## Phase 2: the module, with raster and the engine

### Task 7: Scaffold the nttypeset repository

**Files (all in `~/projects/nttypeset`):**
- Create: `go.mod`, `doc.go`, `LICENSE`, `README.md`, `AGENTS.md`, `DEVELOP.md`, `CHANGELOG.md`, `NOTICE.md`, `CODE_OF_CONDUCT.md`, `THIRD_PARTY_NOTICES.md`, `Taskfile.yml`, `.gitignore`, `.github/workflows/ci.yml`, `internal/tools/notices/main.go`

**Interfaces:**
- Produces: a module that `task ci` passes on, pushed as `main`.

- [ ] **Step 1: Write the scaffold from ntpicture's**

`go.mod`: `module nimbleterminal.dev/nttypeset`, `go 1.26`, no requires yet. `doc.go`: `// Package nttypeset lays out and rasters styled text: blocks of spans in, viewport images out.` `LICENSE`: ntpicture's MIT text with `Copyright (c) 2024-2026 Neomantra Corp`. `CODE_OF_CONDUCT.md`: copy ntpicture's. `README.md`: title `nttypeset — Typesetting for Nimble Terminal`, the pkg.go.dev and CI badges with `nttypeset` in the URLs, a two-paragraph description (what it draws, what it does not do: no terminal output, no parsing), install line, and a "Status: extracted from gloss, API settling before v0.1.0" line; the quick-start code block is added in Task 14. `AGENTS.md` and `DEVELOP.md`: ntpicture's structure (what this is, commands, module layout, invariants, tests and reporting, documentation) with nttypeset content: one module, no browser build, the invariants list from the spec (no panics across the boundary, limits lower-only, caches bounded, display list private, core imports). `CHANGELOG.md`: `# nttypeset CHANGELOG` and an `## Unreleased` heading. `NOTICE.md`: source MIT; a line saying the faces come from `nimbleterminal.dev/ntfonts`, which carries the OFL notices. `Taskfile.yml`: ntpicture's with `env: GOWORK: 'off'` and tasks `fmt`, `fmt-check`, `go-tidy`, `go-tidy-check`, `go-verify`, `test`, `test-race`, `vet`, `lint` (golangci-lint v2.14.0 into `.task/bin`), `build`, `cross-windows`, `cross-wasm` (`GOOS=js GOARCH=wasm CGO_ENABLED=0 go build -mod=readonly ./...`), `notices`, `notices-check`, `vulncheck` (govulncheck v1.8.0), `ci` (fmt-check, go-tidy-check, go-verify, test-race, vet, lint, cross-windows, cross-wasm, build, notices-check, vulncheck), `clean`; no `examples` or `wasm` tasks. `.gitignore`: `/dist/`, `*.test`, `/bin/`, `/.task/`, `.DS_Store`. `ci.yml`: ntpicture's without the Node step and with `cache-dependency-path: go.sum`. `internal/tools/notices/main.go`: ntqrcode's generator (module LICENSE files via `go list -m -json all`), header `nttypeset is MIT licensed; see LICENSE.`; run it once to produce `THIRD_PARTY_NOTICES.md` (standard library only at this point).

- [ ] **Step 2: Run the gate**

Run: `cd ~/projects/nttypeset && task ci`
Expected: every task passes (vulncheck needs network).

- [ ] **Step 3: Commit and push, creating `main`**

```bash
git add -A
git commit -m "Scaffold the nttypeset module"
git push -u origin main
```

### Task 8: Move the glyph-mask rasterizer

**Files:**
- Create: `~/projects/nttypeset/raster/raster.go`, `raster/raster_test.go` (moved from gloss `internal/typeset/raster/`)
- Modify: `~/projects/nttypeset/go.mod` (go-text typesetting v0.3.5, render v0.2.1, x/image v0.47.0)

**Interfaces:**
- Produces: `nimbleterminal.dev/nttypeset/raster` with the same API as gloss's package (`New`, `Stats`, `SetBudget`, `Clear`, `Draw`, `DrawDirect`).

- [ ] **Step 1: Copy both files unchanged, `go mod tidy`, run the test**

Run: `cd ~/projects/nttypeset && go get github.com/go-text/typesetting@v0.3.5 github.com/go-text/render@v0.2.1 golang.org/x/image@v0.47.0 && go mod tidy && go test ./raster`
Expected: PASS (`TestDrawCachesShapedGlyphs`).

- [ ] **Step 2: `task notices` then `task ci`; commit; push**

```bash
task notices && task ci
git add -A && git commit -m "Move the glyph-mask rasterizer from gloss" && git push
```

Gloss's copy is deleted in Task 16.

### Task 9: Document model, limits and errors

**Files:**
- Create: `~/projects/nttypeset/document.go`, `limits.go`, `limits_test.go`

**Interfaces:**
- Produces (exactly the spec's "Document model" and "Limits and errors" blocks): `Style` with `Bold, Italic, Mono, Strike`; `Role` with `RoleText, RoleAccent`; `Span`; `Kind` with `Paragraph, Heading, Quote, Code, Line, Rule, Image, Table`; `Quote`; `Bounds` with `Fit(w, h int) (int, int)` and `Offset(available, width int) int` (gloss's `ImageLayout` methods); `Image`; `Block`; `Document`; `Limits` with `func (l Limits) withDefaults() Limits` (unexported); `ErrLimit`; `LimitError{What string; Limit int}` with `Error()` returning `What` and `Is(target error) bool` true for `ErrLimit`; `const Margin = 50`.

- [ ] **Step 1: Write `limits_test.go`**

`TestLimitsDefaults`: `Limits{}.withDefaults()` equals the ten default values from Global Constraints. `TestLimitsOnlyLower`: `Limits{Glyphs: 1 << 30}.withDefaults().Glyphs == 262144` and `Limits{Glyphs: 100}.withDefaults().Glyphs == 100`. `TestLimitErrorIs`: `errors.Is(&LimitError{What: "typeset layout exceeds 15 seconds"}, ErrLimit)` and the message is the `What`. `TestBoundsFit`: `Bounds{Width: 100}.Fit(400, 200) == (100, 50)`, `Bounds{}.Fit(400, 200) == (400, 200)`, `Bounds{Align: "center"}.Offset(300, 100) == 100`, `"right"` gives 200.

- [ ] **Step 2: Run to see it fail; implement; run to see it pass; commit; push**

Run: `go test . -run 'TestLimits|TestBounds'`
Expected: compile FAIL, then PASS.

```bash
git add document.go limits.go limits_test.go && git commit -m "Add the block and span model and the layout limits" && git push
```

### Task 10: Fonts: looks, face source, fallback

**Files:**
- Create: `~/projects/nttypeset/fonts.go`, `fonts_test.go`, `internal/testfonts/testfonts.go` (gofont `FaceSource` for tests)

**Interfaces:**
- Consumes: `Style` (Task 9).
- Produces: the string constants `VariantRegular = "Regular"`, `VariantBold = "Bold"`, `VariantItalic = "Italic"`, `VariantBoldItalic = "BoldItalic"` with `func (s Style) Variant() string` (from the Bold and Italic bits); `FaceSource` with `Face(family, variant string) ([]byte, error)`; `FontLook`; `FontLooks()`, `ResolveFont(name, override string) (FontLook, error)`, `FontKey(name, override string) string`; unexported `faces` struct holding `[8]*font.Face`, the lazy `fallback [8]*font.Face`, `fallbackFailed [8]bool`, `missing map[rune]bool`, `fallbackUsed map[string]bool`, and `fallbackMap` implementing go-text's `shaping.Fontmap`; `noLigatures []shaping.FontFeature`; `func loadFaces(src FaceSource, look FontLook) ([8]*font.Face, error)`. `internal/testfonts.Source` maps `"Go"`→`gofont/goregular|gobold|goitalic|gobolditalic` and `"Go Mono"`→`gomono…`, and the look `FontLook{Name: "go", Body: "Go", Code: "Go Mono"}` is `testfonts.Look`; `testfonts.Source.Face` returns an error for any other family (so Literata/Iosevka fallback fails, per the spec).

- [ ] **Step 1: Write `fonts_test.go`**

`TestResolveFont`: `ResolveFont("", "")` is readable with Body `Literata` and Code `Iosevka`; `("readable", "jetbrains")` has Code `JetBrains Mono`; `("missing", "")` and `("readable", "other")` error; `FontKey("", "")` is `readable/`. `TestStyleVariant`: `Style(0).Variant() == "Regular"`, `(Bold|Mono).Variant() == "Bold"`, `(Bold|Italic|Strike).Variant() == "BoldItalic"`. `TestLoadFacesNamesMissingFamily`: a source lacking `"Literata"` makes `loadFaces` return an error whose text contains `Literata`. `TestFontLooksTable`: four looks in order readable, editorial, technical, compact with the spec's families. `TestLoadFacesRejectsBadBytes` (Review Focus 1): a `FaceSource` returning `[]byte("not a font")` makes `loadFaces` return an error. `TestFallbackFailureCountsMissing`: with `testfonts.Source`, a `fallbackMap` resolving `'漢'` returns the primary face, `fallbackUsed` stays empty, `missing` holds the rune, and a second call does not call `Face` again (count calls in a wrapping source).

- [ ] **Step 2: Run to see it fail; implement from gloss's `typeset_fonts.go`; run; commit; push**

Run: `go test . -run 'TestResolveFont|TestFontLooksTable|TestLoadFaces|TestFallback'`
Expected: compile FAIL, then PASS.

```bash
git add fonts.go fonts_test.go internal/testfonts && git commit -m "Add font looks, the face source and glyph fallback" && git push
```

### Task 11: Engine, shaping cache and highlighting

**Files:**
- Create: `~/projects/nttypeset/engine.go`, `shape.go`, `highlight.go`, `engine_test.go`, `highlight_test.go`
- Modify: `go.mod` (chroma v2.27.0)

**Interfaces:**
- Consumes: Tasks 9, 10; `raster.New`.
- Produces: `Option`, `WithFonts`, `WithLook`, `WithLimits`; `Engine`; `NewEngine(opts ...Option) (*Engine, error)` (error `nttypeset: WithFonts is required` without a source); `SetFonts`, `Look`, `FontKey`, `Fallbacks`, `Missing`, `GlyphCache`, `SetGlyphBudget`; unexported `shaped` and `(e *Engine) shape(spans []Span, size int) *shaped` with the 16,384-rune limit and the 32 MiB cache; `(e *Engine) check()` panicking with `e.ctx.Err()`; `Highlight(ctx, source, language string, style *chroma.Style) []Span`.

- [ ] **Step 1: Write the tests (moved assertions)**

`engine_test.go`: `TestNewEngineRequiresFonts` (error without `WithFonts`); `TestSetFontsClearsCaches` (shape something, `SetFonts("go", "")` with a second test look returns `changed == true` and `GlyphCache().Entries == 0`; same look again returns `false`); `TestShapeCacheReuseAndKeys` from `TestTextWrappingPreservesUnicodeAndStyles`'s cache half plus the `Ink` collision assertion from Task 2; `TestShapeRuneLimit` (16,385 runes → recovered `*LimitError` from a small exported wrapper used only in tests: `func (e *Engine) shapeForTest(spans []Span, size int) (s *shaped, err error)` in `export_test.go`); `TestFontFallbackMissingAndLigatures` (moved: with `testfonts`, `Missing()` after shaping `"🙂漢"` has two runes and `"ffi"` shapes to three glyphs). `highlight_test.go`: `TestHighlightCode` (moved, with `Token`), `TestHighlightTokenLimitAndCancel` (a cancelled ctx returns one plain Mono span).

- [ ] **Step 2: Run to see them fail; implement from gloss's `engine`, `shape`, `highlightCodeContext`; run; `task notices`; commit; push**

Run: `go test . -run 'TestNewEngine|TestSetFonts|TestShape|TestFontFallback|TestHighlight'`
Expected: compile FAIL, then PASS.

```bash
task notices && git add -A && git commit -m "Add the shaping engine and chroma highlighting" && git push
```

### Task 12: Layout, raster, links and images

**Files:**
- Create: `~/projects/nttypeset/layout.go`, `raster.go`, `view.go`, `palette.go`, `lines.go`, `layout_test.go`, `lines_test.go`, `testdata/typeset.md`, `testdata/landscape.png` (copied from gloss `examples/`)

**Interfaces:**
- Consumes: Task 11.
- Produces: `Palette`, `LightPalette()`, `DarkPalette()` (gloss's two base palettes with `styles.Get("github")`/`("github-dark")`); `LayoutOptions`; `(e *Engine) Layout(doc *Document, width int, opts LayoutOptions) (*Layout, error)`; `Layout` with `Ensure`, `EnsureBlock`, `Complete`, `Done`, `Laid`, `Starts`, `Extent`, `ExtentX`, `Width`, `LineAt`, `Find`, `Raster`, `Links`, `Images`; `View` with `Size`, `Project`; `RasterOptions`; `Link`; `Placed`; `Lines(ctx, text, language string, style *chroma.Style) (*Document, error)`; unexported `operation`, `(l *Layout) ensure(bottom int)`, `block()`, `text(...)`, `table(...)`, `number(...)`, `gutterWidth(...)`, `rasterTo(...)` as in gloss, and `func (l *Layout) guard(ctx context.Context, err *error)` — the one recover-at-the-boundary helper every exported method defers (`*err` becomes the recovered error: a `context` error as is, a `*LimitError` as is, anything else wrapped as `fmt.Errorf("nttypeset: %v", p)`).

- [ ] **Step 1: Write `layout_test.go` and `lines_test.go`**

Build documents directly or via a small test helper `mdDocument(t, path)` that parses `testdata/typeset.md` with goldmark + GFM + emoji *in the test file only* (`_test.go` may import goldmark; the package may not) and maps it with a test-local copy of the walk — or, if open question 1 is already answered yes, with the Task 19 adapter. Tests: `TestFixtureAndProgressiveLayout` (moved, through `Layout`/`Raster` with `View{Width: 800, Height: 200, Scale: 1}`), `TestTextWrappingPreservesUnicodeAndStyles` (moved, wrapping half, via a `Paragraph` block and `Laid`/`Starts`/operation inspection), `TestHighlightedRasterResizeAndThemeCache` (moved raster half), `TestTypesetGlyphMasks` (moved: cached raster equals a second raster; mask-versus-`DrawDirect` tolerance mean ≤ 12, worst ≤ 160; `SetGlyphBudget(1<<12)` keeps `Bytes <= 2*Budget`), `TestLayoutLimits` (16,385-rune paragraph and a 262,145-glyph document return `ErrLimit`), `TestCancelledEnsureRollsBackOneBlock` (the countdown half of gloss's cancel test: after cancellation `len(Starts()) == Laid()`, and resuming yields the same operation rectangles as a fresh layout), `TestLinksWrapReflowAndViewport` (moved rectangle half), `TestDarkPaletteFillsAndTokens` (dark `Code` background and a keyword's ink from `DarkPalette().Syntax`), `TestLineNumbersGutterAndChop` (gutter ops are Mono, muted, at `x >= Margin`, not text; `Chop` lays a 2,000-rune line on one row and `ExtentX()` exceeds `Width()`), `TestLineAt`, `TestFind` (lays out until the href appears). Review Focus 2–4: `TestWidthOneTerminates` (width 1, `Complete` returns nil, every text op has `Dx() >= 1`), `TestEmptyDocument` (`Done()`, `Extent() == 100`, `Raster` of `View{Width: 100, Height: 50, Scale: 1}` is all background), `TestViewPixelsErrors` (`Pixels: image.Pt(0, 10)` and `image.Pt(8000, 8000)` with a 32 Mi limit both return errors, no panic; `Pixels: image.Pt(1001, 333)` returns exactly that size). `lines_test.go`: `TestLinesDocument` (tabs → four spaces, empty line → `" "`, `Lines == true`, 8,193 lines → `ErrLimit`, highlighted `go` source keeps a multi-line string token's color across lines).

- [ ] **Step 2: Run to see them fail; implement from gloss's `layout`, `rasterTo`, `rasterSize`, `loadTypesetText`'s block loop; run; commit; push**

Run: `go test .`
Expected: compile FAIL, then PASS.

```bash
git add -A && git commit -m "Add incremental layout, the rasterizer and line documents" && git push
```

### Task 13: Selection

**Files:**
- Create: `~/projects/nttypeset/select.go`, `select_test.go`

**Interfaces:**
- Consumes: Task 12's `Layout` and `operation`.
- Produces: `Place`, `Selection`, `Selected` (holding `*Layout`, `start, end caret`), `(l *Layout) Select(ctx, s Selection) (*Selected, error)`, `(s *Selected) Places() Selection`, `(s *Selected) Text() string`; `RasterOptions.Selection *Selected` honored by `rasterTo` (highlight behind runs with `Palette.Selection`).

- [ ] **Step 1: Move the four selection tests**

`TestTypesetSelectionTextAndPlaces`, `TestTypesetSelectionSeparators`, `TestTypesetSelectionHighlight`, `TestTypesetSelectionPreservesWrappedText` from gloss, building the `selectSource` document with the Task 12 test helper and using `Select`/`Places`/`Text`/`Raster`; `runAt` reads the package's operations directly (same package). Review Focus 5: `TestSelectPastDocument` (`Place{Placed: true, Block: 10_000}` → `nil, nil`, and `Laid()` equals the block count, not more).

- [ ] **Step 2: Run to see them fail; implement from gloss's `typeset_select.go`; run; commit; push**

Run: `go test . -run 'TestTypesetSelection|TestSelectPast'`
Expected: compile FAIL, then PASS.

```bash
git add select.go select_test.go raster.go && git commit -m "Add logical selection and caret placement" && git push
```

### Task 14: Preview, the example, the README

**Files:**
- Create: `~/projects/nttypeset/preview.go`, `preview_test.go`, `example_test.go`
- Modify: `README.md`, `AGENTS.md` (file table), `CHANGELOG.md`

**Interfaces:**
- Produces: `Preview(ctx, looks []FontLook, override, body, code string, width, height int, p Palette, opts ...Option) (*image.RGBA, error)`; `ExampleLines` in `example_test.go` rendering `Lines(ctx, "func main() {}", "go", styles.Get("github"))` at width 400 into a `View{Width: 400, Height: 100, Scale: 1}` and printing the image bounds (`// Output: (0,0)-(400,100)`).

- [ ] **Step 1: Write `preview_test.go` and `example_test.go`**

`TestPreviewBoundedAndCancelled` (moved: 480×320 with `testfonts.Look` succeeds and every row has ink; cancelled ctx errors; 5000×5000 errors; no `WithFonts` errors). The example as above.

- [ ] **Step 2: Run to see them fail; implement from gloss's `previewRows` (Task 5); run; update README quick start with the example's body and the file table in AGENTS.md; CHANGELOG Unreleased: "Extracted from gloss: block/span model, engine, layout, raster, selection, highlighting, line documents, preview."; `task ci`; commit; push**

Run: `task ci`
Expected: PASS.

```bash
git add -A && git commit -m "Add the font preview, an example and the README quick start" && git push
```

### Task 15: Gloss gets a temporary face source over its embedded fonts

This package is the exact code Task 18 moves into ntfonts; writing it here lets phase 2 land without the second module.

**Files (gloss):**
- Create: `internal/document/fonts/fonts.go` (package `fonts`), `internal/document/fonts/fonts_test.go`
- Modify: `internal/document/typeset.go` (remove the `go:embed`), `internal/document/typeset_fonts.go` (`loadFace` reads through the new package), `internal/document/typeset_fonts_test.go` (`TestFontSubsetHashes` moves to the new package), `go.mod`, `go.work`

**Interfaces:**
- Produces: `type fonts.Source struct{}` with `func (Source) Face(family, variant string) ([]byte, error)` mapping `Literata`→A, `Newsreader`→B, `IBM Plex Sans`→C, `Source Serif 4`→D, `Iosevka`→E, `IBM Plex Mono`→F, `JetBrains Mono`→G and the variant string to the file suffix, over `//go:embed Gloss*.ttf`, erroring with the family or variant name for anything else; `var fonts.Embedded Source`; `fonts.Families() []string`; `fonts.Licenses() []fonts.License{Family, Text string}` reading the seven `*.OFL.txt` in family order. The package imports nothing outside the standard library.

- [ ] **Step 1: Pin nttypeset and add the workspace entry**

Run: `cd ~/projects/gloss && GOWORK=off go get nimbleterminal.dev/nttypeset@main && go work use ../nttypeset && grep nttypeset go.mod`
Expected: a `v0.0.0-<timestamp>-<hash>` pseudo-version in `go.mod`.

- [ ] **Step 2: Write `fonts_test.go`**

`TestFontSubsetHashes` (moved: 28 lines in `subsets.sha256`, each file's SHA-256 matches), `TestSourceFamilies` (every family in `Families()` × the four variant strings returns bytes that `font.ParseTTF` accepts; `Embedded.Face("Nope", "Regular")` and `Embedded.Face("Literata", "Heavy")` error), `TestSubsetCoverage` (moved from `TestFontLooksAndSubsets`: every face has glyphs for `"Café naïve résumé é — “quotes”…"` and none for `"漢🙂"`), `TestLicenses` (seven entries, each `Text` contains `SIL OPEN FONT LICENSE`), `TestEmbeddedIsFaceSource` (`var _ nttypeset.FaceSource = fonts.Embedded`, in the test file only so the package itself stays free of nttypeset).

- [ ] **Step 3: Run to see it fail; implement; make `typeset_fonts.go` read faces through `fonts.Embedded`; run document tests and the baseline; commit**

Run: `GOWORK=off go test ./internal/document/... && scripts/typeset-baseline.sh check`
Expected: PASS; `baseline unchanged`.

```bash
git add go.mod go.sum internal/document
git commit -m "Serve the embedded typeset fonts through an nttypeset face source"
```

### Task 16: Gloss's document package becomes glue

**Files (gloss):**
- Delete: `internal/document/typeset_layout.go`, `typeset_select.go`, `typeset_highlight.go`, `internal/typeset/raster/`
- Create: `internal/document/typeset_blocks.go` (the goldmark walk producing `*nttypeset.Document`, from `walk`/`inline`/`standaloneImage`/`imageLink`; moves to the library in Task 19 if the adapter ships)
- Modify: `typeset_render.go`, `typeset_fonts.go`, `typeset_text.go`, `typeset.go`, `font_preview.go`, `loader.go`
- Delete tests moved in Tasks 11–14: `typeset_select_test.go`, `typeset_highlight_test.go`, `typeset_glyph_test.go`, the moved halves of `typeset_layout_test.go`, `typeset_fonts_test.go`, `typeset_cancel_test.go`, `typeset_links_test.go`, `typeset_lines_test.go`, `typeset_theme_test.go`
- Keep and adapt the policy tests listed in the spec's "Stay in gloss" paragraph

**Interfaces:**
- Consumes: the whole nttypeset API (Tasks 9–14) and `fonts.Embedded` (Task 15).
- Produces, in package `document`: `type TypesetPlace = nttypeset.Place`, `type TypesetSelection = nttypeset.Selection`, `type TypesetLink = nttypeset.Link`, `type FontLook = nttypeset.FontLook`, `func FontLooks() []FontLook`, `func FontKey(name, override string) string` (delegating), `func ResolveFont(name, override string) (FontLook, error)` wrapping the library's errors as `--font must be readable, editorial, technical or compact` and `--mono must be jetbrains`; `Loader` fields `typesetEngine *nttypeset.Engine`, `typesetDocument *nttypeset.Document`, `typesetImages map[string]image.Image`, `typesetAnimations map[string]*animation.Player`, `direct *nttypeset.Layout`; `func typesetPalette(theme string) nttypeset.Palette`; `func newTypesetEngine(font, mono string) (*nttypeset.Engine, error)` (`nttypeset.NewEngine(nttypeset.WithFonts(fonts.Embedded), nttypeset.WithLook(font, mono))`).

- [ ] **Step 1: Run the remaining gloss tests to see what fails to compile**

Run: `GOWORK=off go test ./internal/document 2>&1 | head -40`
Expected: compile failures naming the deleted identifiers; this is the work list.

- [ ] **Step 2: Rewrite `renderTypeset`**

Same policy, in this order: `SetFonts`; width from `typesetMeasure`; new `Layout` when width, fonts, numbers or chop changed; `TypesetView` handling with `EnsureBlock` for anchoring and `Line`, `Complete` for `End`, `Find` for `Link` (after gloss's `FootnotePeer` check), export slices via `Complete` and `Extent()`; then `Ensure(top+height)`; `View{Top, Left, Width: width, Height: height, Scale: scale, Pixels: pixels}`; `out.Animated` from `Images(view)` filtered by `typesetAnimations`; `Select` into `RasterOptions.Selection` and `out.TypesetSelection`/`TypesetSelected`; `Raster` with `OmitAnimated: len(out.Animated) > 0`; composite as today; `Links(view)`; info fields from `Look()`, `Fallbacks()`, `Missing()`. Errors: a `context` error keeps `l.direct`; any other error from the library sets `l.direct = nil` (today's "failed of itself" rule). The 15-second `context.WithTimeout` stays in gloss as it is.

- [ ] **Step 3: Rewrite the rest**

`typeset_text.go`: reading and budgets, then `nttypeset.Lines(q.Context, text, language, styles.Get("github"))`. `typeset.go`: `prepareTypeset` unchanged except it returns the images and animations maps; the caller builds `typesetImages`/`typesetAnimations` and `typeset_blocks.go`'s `buildBlocks(ctx, root, data, images, animated map[string]bool) (*nttypeset.Document, error)` produces the document (the walk's panics become returned errors here). `font_preview.go`: sniffing, then `nttypeset.Preview(ctx, FontLooks(), override, body, code, width, height, typesetPalette(palette), nttypeset.WithFonts(fonts.Embedded))`. `loader.go`: the fields above; `closePDF` clears them.

- [ ] **Step 4: Adapt the staying tests**

Replace `l.direct.next`/`len(l.direct.ops)`/`l.direct.ops` with `l.direct.Laid()`, `l.direct.Starts()`, `l.direct.Done()`; `l.typesetEngine.cacheBytes` with the "same `*Layout` pointer and same `Starts()` across themes" assertion; `TestTypesetLineNumbersGutter` keeps its `TypesetView` checks only; `TestTypesetCancelledRequestKeepsLayout` keeps the request-level half (a cancelled request keeps `l.direct`; a layout that fails of itself drops it).

- [ ] **Step 5: Document tests, app tests, race, baseline**

Run: `GOWORK=off go test -race ./internal/document ./internal/app ./cmd/... && scripts/typeset-baseline.sh check`
Expected: PASS; `baseline unchanged`.

- [ ] **Step 6: Full gate, then commit**

Run: `GOWORK=off task ci`
Expected: PASS (`notices-check` will fail if `THIRD_PARTY_NOTICES.md` was not regenerated: run `GOWORK=off task notices` first; the new entries are go-text and nttypeset, which were already linked or are MIT).

```bash
git add -A
git commit -m "Typeset through nimbleterminal.dev/nttypeset"
```

### Task 17: Phase 2 gate

- [ ] **Step 1: Both gates**

Run: `(cd ~/projects/nttypeset && task ci) && (cd ~/projects/gloss && GOWORK=off task ci && scripts/typeset-baseline.sh check)`
Expected: PASS, PASS, `baseline unchanged`.

- [ ] **Step 2: Continue to phase 3; the open questions were settled with the spec's approval.**

## Phase 3: fonts and adapters (after the open questions)

### Task 18: Create the ntfonts module and move the font subsets

**Files (all in `~/projects/ntfonts` unless marked):**
- Create: `go.mod` (`module nimbleterminal.dev/ntfonts`, `go 1.26`, no requires), `ntfonts.go` (package `ntfonts`: Task 15's `fonts.go` with the embed pattern pointing at the module root), `ntfonts_test.go` (Task 15's tests; `TestEmbeddedIsFaceSource` is dropped here and lives in gloss instead, since ntfonts must not require nttypeset even in tests), `doc.go`, `LICENSE` (MIT, Neomantra Corp), `README.md` (what is embedded: seven OFL families as 28 Latin subset faces under the internal names Gloss Text A–G, ~1.2 MB; the family and variant names `Face` accepts; the `nttypeset.WithFonts(ntfonts.Embedded)` line; how to regenerate), `AGENTS.md`, `DEVELOP.md` (subset regeneration with `fonttools==4.60.2`, the pinned Google Fonts commit and Iosevka release), `CHANGELOG.md` (`## Unreleased`), `NOTICE.md` (source MIT; fonts OFL-1.1, renamed internally, listed by family with upstream URLs from `sources.json`), `CODE_OF_CONDUCT.md`, `Taskfile.yml` (ntpicture's shape: `fmt-check`, `go-tidy-check`, `go-verify`, `test-race`, `vet`, `lint`, `cross-windows`, `cross-wasm`, `build`, `fonts-check` = `shasum -a 256 -c subsets.sha256`, `notices`, `notices-check`, `vulncheck`, `ci`), `.gitignore`, `.github/workflows/ci.yml`, `internal/tools/notices/main.go` (ntqrcode's generator plus one section per `Licenses()` entry with the wording `OFL-1.1; pinned sources and subset recipe in sources.json and scripts/subset-fonts.py; embedded under distinct Gloss Text internal names`)
- Move: gloss `internal/document/fonts/` (all 47 files) → `~/projects/ntfonts/` root (TTFs, `*.OFL.txt`, `sources.json`, `subsets.sha256`, the fonts `README.md` content folded into the module README, the Go reference TTFs and their `LICENSE` renamed `LICENSE.gofont` so the module's MIT `LICENSE` is unambiguous); gloss `scripts/subset-fonts.py` → `~/projects/ntfonts/scripts/subset-fonts.py` (its `root` becomes `Path(__file__).resolve().parents[1]`)
- Modify (gloss): delete `internal/document/fonts/` and `scripts/subset-fonts.py`; `go.mod` requires `nimbleterminal.dev/ntfonts` at a pseudo-version; `go.work` adds `../ntfonts`; `newTypesetEngine` and `FontPreview` use `ntfonts.Embedded`; `internal/document/typeset_fonts_test.go` gains `TestEmbeddedIsFaceSource` (`var _ nttypeset.FaceSource = ntfonts.Embedded`); `internal/tools/notices/main.go` replaces the seven `os.ReadFile(... .OFL.txt)` with `ntfonts.Licenses()` and the description `OFL-1.1; embedded from nimbleterminal.dev/ntfonts under distinct Gloss Text internal names`; `Taskfile.yml` build `sources` drops `internal/document/fonts/*.ttf`; `experiments/typeset/main.go` and `layout_test.go` default path `../../../ntfonts` (open question 3; if the answer is "keep a copy", instead leave the 28 TTFs and the OFL files in `internal/document/fonts/` with a README line saying they exist for the experiment only, and keep the Taskfile `sources` entry)

**Interfaces:**
- Produces: `ntfonts.Source`, `ntfonts.Embedded`, `ntfonts.Families()`, `ntfonts.Licenses()`, `ntfonts.License{Family, Text string}`, exactly Task 15's shapes under the new package name.

- [ ] **Step 1: ntfonts: scaffold, move, run tests and the gate**

Run: `cd ~/projects/ntfonts && go test ./... && task notices && task ci`
Expected: PASS; `THIRD_PARTY_NOTICES.md` holds the Go license and seven font sections; `fonts-check` reports 28 `OK` lines.

- [ ] **Step 2: Commit and push ntfonts, creating `main`**

```bash
git add -A && git commit -m "Create ntfonts with the OFL subset faces from gloss" && git push -u origin main
```

- [ ] **Step 3: gloss: pin ntfonts, switch to `ntfonts.Embedded`, delete the directory and script, fix notices, Taskfile and the experiment**

Run: `cd ~/projects/gloss && GOWORK=off go get nimbleterminal.dev/ntfonts@main && go work use ../ntfonts && GOWORK=off go mod tidy && GOWORK=off task notices && GOWORK=off go test ./internal/... ./cmd/... && GOWORK=off task ci && scripts/typeset-baseline.sh check && (cd experiments/typeset && go test ./...)`
Expected: all PASS; `baseline unchanged`; `git grep "internal/document/fonts"` finds only the CHANGELOG; `THIRD_PARTY_NOTICES.md` still carries the seven OFL texts (now via `ntfonts.Licenses()`) and gains ntfonts's MIT.

- [ ] **Step 4: Commit gloss**

```bash
git add -A && git commit -m "Take the typeset fonts from ntfonts"
```

### Task 19: Move the goldmark walk into `nttypeset/goldmark` (if open question 1 is yes)

**Files:**
- Create: `~/projects/nttypeset/goldmark/goldmark.go` (`Options`, `Build`), `goldmark/goldmark_test.go`, `goldmark/doc.go`
- Modify: nttypeset `go.mod` (goldmark v1.8.2, goldmark-emoji v1.0.5), `README.md` (Markdown-to-PNG example), `AGENTS.md`, `CHANGELOG.md`
- Modify (gloss): `internal/document/typeset_blocks.go` shrinks to building `Options{Images, Bounds: func(im *ast.Image) nttypeset.Bounds { l := MarkdownImageLayout(im); return nttypeset.Bounds{Width: l.Width, Height: l.Height, Align: l.Align} }, Syntax: styles.Get("github")}` and calling `goldmark.Build`
- If open question 1 is **no**: skip; `typeset_blocks.go` stays as Task 16 left it, and Task 12's test helper remains the library's only Markdown walk.

- [ ] **Step 1: Write `goldmark_test.go`**

Parse `../testdata/typeset.md` with `goldmark.New(goldmark.WithExtensions(extension.GFM, emoji.Emoji))` in the test; `TestBuildKinds` (the fixture yields Heading, Paragraph, Quote, Code, Table, Image blocks; the standalone image block has `Image.Key == "landscape.png"` when `Options.Images` has it, and placeholder spans `[image unavailable: missing-typeset-image.png]` otherwise); `TestBuildListsAndQuotes` (ordered list markers `1. `, `2. `; a nested quote carries two `Quote` entries with indents 24 and 48 and `Last` on the final block); `TestBuildLimits` (8,193-line fence → `ErrLimit`; cancelled ctx → `context.Canceled`); `TestBuildBoundsHook` (the hook's `Bounds` lands on the image block).

- [ ] **Step 2: Run to see it fail; implement from gloss's `typeset_blocks.go`; run; `task notices`; commit; push**

Run: `go test ./goldmark && task ci`
Expected: PASS.

```bash
git add -A && git commit -m "Add the goldmark adapter" && git push
```

- [ ] **Step 3: gloss: re-pin, shrink `typeset_blocks.go`, test, baseline, commit**

Run: `GOWORK=off go get nimbleterminal.dev/nttypeset@main && GOWORK=off go test ./internal/document && scripts/typeset-baseline.sh check && GOWORK=off task ci`
Expected: PASS; `baseline unchanged`.

```bash
git add -A && git commit -m "Build typeset blocks with nttypeset/goldmark"
```

### Task 20: Documentation in both repositories

**Files (gloss):**
- Modify: `DEVELOP.md` "Typeset Markdown" section (the engine is `nimbleterminal.dev/nttypeset` and the faces `nimbleterminal.dev/ntfonts`: what gloss keeps, what it calls; the raster-package sentence becomes the library's `raster`; the embedded-subsets sentences point at ntfonts; the sibling-checkout paragraph lists `../nttypeset ../ntfonts` in the `go work init` lines and names both among the pinned releases), `AGENTS.md` layout list (`internal/document` bullet: typeset layout/raster/selection come from nttypeset, faces from ntfonts; drop the `internal/typeset/raster` mention; add both to the "use published version pins" sentence; the "fonts" guide ownership line is unchanged), `CHANGELOG.md` Unreleased ("Typesetting moved to the nttypeset and ntfonts libraries; no visible change.")
- Modify (nttypeset, ntfonts): `README.md`, `AGENTS.md`, `DEVELOP.md` final pass; `CHANGELOG.md` Unreleased complete in both.

- [ ] **Step 1: Edit, then run gloss's docs check and both gates**

Run: `cd ~/projects/gloss && GOWORK=off task docs-check && GOWORK=off task ci && (cd ~/projects/nttypeset && task ci) && (cd ~/projects/ntfonts && task ci)`
Expected: PASS.

- [ ] **Step 2: Commit all three**

```bash
(cd ~/projects/nttypeset && git add -A && git commit -m "Document the extracted module" && git push)
(cd ~/projects/ntfonts && git add -A && git commit -m "Document the module" && git push)
git add -A && git commit -m "Document the nttypeset and ntfonts extraction"
```

## Phase 4: publish and pin

### Task 21: Tag v0.1.0 of both modules and pin them

ntfonts first: nothing depends on it, so it can be tagged and verified on the proxy before nttypeset's README example (which names `ntfonts.Embedded`) goes out.

- [ ] **Step 1: ntfonts: CHANGELOG `## v0.1.0 (YYYY-MM-DD)` from Unreleased (first-release note: the seven families, the subset recipe, extracted from gloss), commit, tag, push**

```bash
cd ~/projects/ntfonts && task ci
git add CHANGELOG.md && git commit -m "Release v0.1.0" && git tag v0.1.0 && git push && git push origin v0.1.0
```

Expected: `GOPROXY=proxy.golang.org go list -m nimbleterminal.dev/ntfonts@v0.1.0` prints the version within a few minutes.

- [ ] **Step 2: nttypeset: the same, with ntpicture's first-release voice (extracted from gloss, what it does, pair with ntfonts)**

```bash
cd ~/projects/nttypeset && task ci
git add CHANGELOG.md && git commit -m "Release v0.1.0" && git tag v0.1.0 && git push && git push origin v0.1.0
```

Expected: `GOPROXY=proxy.golang.org go list -m nimbleterminal.dev/nttypeset@v0.1.0` prints the version.

- [ ] **Step 3: gloss: pin both, notices, full gates**

Run: `cd ~/projects/gloss && GOWORK=off go get nimbleterminal.dev/ntfonts@v0.1.0 nimbleterminal.dev/nttypeset@v0.1.0 && GOWORK=off go mod tidy && GOWORK=off task notices && GOWORK=off task ci && GOWORK=off task demo-check && scripts/typeset-baseline.sh check`
Expected: PASS; `baseline unchanged`; `go.mod` shows `nimbleterminal.dev/ntfonts v0.1.0` and `nimbleterminal.dev/nttypeset v0.1.0`.

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum THIRD_PARTY_NOTICES.md
git commit -m "Pin nttypeset and ntfonts to v0.1.0"
```

- [ ] **Step 5: Remove the baseline directory when the branch merges (`rm -rf dist/typeset-baseline dist/typeset-check`); the script stays for the next engine change.**

---

## Self-review notes

- Spec coverage: seam (Tasks 2–5, 16), package layout (7–14, 18–19), every API block (9–14), ntfonts module (15 as the staging copy, 18), gloss table (15–16, 18–19), tests split (11–16, 18), pixel oracle (1), phases (6, 17, 21), open questions (19 and the experiment line of 18 conditional, 17 stops), documentation constraint (20), pseudo-version/pin rule (15, 18, 19, 21).
- Type consistency checked: `FaceSource` (string variants), `FontLook`, `Engine`, `Layout`, `View`, `RasterOptions`, `Selected`, `Placed`, `Link`, `Palette`, `Limits`, `LimitError` are named identically in Tasks 9–16 and 18–19; `fonts.Embedded` (Task 15) and `ntfonts.Embedded` (Task 18) have the same shape; gloss aliases in Task 16 match the spec's table.
- Review Focus 1–5 are pinned in Tasks 10, 12, 12, 12, 13.
