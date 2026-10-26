---
title: "Using gloss"
weight: 15
bookToC: false
---

# Using gloss

gloss shows the files you name, or the contents of a folder, or what you pipe into it. With no file it opens empty: drop files on it, or press `o` to browse. An `http://` or `https://` address in place of a file is downloaded first, then shown like the file it is; see [Opening a web address](#opening-a-web-address).

```sh
gloss photo.png drawing.svg report.pdf model.stl
gloss --menu photo.png report.pdf model.stl       # a menu to choose among them
gloss --preview photo.png report.pdf model.stl    # the menu, with a preview pane
```

Kitty graphics are selected automatically on supporting terminals, with colored
half-block glyphs as a fallback. The program uses the alternate screen and
restores the terminal on exit. With `-X` (`--no-alt-screen`, as in `less`) it
draws on the main screen instead: the scrollback is left alone, and the last
view stays where it was drawn when you quit. The choice can wait until you
quit: `q` quits as launched, and `Q` quits leaving the last view in the
scrollback either way, so a picture worth keeping beside the next command
stays, and one that is not does not. Direct graphics transport works over SSH; no shared
filesystem or external converter is needed. In tmux, enable passthrough with
`set -g allow-passthrough on`.

```sh
gloss --page 12 report.pdf
gloss --dpi 300 report.pdf        # higher PDF raster resolution
gloss --render glyph photo.png   # universal terminal rendering
gloss --render kitty photo.png   # override capability detection
gloss --3d software model.stl    # bypass GPU initialization
gloss --3d wireframe model.stl
cat drawing.svg | gloss
cat model.stl | gloss --type stl -
gloss -- -filename.png
gloss -X photo.png               # keep the scrollback; the picture stays after q
gloss --serve report.pdf         # show the viewer on a web page instead
gloss --serve --pick             # ask the user for a file; print its path
gloss --pick --prompt "The invoice, please"   # and say what for
gloss --glob images --glob meshes ~/models    # open every match under a folder
gloss                            # no file yet; drop files or press o to browse
gloss ~/Pictures                 # browse a folder for a file to open
gloss https://example.org/a.pdf  # download a web address and show it
```

## Printing one view

`--print` draws the initial content viewport to stdout and exits. It works in
captured shell output, including `!` commands in coding agents, without a TTY:

```sh
gloss --print photo.png
gloss --print --print-width 100 --print-height 30 report.pdf
gloss --print --page 2 --cols name,total sales.xlsx
cat notes.md | gloss --print -
```

Pictures use ANSI-colored Unicode half-blocks. Text, Markdown and tables use
normal terminal rendering. There is no viewer
chrome, input loop, cursor movement, terminal probing or Kitty protocol. GIFs
and animated WebP stay on frame one. Meshes use a software snapshot. A host
that strips ANSI styles will lose picture colors; for an agent to inspect
pixels, use PNG export and its image-reading tool instead.

The viewport defaults to the stdout terminal size, or **80 columns × 24 rows**
when output is captured. `--print-width 1..512` and `--print-height 1..256`
override it; terminal dimensions are capped at those limits. Each named file
prints one viewport, separated
by a blank line. `--page` selects one page/sheet (or a typeset export slice),
and `--cols`/`--coln` filter tables.
`--typeset`, line numbers and chopped lines are supported. Auto theme is light
without probing or reading saved preferences; use `--theme dark` explicitly
for dark command output.

`--print` cannot be combined with another output mode, interactive viewer
options, or `--render kitty`. It writes only content to stdout and errors to
stderr, with exit 0 on success and 1 on error; an error stops the batch after
any earlier views already printed. `-X` remains the interactive main-screen
viewer, which requires a terminal.

## Frames of a video

A video's first frame is often black. `--at` names the moments wanted, as
`1:23`, `1:02:03`, `83.5` or `1m23s`, up to 16, separated by commas;
`--frames N` spreads N of them through the video, the middle of each of N
equal parts. Several make one sheet with each frame's time in its corner,
or a file each with `--output-dir`, and `--print` shows them too. One ffmpeg
draws them all, and none is enlarged. With `--json`, each entry lists its
frames' times in seconds and the subtitle shown at each. In the viewer,
one `--at` starts the video there.

```sh
gloss --frames 9 --output sheet.png clip.mp4
gloss --at 1:23 --output frame.png clip.mp4
gloss --at 0:05,0:30,1:00 --output-dir frames clip.mp4
gloss --text clip.mp4             # its subtitles, each after its times
gloss --at 1:23 clip.mp4          # play from 1:23
```

## Showing a link as a QR code

`--qr TEXT` shows TEXT, usually a web address, as a QR code in place of a file,
for a phone or tablet to scan. It works wherever a picture does: the viewer,
`--serve` (a web page, where an agent with no terminal can show it to you),
`--output` and `--print`. The viewer and the page show the text above the code,
unless `--prompt` says something else. The text is only encoded: nothing is
fetched, and `GLOSS_NO_FETCH` does not apply.

```sh
gloss --serve --qr http://192.168.1.20:8000/
gloss --qr https://example.com/ --output qr.png
```

A code for a `localhost` address is no use on another device; give the address
the device can reach.

## Opening a web address

Any argument that is an `http` or `https` URL is downloaded before gloss starts
and takes the file's place, so it works wherever a file does: the viewer, the
menu, `--serve`, and the headless modes `--info`, `--text`, `--grep`, and
`--output`, and `--print`. Naming the address is the request, so no option is needed. The
download has one minute and the 128 MiB file limit, follows at most ten
redirects and never from https to http, sends no cookies or stored credentials,
and must be a format gloss shows; `--type` and `--accept` apply to it as to a file.
Other schemes, such as `ftp://`, are errors, and so is a download that fails,
exactly as a file that cannot be opened, except that `--info` reports it among
the others, under its address, and goes on. The file is named after the end of
the address and removed when gloss exits, unless it was picked. A detached
`--serve` or `--pick` downloads before the server starts, into the session's
private folder, where a picked download then stays for the caller. Only file
arguments are downloaded; an address given to an option, such as `--qr` or
`--prompt`, is passed on as it is.

Downloads identify themselves as `gloss/` and the version, as `gloss --version`
prints it. A host that wants another name gets it from `--user-agent`, or from
`GLOSS_USER_AGENT` in the environment when the option is not given: one line of
at most 1 KiB without control characters. The same name is sent for addresses
opened inside documents with `--fetch`.

A URL argument is a network request. A script that passes names it did not
choose can forbid it by setting `GLOSS_NO_FETCH` to any non-empty value in the
environment: address arguments then fail, and `--fetch` is refused. Keep
passwords out of addresses; they would be sent as Basic authentication and
remain in your shell history. A local file whose name begins with `http://`
cannot be named on the command line; there are none.

Options use `pflag` GNU syntax and may appear before or after filenames. Both
`--page=3` and `-p3` work; boolean short flags can be grouped (`-mP`). Use `--` to
end option parsing. `--type image|svg|pdf|stl|3mf|obj|ply|geojson|gpx|pmtiles|docx|xlsx|grist|csv|json|ipynb|html|text|markdown|mermaid|video` (`jsonl` and `ndjson` mean `json`, `md` means `markdown`) overrides detection for all
inputs. Content detection supports extensionless files. A `-` reads stdin once
into a temporary file, removed on exit; keyboard input comes from the controlling
terminal. Interactive output must be a terminal; image export works in scripts.
`gloss --help` lists flags.

Unsupported files, directories, and other non-regular entries are reported on
stderr and skipped, so globs can include unrelated entries. Directories are not
traversed. If none remain, gloss exits with an error, unless a folder was
named: then it opens the file browser there. Supported files
still enforce size limits; `--type` explicitly forces an input format.

Common short options: `-h` help, `-V` version, `-m` menu, `-P` preview,
`-X` main screen, `-N` line numbers, `-S` chop long lines,
`-p` page, `-d` DPI, `-r` render mode, `-t` type, `-o` output PNG,
`-O` output directory, and `-s` maximum image edge.

```sh
gloss report.pdf -p3 -o page.png --vision-profile openai-high
gloss photo.png drawing.svg model.stl -mP
```

## Choose a view

- Read Markdown as terminal text, or press `t` for a typeset page with local
  images, animated GIFs and WebPs, and highlighted code. Text, source, and JSON also
  support `t`. Choose fonts with `f` and themes with `T`; see [Markdown](../markdown/).
- Search document text, table cells, or PDF text with `/`. On source and text,
  `#` shows line numbers, `:` goes to a line, and `S` toggles long-line wrapping.
- Pause a standalone GIF or animated WebP with Space, change speed with `<`/`>`, and scrub or
  step frames. Play a video (MP4, MOV, WebM, MKV, OGV, AVI and more) with Space,
  seek with ←/→ or the bar; see [Formats](../formats/) for what it needs. Orbit STL, 3MF, OBJ and PLY meshes and select 3MF parts with `c`. Pan and zoom a GeoJSON or GPX map, or a PMTiles archive, drawn offline.
- Press `e` to export the current image or typeset viewport. Scripts use
  `--output` without a terminal; meshes can export several views in one sheet.
- Show a table URL as a QR with `u`, or use `--pick-web` for phone uploads and
  replies. See [file handoff](../handoff/) for LAN setup and agent sessions.

The [controls reference](../controls/) covers the keys for each media type.

## Languages

gloss speaks English, Spanish (`es`) and Simplified Chinese (`zh-Hans`). The
viewer takes the language of your terminal's locale (`LC_ALL`, `LC_MESSAGES`,
then `LANG`; `LC_ALL=C` is English) and writes numbers in it. `GLOSS_LANG`
overrides the locale, and `--lang` overrides both:

