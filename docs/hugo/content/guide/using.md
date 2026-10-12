---
title: "Using gloss"
weight: 15
bookToC: false
---

# Using gloss

gloss shows the files you name, or the contents of a folder, or what you pipe into it. With no file it opens empty: drop files on it, or press `o` to browse.

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
```

Options use `pflag` GNU syntax and may appear before or after filenames. Both
`--page=3` and `-p3` work; boolean short flags can be grouped (`-mP`). Use `--` to
end option parsing. `--type image|svg|pdf|stl|3mf|docx|xlsx|grist|csv|json|ipynb|html|text|markdown` overrides detection for all
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
  step frames. Orbit STL/3MF meshes and select 3MF parts with `c`.
- Press `e` to export the current image or typeset viewport. Scripts use
  `--output` without a terminal; meshes can export several views in one sheet.
- Show a table URL as a QR with `u`, or use `--pick-web` for phone uploads and
  replies. See [file handoff](../handoff/) for LAN setup and agent sessions.

The [controls reference](../controls/) covers the keys for each media type.

## Open in another app

In the native viewer, `w` opens the current file in its default app. `W` opens
an **Open with** chooser: system default, installed compatible apps, Reveal in
Finder/Explorer (the containing folder on Linux), and Copy path. In the file
menu or thumbnail grid these keys act on the highlighted file. In the file
browser, Alt+Enter opens the chooser without selecting the entry; directories
only offer Reveal. If your terminal does not report Alt+Enter, open a supported
file with Enter and press `W`, or configure the terminal's key binding.

Use arrows to choose and Enter to open. Press `d` on an app to save it as the
default for this format; this does not launch it. Choose System default and
press `d` to remove an override. Esc closes the chooser; a saved choice stays
saved. Press `m` in the chooser to manage installed apps across all formats:
arrows select, Space hides or restores an app and saves immediately, and Enter
or Esc returns to the chooser. Hidden apps remain in this checklist. System
default and Reveal cannot be hidden. Hiding a saved default uses the system
default until that app is restored.

Preferences live in `gloss/open-with.json` under your OS user-config
directory. They contain app IDs, never executable commands. An unavailable
saved app falls back to the system default. On Linux this needs `xdg-open` and
a desktop association; avoid making gloss its own default opener.

`gloss --apps` lists the available apps by format; `--apps --json` provides the
same listing for scripts without opening anything. App discovery is limited to
a built-in catalog and installed executable or app-bundle probes: initially
Preview and Skim on macOS, plus VS Code, Cursor, Zed and Sublime Text on macOS
and Linux. Windows offers the system default and Reveal. Other installed apps
remain accessible through OS file associations. Custom GUI commands are not
supported. The browser demo, served viewer,
and web picker cannot launch host apps.

For text, Markdown, JSON, HTML, SVG, CSV and notebook files, choose **Edit in
terminal** in `W`. Gloss uses `$VISUAL`, falling back to `$EDITOR` when VISUAL is
empty. For example, run `EDITOR=nano gloss README.md` or set
`VISUAL='nvim -f'` in your shell. The editor receives the original file path;
Word/PDF conversions and other binary formats are not offered for editing.
If no editor is configured, select the action for setup guidance.

Gloss suspends its terminal UI, waits for the editor, then reloads the file,
keeping the source view and scroll position where possible. The menu and browser
return to their existing selection. Use an editor that stays in the foreground
until you finish. Settings support arguments and POSIX-style quotes, with no
shell expansion, pipelines, or command substitution; an exact executable path
may also contain spaces. On Windows, use forward slashes or single quotes for
paths with backslashes when adding arguments. Settings are bounded to 8 KiB and
64 words. An invalid VISUAL is reported rather than silently using EDITOR.
The terminal editor is configured by these variables, not chooser `d` or `m`.

Opening a stdin or downloaded file does not extend its lifetime. Save a copy
from the other app before gloss exits if you want to keep it. Opening submits a
launch request; gloss cannot tell whether the other app displayed the file.

The file browser has its own help: press `?` before typing a filter, or `F1`
while typing. It lists navigation, completion, layouts, type filters, sorting,
find and app-opening controls. Arrows and PageUp/PageDown scroll help; Esc or F1
returns to the same folder, filter and selection. A `?` inside an existing filter
remains part of the filename search.
