# `gloss` CHANGELOG

## Unreleased

- `F2` opens a language picker that previews as you move; the terminal viewer
  remembers the choice, which `GLOSS_LANG` and `--lang` still override.
- Speak Spanish and Simplified Chinese: the help, status line, the viewer's
  notes and messages, menus and choosers, the file browser's help, the
  details box (`i`), search, export, handoff screen and
  `--serve`/`--pick-web` pages follow `--lang`, `GLOSS_LANG` or the terminal's
  locale (a detached page follows the browser), and numbers are written to
  match. Scripts stay in English unless `--lang` is given; keys, options and
  exit codes never change. Drafts awaiting native review; the docs site has
  Spanish and Chinese guides.
- See HTML pages as their own HTML and CSS lay them out (nthtml), with their
  local pictures, in your font and theme where they set none: `t` in the viewer, scrolled and zoomed like a typeset page,
  with links that open on a click and text to select and copy, in the thumbnail grid, and with `--output`,
  in slices that end between lines. Exporting one was refused before; `--typeset` still exports the
  Markdown made of it.
- Open GeoJSON and GPX files as maps: places as pins, tracks, routes and lines
  as lines, areas as outlines, framed to fit. Pan with the arrows or a drag,
  zoom with `+`/`-` or the wheel, `f` frames the whole file again, and `i`
  lists what it holds. Maps are drawn on a plain background and nothing is
  fetched. `--output`, `e`, `--print` and previews draw them too;
  `--type geojson|gpx` and `--accept maps` name them. ntmaps v0.5.1.
- Open raster PMTiles archives (`.pmtiles`, or by their first bytes) as maps
  of their tiles, read from the file as the map needs them, so an archive of
  any size opens; `i` gives its name, zooms, bounds and credit. Vector
  archives are described but not drawn. `--type pmtiles`; `--accept maps`
  includes them.
- With `--fetch`, a map service you name draws its tiles under GeoJSON and GPX
  maps (`GLOSS_MAP_TILES`, an XYZ tile address, credited by
  `GLOSS_MAP_ATTRIBUTION`), and `/` on a map finds a place by name through a
  Nominatim-compatible lookup (`GLOSS_GEOCODER`). gloss names no service of
  its own; tiles are fetched only for the screen and kept in memory, each place
  is asked once a session, and headless modes stay offline.

## `v0.5.0` (2026-10-23)

- Play videos: MP4, M4V, MOV, WebM, MKV, Ogg (Theora), AVI, MPEG, WMV and FLV,
  through ntvideo and mpv's library (`brew install mpv`). They open paused on
  the first frame; Space plays, ←/→ seek, and the bottom row is a scrub bar.
  With FFmpeg installed, `i` and `--info` describe them, and previews,
  thumbnails, `--output` and `--print` use a frame. `--type video` and
  `--accept video/*` name them. In the browser demo and app, videos play in the
  browser's own player, with the same keys; the demo has a sample.
- Show a video's subtitles: files beside it named after it (`clip.srt`,
  `clip.en.srt`, `.vtt`, `.ass`) and its own tracks; `c` changes track or
  turns them off, and `i` names the files. Drawn with glyphs, they are terminal
  text over the picture; `g` switches a video between glyphs and Kitty graphics.
- Export frames of a video at chosen times: `--at 1:23,2:05`, or `--frames 9`
  spread through it, as one sheet with each frame's time, or a file each with
  `--output-dir`. With `--json` each frame carries its time and the subtitle
  shown then. `--at` alone starts the viewer there. `--text` prints a video's
  subtitles with their times; `--info` lists its subtitle streams.
- Right-click a picture, video, link, Markdown table or spreadsheet cell, or
  press `a` or Shift-F10, for a menu of what to do with it:
  - pictures and videos: open or edit with another app, copy the image to the
    clipboard, copy the path, and save a video's frame;
  - links, and web addresses written out in any text, typeset too: open or
    copy the address;
  - Markdown tables: open as a sheet in gloss or in another app, or copy as
    TSV or Markdown;
  - spreadsheet cells: copy the value or row, open the address, show its QR.

  With `--fetch`, a link's or an address cell's menu also opens it in gloss or
  saves the linked file in the current folder, never over another file.
- In rendered Markdown, text or typeset, Tab and Shift-Tab move between
  pictures, links and tables, highlighted, instead of switching files. `]` and
  `[` switch files everywhere, and Tab still does elsewhere.
- Hovering a link on a typeset page tints it, every line of it, as well as
  showing its address in the status bar. A theme's `hover` color sets the
  tint; without one it is the selection color halfway to the page.
- The Open with and Edit with choices open in a box over the screen, which
  stays in view, rather than in place of it.