```sh
GLOSS_LANG=es gloss notes.md
gloss --lang zh-Hans report.pdf
```

`F2` over a document opens a language picker: the arrows show the viewer in
each language as you move, Enter keeps one and Esc goes back. The terminal
viewer remembers the choice in its configuration folder (`gloss/lang`), and it
then comes before `LC_ALL`, `LC_MESSAGES` and `LANG`; `GLOSS_LANG` and `--lang`
still win. **System** forgets the choice and follows the locale again. A
`--serve` viewer switches but saves nothing, and nothing headless reads the
saved choice.

`--lang` takes a BCP 47 tag (`es-MX`, `zh-CN`) or a POSIX name (`zh_CN.UTF-8`);
anything else exits with an error. A language with no translation, such as
`fr` or `zh-TW`, shows English text but still writes numbers the way that
language does. The words in the help, status line, search, export notes, the
handoff screen and the `--serve` and `--pick-web` pages are translated; option
names, values and JSON keys never are.

Output meant for scripts is English whatever your locale: `--output`,
`--print`, `--text`, `--info`, `--grep`, `--apps` and the detached
`--serve`/`--pick` start ignore `GLOSS_LANG`, `LC_*` and `LANG`. Give `--lang`
to ask for another language; then a few values change (a size is written
`7,1 KiB (7.294 bytes)` in Spanish), but keys, enum values and exit codes do
not. A page shown to a person from a detached session follows that person's
browser language; a `--serve` started in a terminal follows the terminal's.

