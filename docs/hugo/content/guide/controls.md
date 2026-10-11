---
title: "Controls"
weight: 20
bookToC: false
---

# Controls

| Key | Action |
| --- | --- |
| `q`, `Ctrl-C` | Quit |
| `q`, `Q` | Quit / quit leaving the last view in the scrollback, whatever `-X` said |
| `?`, `Esc` | Show help / dismiss help |
| `f` in typeset text | Preview font pairs: arrows select, Enter keeps, Esc restores; `+`/`-` still zoom |
| `T` over a document | Preview themes: arrows select, Enter keeps, Esc restores |
| `Esc` | Back to the file list, with previews, from a document |
| `t` | In the file list, show the files as a grid of thumbnails, and back |
| `m` | Open the file-selection menu |
| `o`, `O` | Browse folders for a file to open |
| `w`, `W`; Alt+Enter in the browser | Open in the default native app / choose an app; chooser `d` saves the default |
| `]`, `Tab` / `[`, `Shift-Tab` | Next / previous file |
| `n`, `Space`, `PageDown` / `p`, `b`, `PageUp` | Next / previous PDF page, sheet, or Grist table; next / previous file for other formats |
| `/` | Search text in the current document; `Enter` submits, `Esc` cancels editing |
| `n` / `N`, while searching | Next / previous result, wrapping at the ends; `Esc` clears the search |
| `t` on Markdown, text, source or JSON | Toggle normal text / typeset view |
| `/` in typeset mode | Open the text/source view and search |
| `s` in typeset mode | Toggle the typeset view and text/source |
| `#` | Show or hide file line numbers, in text and typeset views of plain text, source, JSON and Markdown source |
| `:` | Go to a line: type its number, `Enter` goes there, `Esc` cancels; a number past the end goes to the last line |
| `S` | Chop long lines at the right edge instead of wrapping them, as `less -S` does, and back; chopped, `←`/`→` and `h`/`l` scroll sideways and `Home` returns to the left edge |
| `Home` / `End`, `G` | First / last PDF page |
| Arrows, `h j k l` | Move about a spreadsheet by row and column |
| `x`, `X` | Hide the column under the cursor / show every column |
| `c` | List the columns, to show and hide them: `Space` toggles, `a` all, `n` none |
| `Enter` | With `--fetch`, open the web address under the cursor |
| `u` | Show a QR code for the selected table URL; `Esc` or `u` closes it, `g` switches graphics, `e` exports the QR |
| `Esc` | Close a fetched file and return to its cell |
| `+`, `-` | Zoom in / out |
| Arrows, `h j k l` | Pan zoomed images; orbit meshes |
| `f`, `0` | Fit image / reset camera |
| `g` | Toggle Kitty / glyph output when Kitty is supported |
| `R` | Reload file from disk |
| `e` | Export the current page as a PNG in the working directory (a download in a browser) |
| `i` | Show or hide a box of details about the current file and the terminal |
| `r` | Toggle mesh auto-rotation; reload other formats |
| `5` | Toggle mesh orthographic / perspective projection |
| `c` | List the parts of a 3MF: `Space` shows or hides one, `Enter` focuses on it, `a` all, `n` only it |
| `X` | Show every part of a 3MF again |
| `C` | Pick a color for a mesh: `Tab` between palette, sliders, and hex; `Space` applies the swatch, `Enter` chooses the swatch you have moved to and closes, `r` (the Reset button) restores the file's colors, `s` paints all faces or only plain ones, `Esc` undoes |
| `B` | Pick the background color behind a mesh, with the same picker as `C`: `r` restores the default, `Enter` chooses the swatch you have moved to and closes, `Esc` undoes |
| Click, in the `C` / `B` picker | Click a tab, swatch, or slider bar; a swatch applies at once and a second click on it chooses it and closes; drag along a slider |
| Drag / Shift-drag / wheel | Mesh (STL, 3MF) orbit / pan / zoom |
| Drag, then `y` | Typeset text: select, then copy to the clipboard; `Esc` clears |
| Drop files, or paste their paths | Add files to the list; one opens at once, several open the menu |

At a PDF boundary, page navigation stays on that page. Use `[` and `]` to change
files. Loading and rendering run asynchronously; errors appear in the viewer
with retry and next-file controls.

