# Context menus on pictures, links and tables

Status: approved in conversation 2026-10-22, for planning.

## Why

Reading a Markdown document in gloss, the user right-clicked its first
picture, wanting to copy it into another app, and nothing happened. Things
on the page (a picture, a link, a table) can only be acted on through keys
that work on the whole file (`w`, `E`, `e`), or not at all. A right-click,
or a key on a highlighted item, should offer what can be done with that one
thing.

## Goals

- **Right-click** on a picture, link or table opens a small menu of actions
  for that item. A right-click on anything else does nothing: no menu, no
  note.
- **The keyboard reaches the same menus.** In a document, `Tab` /
  `Shift-Tab` move a visible highlight across the page's items (pictures,
  links, tables) in reading order; `a` ("actions") or `Shift-F10` opens the
  highlighted item's menu. This works the same in the normal text view and
  the typeset view.
- **`Tab` / `Shift-Tab` change meaning in documents**: they move between
  items instead of between files. `[` / `]` remain the keys for files
  everywhere. (Elsewhere, such as an image file, a PDF or the menu, Tab may
  keep moving between files.)
- **The menu**: a small box beside the item (or near the pointer for a
  right-click); arrows choose, Enter acts, Esc closes, a click on an entry
  acts, a click outside closes. It looks and behaves like the existing app
  chooser and its layers.
- **The guide is updated** with the new keys and menus (controls page and
  wherever Tab is documented), and the changelog's Unreleased section gets a
  short entry. `task docs` regenerates the generated pages.

## What each menu offers

**Pictures**: any image gloss shows. That covers images inside Markdown
(and the documents converted to Markdown: Word, HTML, notebooks), an image
file open in the viewer, a PDF page, a rendered SVG, Mermaid diagrams, and
GIF/WebP animations (the frame shown).

- **Open with…**: the image's own file, not the document containing it,
  through the existing open-with chooser (`internal/openwith`, ntopenwith).
- **Edit with…**: same, but **never for a temporary file**: edits to a
  temporary are lost (nothing writes them back, and gloss deletes it), so
  the entry is absent. Temporaries are: pictures with no file of their own
  (inside Word/notebooks, a PDF page, a rendered SVG/Mermaid diagram, a
  video frame), and gloss's own temporaries (stdin, downloaded addresses,
  `--qr`).
- **Copy image**: the picture on the operating system's clipboard, to paste
  into another app. **Native builds only**; absent in the browser demo/app
  and the `--serve` page. The picture copied is what is shown (the composited
  frame, the rendered page), as PNG.
- **Copy path**: the image file's path as text (OSC 52, as `y` copies
  today). Absent when the picture has no file of its own.

Open with on a picture with no file writes it to a temporary PNG first, so
it can be viewed in Preview or similar. That temporary lives as gloss's
other temporaries do.

**Videos**: the same menu, acting on the frame on screen: Copy image
(native), Open with… (a temporary PNG of the frame), and, for the video
file itself, Copy path. My call: also **Save frame as PNG** beside the
video, numbered rather than overwriting (gloss never overwrites), which is
what `e` already does; it may simply be listed as the `e` action.

**Links**: **Open** (the default browser, as a click does today) and
**Copy address**.

**Tables inside Markdown** (one Tab stop per table, not per cell):

- **Open table with…**: gloss's own table view first, then the apps that
  open CSV. Gloss's view opens the table as a sheet (as a CSV would be:
  cells, columns, search), one step deeper; Esc returns to the same place in
  the document. Other apps get a temporary CSV.
- **Copy as TSV** (pastes into spreadsheets) and **Copy as Markdown**.

**Spreadsheet cells** (CSV, Excel, Grist): the selected cell (clicks and
arrows already select one; a right-click selects the cell under it first):
**Copy value**, **Copy row** (TSV), and when the cell holds an address,
**Open address** (as Enter does with `--fetch`) and **Show QR** (as `u`).
Tab is not changed in sheets beyond what the document rule above implies;
the builder may leave it as file navigation there.

## Constraints

- Hosts: `Options.OpenWith` is nil in browser/serve hosts, previews and
  headless modes; entries that need it are absent there, not disabled.
  Copy image needs a native host. Text copies use OSC 52
  (`tea.SetClipboard`), as typeset selection does, so they work everywhere.
