# Context Menus Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A right-click, or `a`/Shift-F10 on an item highlighted with Tab, opens a small menu of actions for one picture, video frame, link, Markdown table or spreadsheet cell.

**Architecture:** The menu is one new document layer (`layerActions`). It is drawn as a box with `overlay`, and its entries are built per item by plain functions, so they can be tested without a terminal. Items come from what is already drawn:
- In the text view, from `markdownView.lines`: picture rows, OSC 8 links, and table rows tagged at layout.
- In the typeset view, from a new `Result.TypesetItems` that the loader fills from nttypeset's `Links`, `Images` and `Starts`.
- For a whole-view picture, a video or a sheet, the item is the view itself or the cell under the cursor.

Host abilities arrive on `Options` as they do for `OpenLink`, and are nil where they cannot work:
- `CopyImage`;
- `Scratch`, which holds temporary PNGs and CSVs.

**Tech Stack:** Go 1.26, Bubble Tea v2 (`charm.land/bubbletea/v2`), nttypeset v0.1.1, ntopenwith through `internal/openwith`, and OS clipboard tools through `os/exec`. No new module dependencies.

**Spec:** `docs/superpowers/specs/2026-10-22-context-menus-design.md`

## Global Constraints

- Before Task 1, the uncommitted glyph-geometry fix (`typesetCell`) and the select hint (`selectHint`) must be committed. The executor asks the user to commit them and does not commit them itself.
- An entry that cannot work in this host is left out, never disabled or greyed. `Options.OpenWith`, `Options.OpenLink`, `Options.CopyImage`, `Options.Scratch` and `Options.Fetch` are nil in some hosts. The browser, `--serve`, previews and headless modes set none of OpenWith, CopyImage or Scratch.
- Edit with is never offered for a temporary: a picture with no file of its own, or a path for which `m.opts.IsStdin(path)` or `m.opts.IsTemporary(path)` holds.
- Text is copied with `tea.SetClipboard` (OSC 52). Pictures are copied only through `Options.CopyImage`, as PNG of what is shown.
- A right-click on nothing does nothing: no menu and no note. The existing left-click behaviour must not change: link opening, footnote jumps, typeset drag selection, sheet clicks, the scrubber and Shift/Alt-drag.
- Every file name, address and label shown in a menu or note goes through `safe`.
- Nothing overwrites a file. Temporaries go in the Scratch folder, which is removed at exit. Saved frames go through `saveFile` and `document.WriteNew`.
- Use the menu labels below exactly, with the ellipsis character `…`:
  - Pictures: `Open with…`, `Edit with…`, `Copy image`, `Copy path`.
  - Videos: those, plus `Save frame as PNG`.
  - Links: `Open`, `Copy address`. A picture that is also a link adds `Open link` and `Copy address`.
  - Tables: `Open table with…`, or `Open table in gloss` when OpenWith is nil; `Copy as TSV`; `Copy as Markdown`.
  - Sheet cells: `Copy value`, `Copy row`, `Open in gloss`, `Open in browser`, `Show QR`.
- The note when there are no items is exactly `No pictures, links or tables here · [/] files`.
- Follow AGENTS.md:
  - gofmt;
  - comments in the surrounding style;
  - the guide owns keys;
  - the README is unchanged;
  - the skill is unchanged, since nothing headless changes.
- Run `GOWORK=off task ci` before the work is done. If staticcheck fails on Go 1.27 export data, rerun it with `GOTOOLCHAIN=go1.26.9`.
- Never push. Commit per task on a feature branch, not on main.

## Review Focus

1. **A right-click while a layer is open, during loading, or in help.** Nothing happens behind the layer: the click closes the menu, or is ignored as left-clicks are there. *Test: Task 2, `TestActionMenuIgnoresClicksUnderLayers`.*
2. **A narrow window (under 24 columns or under 6 rows) or a menu near the right or bottom edge.** The box moves inside the screen or is not drawn. It never wraps or overruns the body. *Test: Task 2, `TestActionMenuStaysOnScreen`.*
3. **A reflow or file change while an item is highlighted** (resize, `S`, `#`, theme change, reload, typeset↔text, the next file). The highlight is dropped or re-found, never left pointing at a stale row or rectangle. *Test: Task 4, `TestFocusDroppedOnReflow`.*
4. **A link that wraps across two rows, and a picture that is also a link.** The wrapped link is one Tab stop. The linked picture is one Tab stop whose menu has both sets of entries. *Test: Task 4, `TestItemsMergeWrappedLinkAndLinkedPicture`.*
5. **Copy image with no clipboard tool** (Linux without wl-copy or xclip), and a hostile file name in a menu title. The note says why, with nothing written to the terminal raw. *Test: Task 1, `TestCopyImageMissingTool`; Task 3, `TestActionMenuTitleIsSafe`.*