The file browser, its help and the open-with chooser, the details box labels,
the browser demo and some upload messages in the pages are still English.
These translations are drafts awaiting native review.

## Open in another app

The apps gloss offers, and which one is the default for each kind of file, come
from your operating system: Launch Services on macOS, the XDG desktop entries
and MIME defaults on Linux and BSD, the registry on Windows. Gloss adds no
catalog of its own.

In the native viewer, `w` opens the current file in its default app, `W` opens
an **Open with** chooser, and `E` edits the file. In the file menu or thumbnail
grid these keys act on the highlighted file. In the file browser, Alt+Enter opens
the **Open with** chooser without selecting the entry; directories only offer
Reveal. If your terminal does not report Alt+Enter, open a supported file with
Enter and press `W`, or configure the terminal's key binding.

The chooser lists System default, the apps your system knows for this kind of
file, Reveal in Finder/Explorer (the containing folder on Linux), and Copy path.
Use arrows to choose and Enter to open. Press `d` on an app to save it as the
default for this kind of file and verb; this does not launch it. On Linux an app
registered for a general type is offered for its subtypes too, so a plain-text
editor appears for source files, Markdown and JSON. Esc closes the chooser; a
saved choice stays saved.

`E` edits with the app resolved for editing. For text, Markdown, JSON, HTML,
SVG, CSV, Mermaid, OBJ and notebook files that is `$VISUAL`, falling back to
`$EDITOR` when VISUAL is empty, unless you saved another default, provided that
names an installed program; otherwise, and for other files, it is the system's
default editor. When there is none, the **Edit with**
chooser opens instead. A binary file is never sent to the terminal editor, and
when no app can edit a file gloss says so. Tab in the chooser switches between
**Open with** and **Edit with**; defaults are saved per kind of file and per verb,
so the app you open a Markdown file with can differ from the one you edit it in.
On **Open with**, choose System default and press `d` to remove an override. For example, run
`EDITOR=nano gloss README.md` or set `VISUAL='nvim -f'` in your shell.

A terminal app (a terminal editor, or on Linux a desktop entry marked
`Terminal=true`) takes over the terminal: gloss suspends its UI, waits for the
app, then reloads the file, keeping the source view and scroll position where
possible. The menu and browser return to their existing selection. Use an editor
that stays in the foreground until you finish.

Press `m` in the chooser to manage installed apps across all formats: arrows
select, Space hides or restores an app and saves immediately, and Enter or Esc
returns to the chooser. Hidden apps remain in this checklist. System default and
Reveal cannot be hidden, and terminal apps are not listed. Hiding a saved
default uses the system default until that app is restored.

Preferences live in `gloss/open-with-v1.json` under your OS user-config
directory. They contain app IDs, never executable commands. An unavailable
saved app falls back to the system default. If that file is missing, gloss reads
the older `gloss/open-with.json` while the new one does not exist yet, including
its short IDs (`vscode`, `preview`, ...), and leaves it untouched; your choices are saved to the new file.
If `open-with-v1.json` is damaged, gloss reports it and does not save until you
repair or delete it. On Linux avoid making gloss its own default opener.

`gloss --apps` lists the available apps by format and verb, with the effective
default; `--apps --json` provides the same listing for scripts without opening
anything. Custom GUI commands are not supported. The browser demo, served
viewer, and web picker cannot launch host apps.

Opening a stdin or downloaded file does not extend its lifetime. Save a copy
from the other app before gloss exits if you want to keep it. Opening submits a
launch request; gloss cannot tell whether the other app displayed the file.

The file browser has its own help: press `?` before typing a filter, or `F1`
while typing. It lists navigation, completion, layouts, type filters, sorting,
find and app-opening controls. Arrows and PageUp/PageDown scroll help; Esc or F1
returns to the same folder, filter and selection. A `?` inside an existing filter
remains part of the filename search.
