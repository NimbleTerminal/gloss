---
title: "Markdown"
weight: 40
bookToC: false
---

# Markdown

Open `.md`, `.markdown`, or `.mdown` files, or pipe text with `--type markdown`.
Glamour renders headings, lists, tables, and syntax-highlighted code. Local
Markdown image references (including reference-style links) use the existing
raster and SVG renderers, shown as block figures following their text line.
Paths resolve relative to the Markdown file, or the working directory for stdin.
Missing and remote images show placeholders; HTML image tags are not rendered.

```sh
gloss examples/readme.md
gloss --preview examples/readme.md examples/shapes.svg
cat README.md | gloss --type markdown -
```

Use `j`/`k`, arrows, or the mouse wheel to scroll; `Space`/`b` page down/up;
`Ctrl-D`/`Ctrl-U` move half a page; `Home`/`End` jump to the ends. Press `s` to
switch between rendered Markdown and source. File navigation remains `[`/`]`.
In the native terminal viewer, click an HTTP(S) link or linked picture to open
it in your default browser, in normal or typeset Markdown. Wheel scrolling
still works. Opening a link does not download it into gloss and does not require
`--fetch`. Dragging does not activate links. Relative file links, heading anchors,
and other URL schemes are not handled by this click action.

Normal Markdown also retains OSC 8 links for the terminal's own opener. With
mouse reporting enabled, your terminal may require a modifier: Kitty supports
Shift-click or Ctrl-Shift-click. The browser demo and `--serve` do not launch a
browser on the server machine; normal-view links there use the hosting
terminal's link handling. Typeset browser-host link activation is not implemented.

Documents are limited to 2 MiB of UTF-8 and 32 image references, with a combined
16-megapixel decoded image budget after resizing. Embedded images fit within
1600 pixels, with at most 128 MiB of combined encoded image input. The default
text mode does not export a whole Markdown page as PNG. When a GIF is visible,
`e` exports the first visible GIF’s displayed full frame.

## Embedded GIFs

Local GIF figures play automatically in both views, honoring frame delays,
transparency, disposal and loop counts. Only visible GIFs run. Scrolling them
offscreen, opening help or the file menu, suspending, or switching to source
pauses playback; returning resumes at the saved position. `r`/`R` reloads and
restarts. The standalone GIF pause, speed and scrubber controls do not apply:
Space still scrolls Markdown. Typeset scrolling and reflow preserve playback
positions, and `e` exports the displayed viewport, including its GIF frames.
With Kitty graphics a typeset GIF is redrawn in its own area only, so the rest
of the page is not resent for every frame, and it stands still during a scroll
or resize burst, resuming once the view settles.

A document can animate up to eight distinct GIF destinations with a combined
64 MiB of decoded frame storage, checked before full decoding. Duplicate
references share decoded frames. GIFs beyond either animation budget keep their
first frame; `i` reports the animated and over-budget counts. Each GIF also has
the usual 32 MP canvas and 1,000-frame limits. Previews and headless exports
always use the first frame. Animated WebP and APNG remain unsupported.

## Typeset text and Markdown

Press `t` to switch between Glamour text and typeset Markdown, including from
source view. Switching starts at the top and clears search; the choice applies
to subsequent Markdown, plain text, source and JSON files in the viewer. `s` still toggles source.
`gloss view --typeset notes.md` chooses the initial mode (`view` is
optional). `--markdown typeset` remains a compatibility alias. Glamour remains the default. `--type markdown` and stdin work too;
converted Word, HTML, and notebooks keep their existing text renderer.

Plain text also supports `--typeset` and `t`. It uses a proportional font,
preserves line breaks and blank lines, and wraps long lines. Markdown punctuation
stays literal. Source files use monospace with Chroma highlighting; JSON uses
the existing pretty-printed representation. Tabs display as four spaces.
`s` and `/` open the normal text view without generated Markdown fences.
Converted Word, HTML and notebooks are not included in this mode.

```sh
gloss --typeset notes.txt
gloss --typeset main.go
gloss --typeset data.json
cat notes.txt | gloss --type text --typeset -
```

Text and JSON keep their original `kind` in export manifests. Typeset text
requires valid UTF-8, at most 2 MiB of source and formatted text, and at most
8,192 lines; the existing bounded source view must also fit without truncation.
The per-line shaping, raster and total layout limits below also apply.

Typeset Markdown uses Goldmark, go-text shaping and rasterization, repo-embedded
Go fonts, and Chroma's GitHub light/dark syntax colors. No PDF is generated and PDFium is
not involved. The existing picture component displays one viewport image using
Kitty graphics or colored half-blocks. No terminal text/image layering is needed.
In Kitty mode, scrolling and page changes keep the previous image visible until
the next image is ready, without briefly switching to half-blocks.