---

### Task 1: Host abilities: the image clipboard and the scratch folder

**Files:**
- Create: `internal/clipimage/clipimage.go` (shared doc and `ErrNoTool`)
- Create: `internal/clipimage/clipimage_darwin.go`
- Create: `internal/clipimage/clipimage_windows.go`
- Create: `internal/clipimage/clipimage_other.go` (`//go:build !darwin && !windows && !js`)
- Create: `internal/clipimage/clipimage_js.go`
- Create: `internal/clipimage/clipimage_test.go`
- Create: `cmd/gloss/scratch.go` and `cmd/gloss/scratch_test.go`
- Modify: `internal/app/model.go` (Options), `cmd/gloss/main.go:613-633` (wiring), `Taskfile.yml` sources if `taskcheck` asks

**Interfaces:**
- Produces:
  - `clipimage.Copy(ctx context.Context, png []byte) error`
  - `clipimage.ErrNoTool` (an error whose text names what to install)
  - `app.Options.CopyImage func(png []byte) error`, documented as "nil where pictures cannot reach the system clipboard"
  - `app.Options.Scratch func(name string, data []byte) (string, error)`, which writes a new file in a session folder and returns its path

- [ ] **Step 1: Write the failing tests.**
  - `TestCopyImageCommand` (non-js): with a fake `lookPath` and `run` (package variables), the Copy for this GOOS runs:
    - macOS: `osascript`, with a script that reads the temp file `as «class PNGf»`;
    - Linux and BSD: `wl-copy --type image/png` when `WAYLAND_DISPLAY` is set, else `xclip -selection clipboard -t image/png -i FILE`;
    - Windows: `powershell -NoProfile -STA -Command …[Windows.Forms.Clipboard]::SetImage(...)`.
  - `TestCopyImageMissingTool` (non-darwin, non-windows): with neither tool found, `errors.Is(err, ErrNoTool)`, and the error text contains `wl-copy or xclip`.
  - `TestScratchNeverOverwrites` in `cmd/gloss`: two writes of `table.csv` return different paths, both inside the folder. `holds` is true for both, and `remove` deletes the folder.
- [ ] **Step 2: Run** `GOWORK=off go test ./internal/clipimage ./cmd/gloss -run 'CopyImage|Scratch'`. Expected: FAIL, undefined.
- [ ] **Step 3: Implement.**
  - **clipimage:** write `png` to a `os.CreateTemp` file, run the tool, then remove the file. The js build returns `ErrNoTool`.
  - **Scratch folder:** `cmd/gloss/scratch.go` is a `scratchFolder` shaped like `fetchFolder` (lazy `os.MkdirTemp("", "gloss-scratch-")`, `holds`). Its `write(name, data)` goes through `document.WriteNew`.
  - **Wiring in `main.go`:** next to `opts.OpenLink = browse`:
    - set `opts.CopyImage` to call `clipimage.Copy(context.Background(), png)`;
    - set `opts.Scratch` to the folder's `write`;
    - add `|| scratch.holds(path)` to `IsTemporary`;
    - `defer` removing the folder.
  - **Elsewhere:** `--serve` and the headless paths leave both nil, because they return before this point.
- [ ] **Step 4: Run** the same command, plus `GOWORK=off GOOS=windows go vet ./internal/clipimage` and `GOOS=js GOARCH=wasm go vet ./internal/clipimage`. Expected: PASS and clean.
- [ ] **Step 5: Commit** with the message `feat: image clipboard and scratch folder for the viewer's own temporaries`.

### Task 2: The action menu layer

**Files:**
- Create: `internal/app/actions.go`, `internal/app/actions_test.go`
- Modify: `internal/app/screen.go:18-30` (add `layerActions`), `internal/app/info.go:131` (`overlayAt`), `internal/app/view.go` (`layered`, `statusLine`), `internal/app/keys.go` (`layerKey`), `internal/app/model.go` (mouse routing, field `actions *actionMenu`)