- Click a file in the file menu to select it, and again to open it; the wheel
  moves the selection. Click a spreadsheet cell to select it; a second click on
  one with an address opens it, as Enter does with `--fetch`. Both work in the
  browser demo too.
- The status bar of a text screen says how to select text: Shift-drag, or
  ⌥-drag in iTerm2, since gloss takes plain drags for the wheel and clicks.
- Fixes: selecting text on a typeset page drawn with glyphs starts and ends
  under the pointer, as hovering a link does, and the page fills the screen.
  Quitting with the mouse moving no longer leaves mouse reports, such as
  `35;61;18M`, typed at the shell prompt.
- Internals: video comes from ntvideo v0.3.0; nttypeset v0.1.3.

## `v0.4.0` (2026-10-21)

- Open OBJ and PLY meshes: OBJ with material colors and PNG/JPEG textures, PLY
  with vertex colors, and PLY point clouds. `--up y|z` sets which axis is up.
- Draw Mermaid diagrams: fenced `mermaid` blocks in Markdown and standalone
  `.mmd`/`.mermaid` files, in the theme's light or dark palette. No browser or
  network is used.
- Edit files with `E`. Text files go to `$VISUAL`/`$EDITOR`, other files to the
  system's default editor. In the chooser, Tab switches between Open with and
  Edit with.
- Open with lists the apps and defaults your system knows (Launch Services on
  macOS, XDG on Linux, the registry on Windows). Saved choices carry over and
  now live in `open-with-v1.json`.
- `--qr TEXT` shows text, such as a URL, as a QR code: in the viewer, on a
  `--serve` page, or as a PNG with `--output`. An agent can hand you a link to
  scan with a phone or tablet: `gloss --serve --qr URL`. Nothing is fetched.
- `--print` writes one screen of a file to stdout without a terminal, with
  pictures as colored half-blocks. `--print-width` and `--print-height` size it.
- Recognize JSON Lines by content; `--type` and `--accept` take `jsonl` and
  `ndjson`.
- Report gloss's state (working, waiting for files, done, error) to terminals
  that support OSC 7501.
- Show the version in the `?` help heading.
- PDFs: open macOS (Quartz) and linearized files that were refused before.
  Turning pages, `/` search and `--grep` are much faster on long documents, and
  `--info` no longer renders the page.
- SVG: draw markers such as arrowheads, and percentage lengths.
- `gloss skill`: bare `gloss skill` prints help and `gloss skill show` prints
  the skill. `install` always uses the shared `~/.agents/skills` folder too,
  replaces broken links, and handles `--help`, `auto` and a missing home folder
  correctly.
- Fixes: a detached `--serve` or `--pick` no longer downloads an address given
  to an option, such as `--prompt https://…`; Kitty images and inline Markdown
  images stay on screen after `Q`; a still picture stays visible while its
  background color changes; exports of animated typeset pages show the
  displayed frame; the browser demo's mouse works again.
- **Scripts:** `--apps --json` now has one row per kind and verb, with `verb`
  (`open` or `edit`) and `terminal`. Match rows on `kind` and `verb`.
- Requires Go 1.26.10 or later.
- Internals: typesetting, PDF reading, the file browser and open-with now come
  from the nttypeset/ntfonts, ntpdf, ntfilepicker and ntopenwith libraries.

## `v0.3.2` (2026-10-12)

- Fix signed macOS releases crashing when opening or exporting PDFs.
- Make mesh rotation smoother and reduce flicker in the browser demo.
- Open JPEG XL (`.jxl`) images, including in Markdown and the browser demo.
  Support previews, exports, and image metadata; animated files show frame one.
- Play animated WebP with the same controls as GIFs, including in Markdown.
- Open URLs directly: `gloss https://…/report.pdf`. Downloads also work with
  exports and text extraction. Add `GLOSS_NO_FETCH` to disable downloads and
  `--user-agent` or `GLOSS_USER_AGENT` to set their User-Agent.
- Show local HTML images in Markdown, including linked GIFs, with animation,
  sizing, and alignment.

## `v0.3.1` (2026-10-11)

- Open files in native apps with `w`/`W` or Alt+Enter in the file browser.
  Save per-format defaults, manage visible apps, and list them with `--apps`.
- Edit text files in `$VISUAL` / `$EDITOR` from Open with, then return to gloss
  with the file reloaded and viewing position preserved where possible.
- Support emoji shortcodes, footnotes, and GitHub alerts in terminal text and
  typeset Markdown. Click footnote references and back links to navigate.
- Show file-browser help with `?` when the filter is empty, or `F1` at any time.
- Fix the browser gallery getting stuck on “Preparing image” and a served-viewer
  shutdown race when closing the browser tab.