The reading width follows the terminal's cell-pixel geometry, bounded to
192–960 points at a 150-DPI reading scale. With Kitty graphics the view is
drawn at the terminal's own pixel size, so high-density and wide terminals get
sharper text rather than an enlarged picture. Width changes reflow after a 60 ms
pause; the previous view stays visible while a cancellable job prepares its
replacement. Reflow anchors the current block where possible. Height changes
and scrolling reuse the layout and decoded images. Preparation validates all
source and assets; layout then advances through blocks as needed for the
viewport. A large paragraph or table still needs to finish before it appears.
Reload with `R` after editing the source or its pictures.

| Key | Typeset mode |
| --- | --- |
| `t` | Switch to normal terminal text |
| `s` | Toggle the typeset view and plain Markdown source |
| Up / Down, wheel | Scroll vertically |
| Space / `b`, PageDown / PageUp, `n` / `p` | Scroll one screen |
| `+` / `-`, Left / Right | Zoom / pan horizontally |
| Home, `f`, `0` | Fit width and return to the top |
| End / `G` | Lay out the remaining document and go to the bottom |
| `/` | Open source view and search Markdown text |
| `e` | Export the visible rendered viewport as PNG |

Search currently uses the source viewer's matches and highlights; it does not
highlight text on the typeset image. `s` returns from source to the typeset view.

```sh
gloss --typeset examples/typeset.md
gloss --typeset --page 2 --dpi 200 notes.md
gloss --typeset --page all --output-dir pages notes.md
cat notes.md | gloss --type markdown --typeset --output page.png -
```

Headless exports and file-list previews use a fixed 800-pixel reading width at
150 DPI. `--page` selects numbered 1125-pixel-high slices of the continuous
layout, not paper pages: a slice boundary can cut a line or image. `--page all`
exports every slice, so headless export computes the complete layout first.
`--dpi` changes raster detail without changing line wrapping; `--max-edge`
controls the exported PNG dimensions. In the viewer `--page 2` starts at the
second slice's position. Interactive `e` saves the currently displayed viewport,
including zoom. `--output` produces PNG, never PDF; `--text` still returns the
original Markdown. Export results use `kind: "markdown"`.

Local images resolve relative to the Markdown file, or cwd for stdin. Missing,
unsupported and remote images become placeholders; no images or fonts are
fetched. An image alone in a paragraph, including one wrapped in a link, becomes an image
block and is composited into the page. Embedded GIF blocks animate; inline images
currently appear as labels. Tables have simple proportional columns, without
alignment-aware or full browser table layout. GFM strikethrough and task markers
are supported. Raw HTML, Mermaid and math have no layout. Unknown or untagged
code blocks stay plain monospace. Complex-script layout, CJK/emoji font coverage,
and bidirectional text are not guaranteed.

Before decoding images: source must be UTF-8 and at most 2 MiB, with at most
32 image references (including duplicates), 16 Mi decoded pixels before resizing,
and 128 MiB combined encoded image input. SVGs reserve 1600×1600 pixels each;
duplicate destinations reuse the same decoded image. Further bounds are 32,768
AST nodes/blocks, 16,384 runes per text paragraph or code line, 8,192 lines per
code block, 262,144 laid-out glyphs, 32,768 drawing operations, and 10,000 export
slices. The shaping cache has a 32 MiB estimated-entry budget. Layout jobs have a
15-second deadline, and accumulated layout work is also capped at 15 seconds.
Each raster is limited to 32 Mi pixels. Exceeding a limit reports an error;
the viewer keeps the previous image if a reflow or scroll fails.


## Themes

`--theme auto|light|dark` applies to normal terminal text and typeset Markdown,
plain text, source and JSON. The two palettes coordinate page backgrounds, body
text, headings, quotes, borders, table headers, code blocks and syntax colors.

Auto is the default. In the viewer, gloss asks the terminal for its background
color asynchronously and chooses a light or dark palette. It starts with dark
and keeps that fallback if the terminal does not answer. It uses terminal
colors, not the operating system's appearance setting. Auto rechecks on focus
when the terminal supports focus reporting; subsequent background-color replies
also update the view. Use an explicit theme when detection is unavailable,
including through a multiplexer that does not forward the reply.

Headless exports never probe the terminal: auto uses light, and
`--theme dark` explicitly exports a dark page. `--text` extraction is unchanged.

```sh
gloss --typeset --theme dark examples/typeset.md
gloss --theme light main.go
gloss --typeset --theme dark --output code.png main.go
```

Theme changes preserve the typeset scroll position, fonts, shaped text, layout
and decoded images; only the viewport is repainted. Syntax uses the same font
styles in both palettes to keep line wrapping stable. The previous Kitty image
stays visible until the replacement is ready. Embedded images and existing PDF
pages are never recolored. Transparent images naturally show the page color
behind them. QR codes retain opaque black modules on white. Custom palettes
and OS appearance integration are not part of this setting.