**Interfaces:**
- Produces:
  ```go
  type menuEntry struct {
  	label string
  	do    func(m *Model) tea.Cmd
  }
  type actionMenu struct {
  	title   string
  	entries []menuEntry
  	cursor  int
  	at      image.Point // The cell the box is placed beside: an item's corner or the pointer.
  	box     image.Rectangle // Where it was last drawn, for clicks.
  }
  func (m *Model) openActions(title string, entries []menuEntry, at image.Point) tea.Cmd // Nil entries: no menu, no note.
  func (m *Model) actionsView(w, h int) string
  func (m *Model) actionsKey(k string) (tea.Cmd, bool)
  func (m *Model) actionsMouse(msg tea.Msg) (tea.Cmd, bool)
  func overlayAt(body, box string, w, x, top int) string // overlay places at x = w - width.
  ```
  Running an entry closes the menu first, then calls `do`.

- [ ] **Step 1: Write the failing tests** in `actions_test.go`. Use `loaded(t, "photos/landscape.png")` and call `openActions` with two fake entries.
  - `TestActionMenuKeys`:
    - `down` then `enter` runs the second entry once and closes (`m.layer == layerNone`);
    - `esc` closes without running anything;
    - `up` at the top stays at 0.
  - `TestActionMenuMouse`:
    - a `tea.MouseClickMsg{Button: tea.MouseLeft}` on the second entry's row inside `box` runs it;
    - a click outside `box` closes without running.
  - `TestActionMenuStaysOnScreen`:
    - `at` = (w-1, h-1) draws the whole box within the body, so every entry label appears in `rows(m)`;
    - at 20×5 the body shows no box and the bar note says `enlarge terminal`.
  - `TestActionMenuIgnoresClicksUnderLayers`: with the info layer open, a right-click does not open a menu. With help open, the same holds.
  - `TestActionMenuStatus`: the keys line reads ` ↑/↓ choose · Enter do · Esc close`.
- [ ] **Step 2: Run** `go test ./internal/app -run ActionMenu`. Expected: FAIL, undefined.
- [ ] **Step 3: Implement.**
  - **Box:** drawn like `infoBox`, with `m.boxText()` and a border. The title is `safe` and truncated with `…`. Entries are marked `› ` like the chooser, and the box is at most 40 columns wide.
  - **Placement:** right of and below `at`, flipped left or up to fit.
  - **Overlay:** `overlay` becomes `overlayAt(body, box, w, w-width, top)`.
  - **Routing:**
    - keys go through `layerKey`'s `layerActions` case;
    - mouse messages go through `actionsMouse`, called at the top of the document mouse handling in `model.go` before the sheet and link handlers when `m.layer == layerActions`;
    - the mouse mode stays `CellMotion` while the menu is open.
  - **Closing:** `closeLayer` clears `m.actions`.
- [ ] **Step 4: Run** `go test ./internal/app`. Expected: PASS.
- [ ] **Step 5: Commit** with the message `feat: an action menu layer over the document`.

### Task 3: Menus for a picture shown alone, and for a video frame

This covers an image file, an SVG, a GIF or WebP, a PDF page, a Mermaid file and a video.

**Files:**
- Create: `internal/app/picture_actions.go`, `internal/app/picture_actions_test.go`
- Modify: `internal/app/keys.go` (`a` and `shift+f10` at document level, when `m.layer == layerNone`), `internal/app/model.go` (right-click), `internal/app/help.go`, `internal/app/view.go` (keys line gains ` · a actions` for these views)

**Interfaces:**
- Consumes: `openActions`, `menuEntry` (Task 2); `Options.CopyImage`, `Options.Scratch` (Task 1); `requestOpenWith(path, kind, false, true, verb)`.
- Produces:
  ```go
  // pictureItem is a picture the reader can act on: its own file, if it has
  // one, and how to get what is on screen.
  type pictureItem struct {
  	name  string             // For titles and temporary names: the file's base, or the alt text.
  	path  string             // Its own file; empty when it has none.
  	shown func() image.Image // What is on screen now; called when an entry runs.
  	link  string             // The link around it, if any.
  	video bool               // Offer Save frame as PNG.
  }
  func (m *Model) pictureEntries(p pictureItem) []menuEntry
  func (m *Model) copyImage(shown image.Image) tea.Cmd   // PNG-encodes in the Cmd; notes "Image copied" or the error.
  func (m *Model) openShown(p pictureItem) tea.Cmd       // Writes <name>.png through Scratch, then requestOpenWith(..., true, openwith.Open).
  func (m *Model) viewPicture() (pictureItem, bool)      // The whole-view item for the file on screen.
  ```