- Add Homebrew installation tips and remove duplicate skill-install output.
- Move the Go module to `nimbleterminal.dev/gloss`; the repository remains
  `github.com/NimbleTerminal/gloss`.

## `v0.3.0` (2026-10-11)

- Enforce PDF stream limits across filter chains and stop timed-out parser work.
  Prevent concurrent uploads from exceeding the session quota and expire stalled uploads.
- Typeset Markdown, plain text, source code, and JSON with `t` or `--typeset`:
  embedded fonts, highlighted code, tables, local images, and continuous scrolling.
  Export the viewport or numbered PNG slices without a browser or PDF step.
- Preview four font pairs with `f` and color themes with `T`. Save native choices,
  select them with `--font` and `--theme`, or supply a custom theme JSON file.
- Search document text, table cells, and PDF text with `/` and `n`/`N`. Text
  matches survive wrapping and resizing. Add line numbers with `#` (`-N`),
  go to a line with `:`, and chop long lines with `S` (`-S`).
- Select and copy typeset text by dragging and pressing `y`. Copy preserves
  text across visual wraps. Click Markdown links and preview typeset URLs on hover.
- Play GIFs with pause, speed control, frame stepping, and a draggable scrubber.
  Local GIF figures also animate in Markdown. Export the displayed frame or
  typeset viewport; previews and headless exports use the first frame.
- Improve Kitty scrolling, resizing, and animation: keep the previous picture
  until its replacement is ready, cache glyphs, and update only animated regions.
- Browse files in list, columns, or places layouts, with fuzzy filtering,
  path completion, breadcrumbs, and mouse navigation. Help groups controls by media.
- Request uploads and an optional reply with `--pick-web`. Opt-in LAN/Tailscale
  binding (`--listen`, `--advertise-host`) supports phone handoff by QR code.
  LAN HTTP is unencrypted and token-guarded; `--serve` remains localhost-only.
- Show a table URL as a QR code with `u`, and export it with `e`. Use Kitty
  graphics or a half-block fallback without changing the original text.
- Give browser viewers appropriate viewing controls and renewable idle sessions.
  Expiration or connection loss leaves the document visible with a notice.
- Add detached-session cancellation, repeatable status/resume results, JSON
  schemas, PDF text search with `--grep`, and sparse-page hints for agents.
- Accept explicit URL drops with `--fetch` and constrain inputs with `--accept`.
  Browser URL downloads use CORS directly, without a proxy.
- Fix export page validation, retain notebook/Word images beside extracted text,
  outline named mesh views, and use embedded thumbnails for oversized 3MF files.
- Move gloss, the documentation site, and Homebrew tap to NimbleTerminal.
  Adopt NTCharts v2.8.0 and `nimbleterminal.dev/ntqrcode`; require Go 1.26.9+.
  Source installs use a clone because the renderer uses a pinned module replacement.
- Add `gloss view`, `gloss help`, and `gloss skill install`; keep the old skill
  flags as hidden aliases. Make `task build` skip unchanged builds outside a workspace.

## `v0.2.0` (2026-10-02)

  * Agent protocol
    * `--text` takes the text layer of a PDF: one page with `--page`, a range,
      or `all` into `--output-dir` or `--json`. A page with no text layer is an
      `error` entry, not a blank success. Pictures, SVG, and meshes still point
      at `--output`
    * `--vision-profile` is now only a documented alias for `--max-edge`
      (1600, 1092, 1932 px: the square-safe edge for each budget, wide pictures
      are no longer sized up to the patch budget); `--max-edge` given wins. The
      export manifest gains `max_edge`, and `vision_profile` and
      `vision_reason` when a profile was named
    * With no terminal (stdin not a TTY), `--pick` and `--serve` no longer
      block: they print one JSON object (`url`, `dir`, `timeout_seconds`,
      `resume_token`, `resume`) and exit 0, with the server detached and no
      browser opened; `--timeout` defaults to 10 minutes. The new
      `gloss --resume TOKEN` prints the paths and exits 0, 2, or 124
    * `gloss --status TOKEN` reports a detached session's state immediately
      as JSON, without changing the session, state file, or dropped files.
      It exits 0 when a state is reported and 1 when no such pick exists
    * In the browser, `--prompt` is a persistent heading with a native
      **Choose files** button. Choosing or dropping files opens them for
      review; `Enter` in the viewer confirms the answer

  * `gloss --skill` prints the skill that teaches agents to use gloss, so an
    installed binary can write out `SKILL.md` for the version it is;
    `gloss --skill --install` puts it where the agents on the machine look for
    skills, and `npx skills add NimbleTerminal/gloss` does the same with Node

  * **The viewer**
    * `B` opens a background color picker for meshes; the choice survives
      reloads and appears in previews. PNG exports still flatten onto white
    * The color picker supports mouse clicks and slider dragging. `Enter`
      chooses the swatch you moved to, and `Esc` undoes changes
    * `C` can recolor a whole model, including a 3MF with its own colors;
      `r` restores the file's colors, and `s` limits painting to plain faces
    * `r` starts a mesh turning immediately

  * **Shells and documentation**
    * Bash, zsh, and fish completions ship in the release archives, Debian
      packages, and Homebrew cask, alongside the generated man page
    * A documentation site provides a user guide and generated command
      reference; `--help` links to it. Developer instructions live in
      DEVELOP.md, and the README is a short introduction

  * **Bounds and reliability**
    * Tighter allocation and expansion limits for JSON, CSV, Excel, Word,
      HTML, Markdown, PDF, SVG, 3MF, and ZIP inputs; parsing and rendering
      deadlines keep malformed files from blocking the caller indefinitely
    * Page ranges are capped at 10,000 entries, part and column index lists
      at 65,536 entries, and GIF frames with no area are refused
    * Browser uploads have a session budget; Windows device and stream names
      are refused, and detached servers that fail to start are cleaned up
    * Opt-in fetching refuses redirects from HTTPS to HTTP
    * The browser site serves pinned, hash-checked PDFium assets itself,
      applies a Content-Security-Policy, and does not save documents opened
      through `?src=` in the file library
    * More loader fuzz tests, weekly fuzzing, and Windows compilation checks