Native app handoff: `w` opens the current or highlighted file in its default app;
`W` opens the app chooser. In the file browser use Alt+Enter. In the chooser,
arrows select, Enter opens, `d` saves the default for that format, and Esc closes.
See [Open in another app](../using/#open-in-another-app).

## Line numbers, going to a line, and long lines

`#` puts the file's line numbers in a gutter, and `:` asks for a line to put
at the top, wherever the rows are the file's lines: plain text, source code,
JSON, and the source view (`s`) of Markdown and the other converted formats,
in the normal text view and the typeset one. A wrapped line is numbered on
its first row. Rendered Markdown prose and JSON records have no file lines,
so both keys say to use the source view instead. With the numbers known, the
status bar counts file lines rather than screen rows. `--line-numbers` (`-N`) starts
with the gutter on, in the viewer and in typeset exports.

`S` chops long lines instead of wrapping them, as `less -S` does: each line is
one row, cut at the right edge, and the arrows scroll the text sideways by a
quarter of the screen, with the status bar naming the column at the left edge.
A search match beyond the edge is scrolled into view. In the text views this
chops whatever is shown; a typeset page chops only text, source and JSON,
whose lines then pan as far as the widest one. `--chop-long-lines` (`-S`)
starts chopped.

## Document text search

Press `/`, type a literal query, and press `Enter`. Matching ignores case,
including Unicode case variants; punctuation has no regular-expression meaning.
`n` and `N` visit the next and previous result, wrapping at the ends. The first
result is at or after the current reading position when possible. `Esc` clears
the search before its usual back-to-list action, and restores `n`'s usual page
or file navigation. `/` edits the current query; canceling that edit keeps the
previous search, while submitting an empty query clears it.

- Markdown, text/source, JSON, notebooks, HTML, and Word: search logical text,
  including content outside the viewport. Phrases and long words can span
  wrapped screen rows; each fragment is highlighted. Resizing preserves the
  matches and selected occurrence. Paragraphs, explicit source/code line breaks,
  and Markdown table cells stay separate. Image pixels are excluded. `s` switches
  between rendered and source text and reruns the search.
- CSV/TSV, Excel, and Grist: search full cell values in visible columns of the
  current sheet or table, including off-screen cells. Results select the cell,
  highlight visible matches, and include an excerpt for truncated values.
  Hiding/showing columns or changing sheets reruns the query. Queries do not
  span cells. `PageDown`/`PageUp` retain their table scrolling behavior;
  clear search to restore `n` for changing sheets.
- PDF: search extractable text across pages, jump to matching pages, and show
  excerpts. Results count matching pages, not individual occurrences. Scanned
  pages without text are skipped and counted; there is no OCR or on-image
  highlighting. Extraction errors are reported with any partial results.
- Images, SVGs, GIFs, and meshes do not offer text search.

Text results count individual occurrences; grid results count matching cells.
The query is limited to 256 Unicode characters. Each search stops at 1,000
results, 16 MiB of searchable text, or a 30-second deadline checked between
logical text units, cells, and PDF page extractions. A running page extraction can finish
after cancellation; its obsolete result is ignored. The footer reports partial
results when a limit is reached. Search does not fetch content or change files.

The query editor supports arrows, Home/End (Ctrl-A/Ctrl-E), Backspace/Delete,
Ctrl-U to clear, and Ctrl-W to delete a word. Pasted text goes into the query;
it does not open a path or URL. The input scrolls to keep the cursor visible.
Opening the file menu/browser, changing files, or reloading clears the search.

Markdown table cells wrap independently, including headers. When the terminal
cannot fit even one character per column with separators, cells are stacked in
row order. This keeps their content available at narrow widths.

## File menu and drops

The file menu preserves argument order and marks the active file with `*`.
Use arrows or `j`/`k` to select, `Enter` to open, and `Esc` to cancel without
changing the current page, zoom, or camera. `PageUp`/`PageDown` scroll through
long lists; `Home`/`End` jump to the ends. Full paths distinguish duplicate names.
Press `v` to toggle an independent preview pane. PDF previews show page 1 at
reduced DPI; mesh previews are drawn as the viewer draws them, and 3MF previews show the
picture the file carries, when it has one: a slicer's rendering of the plate
if there is one, otherwise the declared thumbnail. The pane appears in terminals
at least 64 columns wide and 9 rows high. `--menu` starts in the selector;
`--preview` starts there with previews enabled. `t` in the list shows the
files as a grid of thumbnails instead: one picture, made from each file's own
as they are drawn in turn, with the file's kind standing in where it has
none. Arrows move about it, `Enter` opens, a click selects and a second click
opens, and `t` returns to the list. `Esc` from a document comes back to
whichever style was last used.

Drag files from a file manager onto the terminal to add them to the list.
Terminals deliver a drop as a bracketed paste of paths or `file://` URIs; text
that does not read as paths is ignored, except that `--fetch` also enables a
single pasted or dropped http(s) URL. Dropped files follow the same format and
size rules as arguments, and any that are skipped are reported on exit. A
folder dropped on its own opens the file browser there, as a folder named on
the command line does; one dropped beside files is skipped.
`gloss` alone opens the viewer with no file, as a drop target. In the
browser demo, URL drops are enabled and fetch directly in your browser (the
remote server must allow CORS). `--accept image/*` requires images, including
SVG; a mismatch reports the received and required format. In the demo, dropped files stay in the tab's memory until it reloads.

Press `o` to browse for a file instead, starting in the folder of the one you
are viewing. Type to filter the list, or type a path such as `~/Pictures/` to
go straight there; `Tab` completes, `Enter` opens a file or enters a folder,
and `Esc` cancels. The chosen file joins the list like a dropped one.
[The file browser](../browser/) describes its layouts, filter, and completion.
`gloss folder` starts in the browser at that folder. Beside files, folders are
skipped, so globs stay safe.

Files gloss cannot show are greyed and cannot be chosen; `Ctrl-T` hides them,
and again shows them. Known kinds are judged by extension; any other file is
looked into as the folder is listed, and offered when it reads as text, so
only binary files are greyed. `Ctrl-X` sets text files aside, for folders
where they are clutter, and offers them again. With `--type`, nothing is
greyed. Each file is marked by kind: 📁 folders, 📷 pictures, 🎨 SVG, 📕 PDF,
🧊 meshes, 📝 Markdown, HTML, and text, 📃 text by content, 🧾 JSON, 📓 notebooks, 📄 Word, 📊 tables.
`Ctrl-S` changes the order: by name, by date with the newest first, or by
kind; folders always come first, and the order is kept for the next browse.
`G`, with nothing typed in the filter, asks for a folder's path, starting from
the one shown: `Tab` completes folders (files are still listed, but not completed),
`~/` starts from home, `Enter` goes there,
and `Esc` comes back to the listing.

`/`, likewise, searches the folder shown and those under it. Ask for globs
(`*.png`, `report*`, `deep/*.pdf`), extensions (`png`), or kinds (`images`,
`svg`, `pdf`, `docs`, `word`, `meshes`, `tables`, `excel`, `grist`, `csv`,
`markdown`, `html`, `text`, `json`, `notebooks`), several at once; names are matched
without regard to case. `Enter` searches, then opens the file under the
cursor; `Ctrl-A` adds every match; `Esc` returns to the listing. A search is
kept safe by its limits: hidden folders and links are not entered, folders
deeper than 6 are not searched, at most 10,000 entries are looked at and 500
matches shown, and it stops after 3 seconds, saying which limit it met.
`--glob pattern` does the same from the command line for the folders named,
or the current one: `gloss --glob images --glob '*.stl' ~/models` opens every
match, and with `-o` or `--info` exports or describes them all.
The embedded demo, which has no folders, does not offer the browser.

Press `e` to export what you are viewing: the current image, SVG, or PDF page,
or a mesh from the camera's position. The PNG is rendered as `--output` would
render it, honors `--max-edge`, and is named after the file
(`report-page-3.png`). An existing file is never replaced; the name gains a
number instead. The browser demo, and a page shown with `--serve`, offer the
PNG as a download instead: what is saved goes where the browser saves, not
where gloss runs.

Standalone animated GIFs play automatically. **Space** pauses or resumes; after a finite
animation finishes, Space starts it again. **`<` / `>`** slow down or speed up
through **0.25×, 0.5×, 1×, 2×, and 4×**; **Backspace** restores 1×. The status
bar shows playback speed separately from zoom. Changing speed preserves pause
and restarts the current frame's delay; it does not skip frames or restart a
finished animation. Space restart retains speed. `r` or `R` reloads from the
start at 1×; opening another GIF also starts at 1×.

The bottom row is a **frame scrubber**: click or drag its bar to seek. **`,` / `.`**
step backward/forward one frame; **Home / End** select the first/last frame.
Seeking pauses playback and clamps at the ends. **Space** resumes from the
selected frame with a fresh loop count, retaining playback speed and zoom.
The bar represents frame positions, not elapsed time (GIF frame delays vary).
Rapid dragging keeps only the latest requested position; the old picture stays
visible while the requested frame is composed. The status bar reports seeking;
wait for it to finish before exporting that frame. The bar is hidden behind
help and overlays; arrow keys still pan the image.

`n`/`p` and `[`/`]` still switch files. Zoom and pan apply to every frame, and
`e` saves the displayed full frame as PNG. Help and the file browser suspend
playback; list previews remain still. Embedded Markdown GIFs play while visible,
without standalone playback controls; `e` exports the first visible GIF’s current
frame in normal Markdown or the whole viewport in typeset mode. Large/slow frames play
more slowly rather than building a rendering backlog.

In native Markdown views, click an HTTP(S) link or linked image to open it in
the default browser. This also works in typeset mode; wheel scrolling remains
available. Dragging, source view, and overlays do not activate links.

Text screens take the mouse for the wheel and for clicks, so select text on
them with your terminal's override, usually Shift held while dragging. A
typeset page is a picture, which the terminal cannot select from: drag over
it to select text, `y` copies the selection through the terminal's clipboard
(OSC 52, which the terminal must allow; most do), and `Esc` clears it. The
selection follows the text through scrolling, zooming and resizing. Visual
wrapping adds no spaces to copied URLs or identifiers. A
plain click or another drag replaces it. Shift-drag is still the terminal's.

In a table, select a cell containing an http(s) URL and press `u` to show its QR
code. This does not fetch the URL and needs no `--fetch`. The original cell
stays unchanged; the overlay shows a single address footer. `Esc` or `u`
returns to the table. While the QR is open, `e` saves it as `qr.png` (numbering
the name if it exists), or offers a download in the browser. This export uses
eight pixels per module and includes the white border.

QR codes use black on white, at least Medium error correction (approximately
15% recovery), and a four-module quiet zone. Correction increases when a
stronger level fits without making the code larger. Text segments are optimized
for size, with explicit UTF-8 encoding for non-ASCII text.
The compact overlay keeps a single URL footer (ellipsized if needed);
the full text remains in the table, and the status bar shows the available keys.
They use Kitty graphics when available, with a direct Unicode half-block
fallback. If the whole code cannot fit, gloss reports that instead of cropping
it; enlarge the terminal or export the PNG. Glyph rendering assumes a 1:2 cell
ratio until the terminal reports its size. It allows slightly rectangular
modules (up to 12.5% side-length difference) to fit ordinary font proportions;
Kitty and PNG modules are exactly square. Unusual fonts may still need more
space. Camera readability still depends on the
terminal, font, display size, and camera. A QR code does not make a local or
private URL reachable from another device; it is not a phone-upload handoff.

Try `gloss examples/qr-links.csv` from the repository, then Down and `u`.

Press `i` for what a file says about itself. Every format shows its path, size,
and modification time, followed by:

| Format | Details |
| --- | --- |
| Images | Format, pixel dimensions, color model; from Exif in JPEG, TIFF, and HEIC: camera, lens, date taken, exposure, orientation, location, software, artist, copyright |
| SVG | Declared size, view box, title, description, element count |
| PDF | Version, page count, size of the current page, title, author, subject, keywords, creator, producer, dates |
| STL | Encoding, name, triangle count, extent, surface area (STL records no unit) |
| 3MF | Title, designer, description, application, dates, license, object and triangle counts, the parts by name, extent and surface area in the file's unit, thumbnail size |
| Markdown | Title, lines, words, headings, links, images |
| Word | Title, author, dates, application, pages and words as Word counts them, headings, tables, images, links |
| Excel | Sheets with their size, title, author, dates, application |
| Grist | Tables with their size, time zone, schema version, what is not shown |
| CSV | Separator, rows, columns |
| JSON | Whether it is one value or lines of records, how many, the top-level kind and first keys |
| Notebook | Format version, language, kernel, cells by kind, outputs, pictures |
| HTML | Title, and the lines, words, headings, links, and images of the text |
| Text | Lines, words, characters |

Only fields present in the file are listed. The box sits in the top-right
corner over the document, which stays in use beneath it: the details follow as
you turn pages or change files. `i` or `Esc` closes it.

The box ends with a `Terminal` section: the screen size in cells and, when the
terminal has said, in pixels; whether pictures are drawn as glyphs or Kitty
graphics and whether Kitty graphics were found to be supported; how meshes are
rendered; the terminal program and multiplexer; and which screen is in use.
With no file open, `i` shows that section alone, so `gloss` then `i` says what
the terminal can do.


### Text colors

`--theme auto|light|dark|NAME|FILE.json` selects normal text and typeset colors. Auto follows
the terminal background, with a dark fallback when no reply arrives, and
rechecks on terminal focus. Headless exports use light for auto. Explicit themes
override detection. See [Themes](../markdown/#themes) for details.