- Entry rules, in this order:
  - **`Open with…`:** offered when OpenWith is set. It uses `path` when there is one; otherwise it uses `openShown`, which also needs Scratch.
  - **`Edit with…`:** offered when OpenWith is set, `path` is not empty, and the path is not temporary.
  - **`Copy image`:** offered when CopyImage is set.
  - **`Copy path`:** offered when `path` is not empty.
  - **`Save frame as PNG`:** offered when `video` is set. It runs `m.export()`.
  - **Linked pictures:** when `link` is set, the link's entries follow (Task 4's `linkEntries`; in this task, an empty hook).
- What `viewPicture` returns:
  - **path:** the file's own path for image, SVG, GIF/WebP, Mermaid and video files. It is empty for a PDF page.
  - **shown:** `m.animation.Status().Presented.Image()` when animating, else `m.source`. For a video, it is the frame `export()` would draw.
  - **not ok:** for meshes, sheets, text and Markdown.
- Right-click (`tea.MouseClickMsg{Button: tea.MouseRight}`) on the picture's drawn area opens the menu at the pointer. The drawn area is the body for still pictures. For a video it excludes the scrubber row. Anywhere else, the right-click does nothing.

- [ ] **Step 1: Write the failing tests.**
  - `TestPictureEntriesByHost` (table-driven over hosts):
    - all set, with a file → `Open with…, Edit with…, Copy image, Copy path`;
    - OpenWith nil → `Copy image, Copy path`;
    - CopyImage nil, with a temporary path → `Open with…, Copy path`;
    - no file, Scratch nil, CopyImage nil → no entries, so no menu opens.
  - `TestPictureMenuFromKey`:
    - `loaded(t, "photos/landscape.png")` with fake hosts;
    - `press("a")` opens a menu titled `landscape.png`;
    - choosing `Copy image` calls CopyImage with bytes that `png.Decode` reads at 64×40.
  - `TestShiftF10AndRightClick`:
    - `tea.KeyPressMsg{Code: tea.KeyF10, Mod: tea.ModShift}` opens the same menu;
    - a right-click inside the picture opens it at the pointer;
    - a left-click opens nothing.
  - `TestPDFPageOpensATemporary`:
    - a PDF result (Kind "pdf") has no `Edit with…` and no `Copy path`;
    - `Open with…` writes `<base>-page-1.png` through Scratch;
    - then fakeOpenHost sees a target with `Temporary: true`.
  - `TestActionMenuTitleIsSafe`: a file named `a\x1b]2;x\a.png` shows no ESC byte in `m.View().Content` outside SGR.
  - `TestVideoMenuHasSaveFrame`: `pictureEntries(pictureItem{video: true, path: "v.mp4", ...})` ends with `Save frame as PNG`.
- [ ] **Step 2: Run** `go test ./internal/app -run 'Picture|ShiftF10|PDFPage|TitleIsSafe|VideoMenu'`. Expected: FAIL.
- [ ] **Step 3: Implement** the code in `picture_actions.go`, then add the key, mouse, help and status changes.
  - **Help line:** `a / Shift-F10   Actions for the picture, link, table or cell`.
  - **Temporary flag:** `requestOpenWith` already sets `Temporary` from `IsTemporary`, which now includes Scratch.
- [ ] **Step 4: Run** `go test ./internal/app`. Expected: PASS.
- [ ] **Step 5: Commit** with the message `feat: right-click or a for a picture's actions: open, edit, copy image, copy path`.

### Task 4: Items in the Markdown text view: Tab, the highlight, and link and picture menus

**Files:**
- Create: `internal/app/items.go`, `internal/app/items_test.go`
- Modify:
  - `internal/document/markdown.go:78-84,160-170`: `MarkdownImage.Path string`, documented as "The file it was read from, when it is one beside the document; empty for packaged, embedded or drawn pictures." Set it from `resolved` only when `files == nil`, and clear it in the packaged loaders (Word, notebooks) where `Packaged` is set.
  - `internal/app/markdown.go`: `markdownLine.table int` (the table the row belongs to, counted from 1; 0 for none), set in `layout` for `block.table` rows; `markdownView.focus *pageItem`; and the drawing of the highlight in `view()`.
  - `internal/app/keys.go:265-268`: Tab and Shift-Tab.
  - `internal/app/links.go`: `linkEntries`.
  - `internal/app/view.go`: the keys line.
  - `internal/app/help.go`.