## `v0.1.0` (2026-10-01)

The first release, to be tagged `v0.1.0`.  Happy Hacktober!

`gloss` is a visual pager for the terminal: like `less`, for images, SVGs, PDFs,
meshes, Markdown, and documents of many kinds.

  * **Formats**
    * Images: PNG, JPEG, GIF, WebP, BMP, TIFF, and HEIC (pure Go, no CGO)
    * SVG, rasterized by NTCharts; PDF, rendered by PDFium over WebAssembly
    * STL and 3MF meshes, up to 932,067 faces: flat-shaded, drawn on the GPU
      where there is one and in software where not; 3MF colors, parts, and
      slicer filament colors; `c` lists the parts, `C` picks a color
    * Markdown, with local pictures inline; HTML and Word, converted to Markdown
    * JSON, JSONL, and NDJSON, pretty-printed and highlighted; Jupyter notebooks,
      with their picture outputs
    * Plain text, and any file that is not binary, as `less` would show it;
      source is highlighted
    * Excel, CSV, TSV, and Grist documents, as scrollable sheets (`n`/`p` turn
      sheets or tables); columns can be hidden (`x`, `X`, `c`, `--cols`, `--coln`)
    * Every loader is bounded: 128 MiB of input, and limits on pixels, rows,
      and faces
  * **The viewer**
    * Kitty graphics where the terminal has them, colored half-block glyphs
      elsewhere; alternate screen by default, `-X` to stay on the main screen,
      `Q` to quit leaving the view in the scrollback
    * Zoom and pan, mesh orbit, and an ortho/perspective toggle
    * `i` describes the file and the terminal; `e` exports the current view as PNG
    * A file menu, a preview pane, thumbnails, and a file browser (`o`) that
      marks kinds, sorts, searches by glob, extension, or kind, and goes to a
      typed folder (`G`)
    * Drag and drop, and bracketed paste, of files and folders
    * `--fetch` lets `Enter` on a table cell holding an http(s) address download
      and open it; gloss never fetches on its own
    * Table cells with a web address are marked 🔗 and are links to the terminal
  * **For scripts and agents**
    * `--output`, `--output-dir`, and `--output -` export PNGs with no terminal;
      `--max-edge` and `--vision-profile` size them for vision models
    * `--view`, `--camera`, `--projection`, `--parts`, and `--color` pose meshes,
      including multi-view contact sheets
    * `--info [--json]` prints what the info box shows; `--text` and `--page`
      take the text out of documents and sheets, with page ranges
    * `--serve [--pick] [--prompt]` shows the viewer, or asks a human for a file,
      on a temporary localhost page
    * `--type` forces a format; `-` reads stdin
    * A skill for agents in `skills/gloss/SKILL.md`
  * **On the web**
    * A live demo, with a sample of every format, running the real pager as
      WebAssembly at <https://nimbleterminal.github.io/gloss/>
    * An app page for your own files, kept in the browser's storage and never
      uploaded; `?src=` opens a document from an address that allows it
  * **Install**
    * `brew install --cask nimbleterminal/tap/gloss`
    * `.deb` packages for amd64 and arm64
    * Archives for macOS, Linux, and Windows
    * `go install github.com/NimbleTerminal/gloss/cmd/gloss@latest`
  * MIT licensed; the licenses of linked modules are in `THIRD_PARTY_NOTICES.md`
