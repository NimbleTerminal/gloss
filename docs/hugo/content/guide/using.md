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
  images, animated GIFs, and highlighted code. Text, source, and JSON also
  support `t`. Choose fonts with `f` and themes with `T`; see [Markdown](../markdown/).
- Search document text, table cells, or PDF text with `/`. On source and text,
  `#` shows line numbers, `:` goes to a line, and `S` toggles long-line wrapping.
- Pause a standalone GIF with Space, change speed with `<`/`>`, and scrub or
  step frames. Orbit STL/3MF meshes and select 3MF parts with `c`.
- Press `e` to export the current image or typeset viewport. Scripts use
  `--output` without a terminal; meshes can export several views in one sheet.
- Show a table URL as a QR with `u`, or use `--pick-web` for phone uploads and
  replies. See [file handoff](../handoff/) for LAN setup and agent sessions.

The [controls reference](../controls/) covers the keys for each media type.