**Interfaces:**
- Consumes: `pictureItem`, `pictureEntries` (Task 3); `openActions` (Task 2).
- Produces:
  ```go
  type itemKind int
  const (
  	itemPicture itemKind = iota
  	itemLink
  	itemTable
  )
  // pageItem is a thing on a rendered Markdown page: where it is drawn, in
  // rows of the text view or rectangles of the typeset page, and what it is.
  type pageItem struct {
  	kind    itemKind
  	picture pictureItem      // itemPicture
  	url     string           // itemLink
  	cells   [][]string       // itemTable: plain text, header row first
  	rows    [2]int           // Text view: the first and last line of m.markdown.lines.
  	cols    [2]int           // Text view, links: the cells of the first row it covers.
  	rects   []image.Rectangle // Typeset: page layout pixels.
  }
  func (v *markdownView) items() []pageItem            // In reading order; nil for raw (source) and fenced views.
  func (m *Model) itemsHere() bool                     // Rendered Markdown in the text or typeset view, on the document screen.
  func (m *Model) moveFocus(step int) tea.Cmd          // Tab +1, Shift-Tab -1, wrapping; scrolls the item into view.
  func (m *Model) itemEntries(it pageItem) []menuEntry
  func linkEntries(m *Model, url string, label string) []menuEntry // label "Open" or "Open link"
  func tableTSV(cells [][]string) string
  func tableMarkdown(cells [][]string) string          // Pipes in cells escaped as \|.
  ```
- Rules:
  - **Links:**
    - every OSC 8 run on a row is a link;
    - runs with the same address on consecutive rows merge when the first run reaches its row's last drawn cell;
    - footnote addresses (`document.FootnotePeer`) are not items;
    - a link's `Open` is offered only when `webLink(url) != ""` and `OpenLink` is set; `Copy address` always is.
  - **Pictures:**
    - a picture is the `[image: …]` caption row plus its `image` rows;
    - its `link` is `renderedText.imageLinks[index]`;
    - its `path` is `doc.Images[i].Path`;
    - its `shown` is the animator's presented frame or `doc.Images[i].Image`.
  - **Tables:** a table is its tagged rows. Its cells come from `textTable.rows` through `units[id].plain`. Task 6 adds the table's open entry; in this task, `Copy as TSV` and `Copy as Markdown`.
  - **Tab and Shift-Tab:**
    - when `itemsHere()`, they move the focus instead of `switchFile`;
    - with no items, they set the no-items note;
    - `]` and `[` still switch files;
    - Tab first focuses the first item at or below the top visible row.
  - **The highlight** is drawn with reverse video (`\x1b[7m`), so it shows under glyphs and Kitty alike:
    - links: the link's cells;
    - pictures: the caption row;
    - tables: the header row.
  - **`a`/Shift-F10** opens the focused item's menu at its first row. With no focus, it acts like Tab, then opens.
  - **A right-click** on an item focuses it and opens its menu at the pointer:
    - a picture's hit area is `inset ≤ x < inset+columns` on its rows;
    - a link's hit area is `oscLinkAt`;
    - a table's hit area is its rows.
  - **Dropping the focus:** the focus is cleared on `layout`, `s`, file change, reload and `clearMarkdown`. After `layout`, it is re-found by kind and ordinal.