- The image clipboard: OSC 52 carries text only, so Copy image must reach the
  system clipboard directly. macOS builds use cgo, Linux/Windows do not, so a
  Linux/Windows implementation must be pure Go or call an external tool
  (e.g. `wl-copy`/`xclip`); when none is available the entry says why in a
  note rather than failing silently. Over SSH it copies on the remote
  machine; that is accepted.
- Mouse: right-click must not disturb existing left-click behaviour (link
  opening, footnote jumps, typeset selection, sheet clicks) or Shift-drag
  terminal selection. The text view already reports clicks
  (`MouseModeCellMotion`); typeset may use all-motion for link hover.
- Item geometry: in the text view the builder must know where each
  picture/link/table is on screen (Glamour-rendered rows); in typeset,
  nttypeset's layout already knows links (`TypesetViewport`/`Links`) and
  images. Whatever the method, the highlight and the right-click hit test
  must agree with what is drawn, in both glyph and Kitty modes.
- Tab moving to an item off screen scrolls it into view.
- Untrusted input: menu entries and notes show file names and addresses
  sanitized as gloss does today (`safe`).
- Never overwrite files; temporaries live in gloss's existing temp handling.
- Follow AGENTS.md: README stays short; the guide (`docs/hugo/content/guide`)
  owns keys; the skill is unaffected (nothing headless changes); `task ci`
  passes.

## Non-goals

- No per-cell Tab stops in Markdown tables; cells are reached through
  "Open table with… gloss".
- No menus for headings, code blocks, footnotes or plain text in this
  change.
- No editing of tables, and no filtering or sorting beyond what the sheet
  view already does.
- No menu in the file menu, file browser or previews (they have their own
  keys and the open-with chooser).
- No headless/CLI surface.

## Anti-goals

- Edit with on a temporary: losing someone's edits silently is worse than
  not offering it.
- A disabled, greyed-out entry where an action cannot work in this host:
  leave it out.
- A right-click that does something on empty space.
- Breaking existing clicks (links, footnotes, selection, sheets) or the
  terminal's Shift-drag.

## Decisions after review (approved with the design)

- **Where Tab moves between items**: rendered Markdown, in the text and
  typeset views, including documents shown as Markdown (Word, HTML,
  notebooks). There Tab always moves between items; with none, it does
  nothing but note "No pictures, links or tables here · [/] files". In every
  other view (plain text, source, JSON, Markdown source with `s`, images,
  PDFs, sheets, meshes, video) Tab keeps switching files, so its meaning
  depends on the kind of view, not on a file's contents.
- **Which links are items**: every link drawn as a link. HTTP(S) links offer
  Open (through `Options.OpenLink`, as a click does) and Copy address; other
  links (relative files, `mailto:`) offer Copy address only. Footnote
  references and back links are not items (they keep their click jumps). A
  picture that is also a link is one item, whose menu has the picture's
  entries followed by Open link / Copy address. With no `OpenLink` (browser,
  `--serve`), links offer Copy address only.
- **Sheet cells with an address**: **Open in gloss** only with `--fetch` (as
  Enter does), **Open in browser** when `OpenLink` exists, and **Show QR** for
  every address cell.
- **"Open table with… gloss"** reuses the fetched-document mechanism
  (`internal/app/fetch.go`): the table is written to a temporary CSV, added
  to the file list after the document as a fetched file is, and opened; the
  status bar shows "Esc close", Esc closes it, deletes the temporary, and
  returns to the document where it was (scroll position and the highlighted
  table). Other keys behave as they do on a fetched file. No new view stack.
- **Order**: the uncommitted glyph-geometry fix (`typesetCell`) and the
  select hint land first; this work builds on them.

## Left to the builder

- How items are found and hit-tested in each view, and how the highlight is
  drawn (outline, reverse video), provided it is visible in glyph and Kitty
  modes.
- The menu's exact placement and drawing, within the existing layer system.
- The image clipboard mechanism per OS, within the constraints above.
- Whether the work lands in one change or in stages (for example: menu and
  pictures first, then links and tables, then sheets).
