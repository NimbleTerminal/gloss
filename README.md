# gloss

A visual pager for the terminal: like `less`, for images, SVGs, PDFs, STL and 3MF
meshes, Markdown, HTML, plain text, JSON, Jupyter notebooks, Word, Excel, Grist,
and CSV files.

[**Try gloss in your browser →**](https://nimbleterminal.github.io/gloss/) — starts with a sample image, including on phone-width screens.

[![gloss showing images, GIF playback, PDFs, 3D meshes, Markdown search, tables, and QR codes](docs/assets/gloss-demo.gif)](https://nimbleterminal.github.io/gloss/)

Recorded with [ntrecord](https://github.com/NimbleTerminal/ntrecord), including real
Kitty graphics. [Demo tape](scripts/gloss-demo.tape).

## Install

```sh
brew install --cask nimbleterminal/tap/gloss     # macOS or Linux
```

On Debian or Ubuntu, download the `.deb` from the
[latest release](https://github.com/NimbleTerminal/gloss/releases/latest) and run
`sudo apt install ./gloss_*_linux_amd64.deb` (or `_arm64`). The release also has
archives for macOS, Linux, and Windows: one binary, nothing else to install. With
Go, `go install github.com/NimbleTerminal/gloss/cmd/gloss@latest` builds it from
source.

## Use

```sh
gloss photo.png report.pdf model.stl   # page through them
gloss animation.gif                  # plays automatically; Space pauses
gloss --menu *.png                     # choose from a menu
gloss --preview ~/Documents/*          # a menu with a preview pane
gloss ~/Pictures                       # browse a folder
gloss --page 3 report.pdf              # a page of a PDF
cat drawing.svg | gloss                # read from a pipe
```

| Key | |
| --- | --- |
| `?` | Help |
| `q` | Quit |
| `n` / `p` | Next / previous page, or file |
| `/`, `n` / `N` | Search text; next / previous result (`Esc` clears) |
| `[` / `]` | Previous / next file |
| `m` / `o` | File menu / browse for a file |
| `+` / `-`, arrows | Zoom and pan; orbit a mesh |
| `i` | Details about the file |
| `e` | Export what you see as a PNG |
| `t` on text, source, JSON or Markdown | Toggle normal text / typeset view |
| `s` on Markdown | Toggle rendered text or typeset view / source |
| `u` | Show a selected table URL as a QR code |
| Space, `<` / `>` on a standalone GIF | Pause / resume; slower / faster playback |
| `,` / `.` on a standalone GIF | Step frames; click or drag the bottom bar to seek |

Pictures use Kitty graphics where the terminal has them, and colored half-blocks
where it does not. Drag files onto the terminal to open them; add `--fetch`
to open a dropped web URL. The [file browser](https://nimbleterminal.github.io/gloss/docs/guide/browser/)
offers list, columns, and places layouts, with filtering and Tab completion.

## Markdown

Markdown uses Glamour by default. Press `t` for typeset Markdown, or run
`gloss --typeset examples/typeset.md`: embedded fonts, highlighted code,
tables, and local pictures, drawn directly with go-text. The document reflows to
the terminal width and scrolls continuously with arrows or the wheel. It looks
best in Kitty; half-blocks work elsewhere. Use `+`/`-` to zoom, `s` for source,
and `e` to export the visible viewport as PNG. Plain text, source code and JSON
also support `--typeset` and `t`; normal terminal text remains the default.
`--theme auto|light|dark` sets text and typeset colors; auto follows the terminal,
while headless PNG exports default to light.

## From a script or an agent

gloss works without a terminal too: it exports PNGs sized for vision models,
takes out a file's text or its details, and can ask a person for a file.

```sh
gloss --output page.png --page 3 report.pdf   # a PNG, with no terminal
gloss --text report.docx                      # Markdown on stdout
gloss --grep '(?i)total' report.pdf           # the PDF pages that match, and where
gloss --info --json model.3mf                 # what a file says about itself
gloss --serve report.pdf                      # show a document in a browser
gloss --serve --pick                          # ask the user for a file in a browser
gloss --pick-web --prompt "Send a photo and a note" # simple upload page
```

Without a terminal on stdin, `--serve` and `--pick` return session JSON
immediately; `gloss --resume TOKEN` waits for the chosen paths, `--status`
looks without waiting, and `--cancel` ends the session. `gloss skill schema`
prints the JSON Schema of everything gloss writes as JSON. Add
`--accept 'image/*'` to require images, including SVG. See
[handing files over](https://nimbleterminal.github.io/gloss/docs/guide/handoff/).

`--pick-web` lets a person send files and an optional reply (`message` in
status/resume JSON). Add `--listen IP:port` and, for a wildcard bind,
`--advertise-host IP-or-name` to pick from another device; foreground terminal
sessions show a QR code. LAN mode is opt-in, plain HTTP, and token-guarded:
keep the link private and use a trusted network. See the handoff guide above.

gloss also carries a skill that teaches an agent all of this, in the order it
needs it: read a file's text, see a page or model as a PNG sized for its vision
budget, learn a file's details as JSON, and ask you for a file. Install it for
the agents on your machine, from the binary or from the repository:

```sh
gloss skill install                           # from the binary, so it matches its flags
npx skills add NimbleTerminal/gloss            # or from the repository, for every agent it finds
```

Read it on the site, [For LLMs](https://nimbleterminal.github.io/gloss/docs/guide/for-llms/),
or as [`SKILL.md`](skills/gloss/SKILL.md).

## Documentation

The [documentation](https://nimbleterminal.github.io/gloss/docs/) has the rest: the
controls, the formats gloss opens and their limits, exporting for vision models,
handing files over, and every option, grouped by what it is for. `gloss --help`
lists the options, and `man gloss` and tab completion for bash, zsh, and fish work
after a Homebrew or `.deb` install; the release archives carry both too.

To work on gloss, see [DEVELOP.md](DEVELOP.md).

## License

This `gloss` project is released under the [MIT License](https://en.wikipedia.org/wiki/MIT_License), see [LICENSE](./LICENSE). The licenses of the modules gloss links are in [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).

Copyright (c) 2026 [Neomantra Corp](https://www.neomantra.com).

----
Made with :heart: and :fire: by the team behind [Nimble.Markets](https://nimble.markets).