- [ ] **Step 1: Write the failing tests.** Use `searchDocument` with `m.markdown.raw = false; m.layoutMarkdown()`, as `TestMarkdownLinkClicks` does.
  - `TestItemsInReadingOrder`: write `x.png` (a small RGBA) and `doc.md` into `t.TempDir()`, then load `doc.md`. The document `"![a](x.png)\n\n[l](https://e.org/)\n\n|h|i|\n|-|-|\n|1|2|\n\n[^1]\n\n[^1]: note"` gives kinds `[itemPicture, itemLink, itemTable]`, and the picture's `path` is the absolute `x.png`. The footnote reference and back link are not items.
  - `TestItemsMergeWrappedLinkAndLinkedPicture`:
    - a link label long enough to wrap at width 20 is one item spanning 2 rows;
    - `[![a](x.png)](https://e.org/)` is one picture item with `link == "https://e.org/"`;
    - its entries end with `Open link, Copy address`.
  - `TestTabMovesFocusNotFiles`:
    - with two files, Tab keeps `m.index` and focuses item 0;
    - Shift-Tab from item 0 wraps to the last item;
    - `]` changes the file.
  - `TestTabWithNoItems`: a plain paragraph document; Tab leaves the index unchanged and sets the no-items note.
  - `TestTabScrollsIntoView`: an item after 100 paragraphs; after Tab, its `rows[0]` is within `[offset, offset+height)`.
  - `TestFocusDroppedOnReflow`: focus the link, then send `tea.WindowSizeMsg` with a new width. Afterwards, `m.markdown.focus` is the same link (re-found) or nil, never a row index past `len(lines)`.
  - `TestRightClickOnLinkOpensMenu`:
    - right-click on the link's cell → the menu has `Open, Copy address` when OpenLink is set, and `Copy address` alone when it is nil;
    - a right-click on blank text → no menu, and `m.note == ""`.
  - `TestTableCopies`:
    - `tableTSV([][]string{{"h","i"},{"1","2"}}) == "h\ti\n1\t2\n"`;
    - `tableMarkdown` of `{{"a|b"}}` contains `a\|b`.
  - `TestRawViewHasNoItems`: after `s` (source), Tab switches files.
- [ ] **Step 2: Run** `go test ./internal/app ./internal/document -run 'Items|Tab|Focus|RightClick|TableCopies|RawView|MarkdownImagePath'`, together with `TestMarkdownImagePath`. That test belongs in `internal/document`: Path is absolute for a beside-file image, and empty for a Mermaid fence and for a notebook picture. Expected: FAIL.
- [ ] **Step 3: Implement** in the order of the files listed. Keep `linkAt`'s left-click path unchanged; the right-click handling is separate, before it.
- [ ] **Step 4: Run** `go test ./internal/app ./internal/document`. Expected: PASS. That includes `TestMarkdownLinkClicks`, `TestLinkedMarkdownImageClick` and the footnote tests, unchanged.
- [ ] **Step 5: Commit** with the message `feat: Tab across pictures, links and tables in Markdown, with their actions`.

### Task 5: Items on the typeset page

**Files:**
- Create: `internal/document/typeset_items.go`, `internal/document/typeset_items_test.go`, `internal/app/typeset_items_test.go`
- Modify: `internal/document/typeset_render.go` (fill `out.TypesetItems`; draw the focus), the Request (`TypesetFocus`), `internal/app/typeset.go` (keep the items on `acceptTypesetView`, as `typesetLinks` are kept; the reveal), `internal/app/items.go` (typeset branch of `moveFocus`, hit test, entries)

**Interfaces:**
- Consumes: `pageItem`, `itemEntries`, `moveFocus` (Task 4); `typesetPixel` (existing).
- Produces:
  ```go
  // document
  type TypesetItemKind int
  const (
  	TypesetPicture TypesetItemKind = iota
  	TypesetLinkItem
  	TypesetTable
  )
  type TypesetItem struct {
  	Kind        TypesetItemKind
  	Rects       []image.Rectangle // Page layout pixels (scale 1, top of page 0).
  	URL         string            // A link, or the link around a picture.
  	Destination string            // A picture's key.
  	Path        string            // A picture's own file, as MarkdownImage.Path.
  	Image       image.Image       // A picture's still.
  	Cells       [][]string        // A table's plain text, header first.
  }
  // Result.TypesetItems []TypesetItem: every item laid so far, in reading order, Markdown only.
  // Request.TypesetFocus []image.Rectangle: page rectangles to outline.
  ```
- Rules:
  - **Where the items come from:** the items are computed from `lay` over the laid extent:
    - `lay.Links` and `lay.Images` with `nttypeset.View{Top: 0, Height: lay.Extent(), Width: lay.Width(), Scale: 1}`, so that rects are in page pixels;
    - tables from the `Starts()` span of each `KindTable` block, with cells joined from `Span.Text`.
  - **Merging and filtering:**
    - consecutive link rects with the same URL merge into one item;
    - a link rect inside a picture's rect becomes that picture's `URL`;
    - footnote URLs are dropped.
  - **The focus outline:**
    - drawn in `Palette.Accent`, 3 layout pixels wide, just outside each focus rect, scaled to the raster;
    - drawn onto the fresh raster before compositing, so GIF overlays do not cover it;
    - it shows in glyph mode and under Kitty because it is in the picture.
  - **App side:**
    - `moveFocus` sets the focus to the item's `Rects`, scrolls with a new `m.typesetGoTop` (−1 for none) when the first rect is outside the viewport, then re-requests through `queueTypesetReflow`;
    - when the next item is not yet laid and `!TypesetDone`, Tab scrolls one screen down and tries again on the result.
  - **What a typeset picture copies:** `shown` crops the page picture on screen (the presented composite when GIFs play) to the picture's rectangle, so an animation copies the frame shown. If the picture is partly off screen, it falls back to `TypesetItem.Image`.
  - **Hit tests:** a right-click hit test goes through `typesetPixel` against the item rects mapped into the viewport (`rect.Sub(image.Pt(Left, Top))`, scaled as `typesetLinks` are).

- [ ] **Step 1: Write the failing tests.**
  - `TestTypesetItemsMarkdown` (document):
    - typeset the source from `TestItemsInReadingOrder`;
    - `TypesetItems` kinds are `[picture, link, table]`;
    - the link has 1 or more rects, all with `Min.Y >= 0`;
    - the table cells are `{{"h","i"},{"1","2"}}`;
    - no item has a `#fn` URL.
  - `TestTypesetFocusOutline` (document):
    - the same request with `TypesetFocus` set to the link's rects;
    - the raster differs from the unfocused raster only within 6 pixels of those rects, scaled.
  - `TestTypesetTabAndMenu` (app):
    - a typeset Markdown model (as in `typeset_select_test.go`);
    - Tab sets a focus;
    - `a` opens a menu whose entries are `Copy address` (OpenLink nil) for the link;
    - Shift-Tab from the first item wraps.
  - `TestTypesetRightClickPicture` (app), in glyph geometry (the `TestTypesetSelectionGlyphGeometry` setup):
    - a right-click at the cell over the picture's centre opens a menu whose first entry is `Copy image` (CopyImage set, OpenWith nil);
    - a right-click on margin opens nothing.
- [ ] **Step 2: Run** `go test ./internal/document ./internal/app -run 'TypesetItems|TypesetFocus|TypesetTab|TypesetRightClick'`. Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/document ./internal/app`. Expected: PASS, with selection, link hover and footnote tests unchanged.
- [ ] **Step 5: Commit** with the message `feat: Tab and actions on the typeset page, outlined in the picture`.

### Task 6: Open a Markdown table, in gloss or another app

**Files:**
- Modify: `internal/app/items.go` (`tableEntries`), `internal/app/fetch.go` (`fetchedDoc` gains `markdown *markdownView`, `focus int`, `typesetTop int`), `internal/app/open_with.go` (a leading `gloss` row)
- Test: `internal/app/table_open_test.go`

**Interfaces:**
- Consumes: `Options.Scratch` (Task 1); `pageItem.cells` (Tasks 4 and 5); `addDropped`, `isFetched`, `closeFetched` (existing).
- Produces:
  ```go
  func tableCSV(cells [][]string) []byte                // encoding/csv
  func (m *Model) openTable(cells [][]string, chooser bool) tea.Cmd
  ```
- Rules:
  - **The entry:**
    - `Open table with…` is offered when Scratch and OpenWith are both set;
    - `Open table in gloss` is offered when only Scratch is set;
    - neither is offered when Scratch is nil.
  - **The chooser:**
    - the file is `table.csv` written with Scratch;
    - the chooser lists `{ID: "gloss", Name: "gloss"}` first, then the host's Open choices without `Copy path`.
  - **Opening in gloss:** choosing `gloss` (or the in-gloss entry) appends a `fetchedDoc{path, from: m.index, ...}` holding the Markdown view's offset or the typeset top, plus the focused item's ordinal. It then calls `addDropped`, as `fetchedFile` does, with the note `opened table`.
  - **Closing:** Esc on it goes through `closeFetched`. That restores the document, its scroll position and the focus on the same table.

- [ ] **Step 1: Write the failing tests.**
  - `TestTableCSV`: cells with a comma and a quote round-trip through `csv.NewReader`.
  - `TestOpenTableInGloss`:
    - Scratch writes into `t.TempDir()`, and OpenWith is nil;
    - focus the table, then `a`, then choose `Open table in gloss`;
    - after loading, `m.sheet != nil` and `m.isFetched()`;
    - `esc` returns to the Markdown file with the same `offset`, and the focus is on that table;
    - the CSV file no longer exists.
  - `TestOpenTableWithListsGlossFirst`: with fakeOpenHost, the chooser's first row is `gloss`, and no row is `Copy path`.
- [ ] **Step 2: Run** `go test ./internal/app -run 'TableCSV|OpenTable'`. Expected: FAIL.
- [ ] **Step 3: Implement.** Handle the `gloss` row in `openWithKey` before `openWithLaunch`.
- [ ] **Step 4: Run** `go test ./internal/app`. Expected: PASS, with the existing fetch tests unchanged.
- [ ] **Step 5: Commit** with the message `feat: open a Markdown table as a sheet in gloss, or in another app`.

### Task 7: Spreadsheet cell menus

**Files:**
- Create: `internal/app/sheet_actions_test.go`
- Modify: `internal/app/sheet.go` (`cellEntries`), `internal/app/model.go:729` (a right-click selects with `sheet.click`, then opens the menu), `internal/app/keys.go` (`a` and `shift+f10` on sheets), `internal/app/view.go` (the sheet keys line gains ` · a actions`)

**Interfaces:**
- Consumes: `openActions` (Task 2); `m.open`, `m.openQR`, `m.opts.OpenLink` (existing).
- Produces: `func (m *Model) cellEntries() []menuEntry`, for the cell under the cursor.
- Entries:
  - `Copy value` is the cell text.
  - `Copy row` is the cursor row's shown columns, tab-joined.
  - When `sheet.url() != ""`:
    - `Open in gloss` is offered only when `Fetch` is set;
    - `Open in browser` is offered when `OpenLink` is set;
    - `Show QR` is always offered.
  - The title is the cell's reference, for example `B3`, and the value is truncated.
- Tab in sheets keeps switching files.

- [ ] **Step 1: Write the failing tests.**
  - `TestCellEntries`: on a sheet with `https://e.org/` at the cursor:
    - all hosts set → `Copy value, Copy row, Open in gloss, Open in browser, Show QR`;
    - Fetch and OpenLink nil → `Copy value, Copy row, Show QR`;
    - a non-address cell → `Copy value, Copy row`.
  - `TestRightClickSelectsCell`: a right-click on another cell moves `sheet.at` there and opens a menu titled with that cell's reference. A right-click past the last column opens nothing.
  - `TestSheetTabStillSwitchesFiles`.
- [ ] **Step 2: Run** `go test ./internal/app -run 'CellEntries|RightClickSelectsCell|SheetTab'`. Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `go test ./internal/app`. Expected: PASS, with `sheet_test.go` unchanged.
- [ ] **Step 5: Commit** with the message `feat: actions on a spreadsheet cell: copy value or row, open, show QR`.

### Task 8: Guide, changelog, AGENTS.md and the gate

**Files:**
- Modify:
  - `docs/hugo/content/guide/controls.md`: a "Actions" section covering right-click, Tab/Shift-Tab in rendered Markdown, `[`/`]` for files, `a`/Shift-F10, what each menu offers, and what is left out in the browser and `--serve`. Wherever Tab is documented as the next file, change it to "`]` (Tab outside rendered Markdown)".
  - Any other guide page that names Tab for files. Find them with `grep -rn 'Tab' docs/hugo/content/guide`.
  - `CHANGELOG.md` Unreleased: one bullet for the menus, and one for Tab's new meaning in Markdown.
  - `AGENTS.md`: under **Markdown**, a sentence on items, Tab and `layerActions`. Under **Native open-with**, the gloss row for tables and the Scratch folder. Under **Key viewer features**, `a` actions.
  - `DEVELOP.md`, only if it lists layers.
- Test: `internal/app/help_test.go`, if it pins help lines.

- [ ] **Step 1:** Edit the docs. Run `task docs`.
- [ ] **Step 2: Run** `GOWORK=off task ci`. Expected: every step passes. If staticcheck fails on export data, rerun that step with `GOTOOLCHAIN=go1.26.9` and the later steps directly, as in memory.
- [ ] **Step 3: Check the behaviour by hand in tmux,** using `send-keys -H` with SGR right-clicks (`\x1b[<2;X;YM`) on `examples/` Markdown, in the text and typeset views and under glyphs (`g`). Confirm:
  - the menu appears;
  - Copy image pastes into Preview;
  - Esc returns from an opened table to the same place.
- [ ] **Step 4: Commit** with the message `docs: actions menus, Tab across items, and [/] for files`.
