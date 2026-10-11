# Open with

Status: Phase 1 implemented on `feat/open-with`. CI, demo checks and the WASM
build pass. Real GUI launches, Alt+Enter delivery and Kitty/tmux restoration
remain manual checks. The initial enabled catalog is Preview (raster/PDF), Skim,
and VS Code/Cursor/Zed/Sublime on macOS; those editors on Linux; system/reveal
on Windows. The larger seed below remains a candidate list. The chooser uses an
opaque modal body while preserving the underlying view, avoiding graphics over
its rows.

## Agreed scope

Phase 1 adds native pager launching, an app chooser, per-kind defaults, and
`--apps [--json]`. This round produces this specification and its implementation
plan, not the implementation. The user confirmed `w`/`W`, Alt+Enter in the file
browser, saving defaults with `d` in the chooser, and deferring terminal editors.
Phase 2 (served-page and headless launching) is described here but is not in the
implementation plan. Phase 3 covers terminal applications.

The Go package prefix is `nimbleterminal.dev/gloss`. The project, issues and
releases remain at `github.com/NimbleTerminal/gloss`.

## Interaction

| Context | Target | Action |
| --- | --- | --- |
| Document | Original file being viewed, not an exported page | `w` opens default; `W` shows chooser |
| File menu or grid | Highlighted file, not the last loaded preview | Same keys |
| File browser | Highlighted entry | Alt+Enter shows chooser without selecting or navigating |
| Browser directory | Highlighted directory, including parent row | Reveal in file manager only |
| Preview | None | No keys or launching |
| Public browser app/demo | None | `w`/`W`: “not available in the browser” |
| Served viewer in Phase 1 | None | No launcher installed; explain unavailability |
| `--pick-web`, non-loopback listener | None | Never expose launching |

The chooser lists System default first, then installed, visible compatible
catalog apps in stable name order, then Reveal in Finder/Explorer/file manager,
then Copy path. Directories have only Reveal. Select the effective default row
initially. Up/Down or j/k move, Enter acts, Esc or q closes, Ctrl-C quits gloss.
`d` saves the selected app as the default for this kind without launching or
closing; System default removes the saved override. Copy and Reveal cannot be
defaults. Saving is immediate; Esc does not undo a successful save. No live
launching on cursor movement. A save failure leaves the previous default intact.

The header identifies the file and kind. Names and errors use existing safe
terminal-text escaping. The overlay scrolls on short screens and retains its
cursor through resize; below its minimum size it says to enlarge the terminal
and allows cancellation. It does not allocate graphics IDs. Reuse existing
overlay picture suppression/restoration so Kitty images cannot cover the list.
Pause document animation while the chooser covers it and resume on close.

If Alt+Enter is not delivered distinctly, Enter keeps its normal behavior. For
a supported file, open it with Enter and press W; alternatively configure the
terminal to send Alt+Enter. Do not guess from a separate Esc followed by Enter.
Directory reveal requires a terminal that reports the binding.

## Host boundary and package

Add `internal/openwith`: shared value types and native catalog, preferences,
probes and launch implementation. It has no dependency on app, document, or
Bubble Tea. Native filesystem/process code has `!js` build constraints;
portable value types remain usable by the WASM app. Gloss adapts document kinds
at the application/CLI boundary. No new external dependency is required.

```go
type Target struct {
    Path string
    Kind string
    Directory bool
    Temporary bool
}
type Choice struct {
    ID string
    Name string
    Default bool
    Available bool
}
type Host interface {
    Choices(Target) ([]Choice, error)
    Launch(Target, string) (string, error) // empty ID resolves default; returns app name
    SetDefault(kind, id string) error
}
```

`Options.OpenWith openwith.Host` is nil unless the embedding host explicitly
permits native launching. Main installs the native host only for a native
interactive session, not the server's terminal transport. `Copy path` is an app
action using `tea.SetClipboard`, not an executable catalog entry. Host Choices
returns System default, apps and Reveal; the app appends Copy for regular files.

Snapshot the target when the action is requested. Probe and launch in `tea.Cmd`,
never during View or synchronously in Update. Tag responses with an operation
counter and document/screen generation; discard stale list/status responses.
Allow only one outstanding launch/save per chooser. Ignore repeated Enter while
busy. An already started launch cannot be undone by changing files.

Resolve native paths with `filepath.Abs` and `Clean`, stat before opening, and
accept only regular files or directories for reveal. A viewed document supplies
its original loader kind, including Word/HTML converted internally to Markdown.
Menu/browser targets use bounded `document.Probe` outside Update. Normalize
raster kinds to `image/*`; SVG retains `svg`. Unknown browser regular files get
System default, Reveal and Copy only, no saved kind default; do not force a
text kind or decode whole files to populate the chooser. Existing accept rules
continue to govern importing into gloss, not external launching from the browser.

Add `browse.KeyMap.OpenWith` and exported `browse.OpenWithMsg{Path string,
Directory bool}`. The component emits a snapshot from CurrentPath/Current,
without changing Chosen, filter, directory or history. Disable it within the
browser's own sidebar/type popup. The gloss adapter handles the message and
converts the slash path to a native path. Route an active open-with overlay
before the current early screenBrowser return in `Model.key`; otherwise its
letters would enter the browser filter. Finder and go-to input retain their keys.

## Catalog and defaults

Each compiled entry has `ID`, `Name`, `Kinds`, `Exec []string` and a platform
availability probe. IDs are stable across platforms where the same app exists.
Only an entire `{}` argument is substituted; append the absolute file path when
absent. Unlike thinkt's terminal templates, no inline template language is needed.
Commands never come from the preference file. Do not import thinkt or its config.

Start with this deliberately small seed. Each native entry must have a fake-probe
test and a verified direct GUI invocation before it is enabled in the catalog.
An app accepting a format does not guarantee support for every variant of it.

| IDs / names | Platforms / probes | Kinds |
| --- | --- | --- |
| system / System default | macOS `open`, Linux `xdg-open`, Windows `rundll32.exe` | All regular files |
| reveal / Reveal | macOS `open -R`, Windows Explorer selection, Linux `xdg-open` parent folder | All, including directories |
| preview / Preview | macOS Preview.app | image/*, svg, pdf |
| skim / Skim | macOS Skim.app | pdf |
| prusaslicer / PrusaSlicer | macOS PrusaSlicer.app; Linux `prusa-slicer` | stl, 3mf |
| bambustudio / Bambu Studio | macOS BambuStudio.app; Linux `bambu-studio` | stl, 3mf |
| gimp / GIMP | macOS GIMP.app; Linux `gimp` | image/* |
| inkscape / Inkscape | macOS Inkscape.app; Linux `inkscape` | svg |
| okular / Okular; evince / Document Viewer | Linux respective executables | pdf |
| vscode / VS Code; cursor / Cursor; zed / Zed; sublime / Sublime Text | macOS named bundles; Linux GUI `code`, `cursor`, `zed`, `subl` | markdown, text, json, html, ipynb, csv |
| word / Microsoft Word; excel / Microsoft Excel | macOS respective bundles | docx; xlsx,csv |
| pages / Pages; numbers / Numbers | macOS respective bundles | docx; xlsx,csv |
| libreoffice / LibreOffice | macOS LibreOffice.app; Linux `libreoffice` | docx,xlsx,csv |

macOS checks `/Applications` and `~/Applications`, launching the detected bundle
path with `open -a BUNDLE FILE`; Preview also checks `/System/Applications`.
Linux resolves an executable through LookPath and retains its absolute path.
Windows v1 offers System default and Reveal; do not run `.cmd` editor shims through
`cmd /c` to fill the catalog. Other native platforms use a probed `xdg-open` or
report unavailable. Defer the larger suggested catalog, Jupyter server processes,
terminal editors, desktop-file parsing and registry discovery until verified.

Resolution: compatible saved ID, if installed and not hidden; otherwise System
default. If a saved app is missing/hidden, say that System default was used;
retain the known saved ID so reinstalling restores it. Never select an arbitrary
different catalog app. System default is always represented but may be unavailable
on a headless Linux host; launching then returns an actionable error. The OS
association can itself point at gloss; do not install gloss as a default handler
as part of this feature. Warn about that possibility in troubleshooting.

## Preferences and launch behavior

Store `theme.Dir()/open-with.json`, maximum 64 KiB:

```json
{"defaults":{"pdf":"skim","markdown":"zed"},"hidden":["vscode"]}
```

Canonical keys: image/*, svg, pdf, stl, 3mf, docx, xlsx, grist, csv, json, ipynb,
html, text, markdown. Unknown kinds/IDs and incompatible pairs are dropped.
Validate against the full compiled catalog, before availability filtering.
Reserved system/reveal cannot be hidden; reveal/copy cannot be saved. Bound input
before JSON decoding, reject unknown top-level fields, trailing JSON and invalid
field types. Invalid configuration uses defaults with one nonfatal diagnostic;
missing file is silent. A save must not silently overwrite a malformed file.

Save by same-directory temporary file (0600), close and atomic rename; directory
0700. Serialize host preference operations and publish new state only after a
successful save. Follow existing theme/font conventions, but enforce the actual
64 KiB byte bound using limit+1, rather than assuming a JSON decoder limit alone
proves the file is short. Exports never read this preference. `--apps` does read
it because it describes the chooser. No default-setting flag or hide UI in v1.

Use exec.Command with a trusted executable and separate arguments, never a
shell. Absolute paths cannot begin with `-`; they do not need a universal `--`
which some openers do not accept. Windows serialization and Explorer's selection
syntax still need explicit tests, including commas and quotes. Reject NUL paths.
Do not inherit terminal stdin/stdout/stderr. Start, then Wait in a goroutine to
reap; never block the pager on app exit. Start failures become status notes.
“Opened in Preview” means the launch was submitted, not proof the app displayed
the file; later opener-process failures cannot establish that either.

Stdin/download files keep their existing lifetime. Track temporary provenance
from the host/session, not a filename prefix supplied by a user. Append
“temporary file; save a copy in the app to keep it” to the launch note. Do not
copy, retain or reload files implicitly. Symlinks chosen locally are allowed if
their current target is a regular file; revalidate at launch. This is not a
sandbox against another local process swapping a path after the check.

## Listing option

`gloss --apps` lists each canonical kind, its effective default and available,
visible choices in chooser order. `--apps --json` emits a deterministic array:

```json
[{"kind":"pdf","default":"skim","apps":[{"id":"system","name":"System default","available":true},{"id":"skim","name":"Skim","available":true},{"id":"reveal","name":"Reveal in Finder","available":true}]}]
```

Copy path is not a host app and is omitted. No file arguments or stdin reads;
no launch, TTY, network, document decoding or preference writes. Exit 0 for a
listing, including missing optional apps; exit 1 for fatal discovery/config-read
errors with no partial stdout. Malformed preference content follows the nonfatal
fallback above. Reject combinations with input files, --glob, --serve, --pick,
--pick-web, export/extraction and session-management modes. Allow --json; existing
help/version precedence remains. Place `apps` in the existing extract domain and
extend `gloss skill schema` with this result type.

## Later phases

Phase 2 adds a loopback viewer chrome control, never a generic host launcher in
the public app. POST `/<token>/open-with` accepts `{app,file}`; GET
`/<token>/open-with/apps?file=ID` returns choices. The file is an opaque,
session-owned identifier, never a path. The current served viewer does not yet
provide a general current-document ID registry: add one tied to file-load/drop
events, including initially supplied and host-browsed files. Export IDs are not
document IDs. Do not claim that mapping already exists.

Require the existing token and canonical same-origin check on POST, a loopback
listener verified from its bound address, and a loopback peer; reject proxy
headers as authority. Reject missing Origin for this browser-only mutation.
Limit JSON to 1 MiB, reject unknown/trailing fields, require an enabled compatible
catalog ID, resolve ID to a registered canonical file, and revalidate membership
and path at launch. Restrict to waiting/viewing sessions, one in-flight request,
with a short duplicate-request guard. No preference mutation endpoint. Return
400 invalid body, 403 origin/permission, 404 file, 409 settled/busy, 500 launch
failure. No route exists for pick-web, even on loopback. Tests must cover all
these cases and symlinks changed after registration.

Also add `--open-with APP FILE…` as a view option: bounded kind detection,
explicit filenames, no TTY, exit 0 if all launch requests start and 1 otherwise,
per-file stderr errors. No shell commands, implicit URL downloads, directory
traversal or silent fallback for an explicitly named unavailable app. Headless
launch does not consult interactive defaults. Keep JSON out initially unless
its result schema is designed and added to `skill schema`. Agents use it only
when a human explicitly requests opening in their own app; otherwise prefer
--serve or export. Document the side effect and temporary-file race with exit.

Phase 3 adds $EDITOR/$VISUAL for text with tea.ExecProcess, terminal suspension,
resume and reload; shell interpretation needs its own design. Consider thinkt's
ExecRun and terminal detection only then. `v` remains preview in the menu.

## Threat model and reuse

Config tampering can choose only a trusted app ID, not a command or arguments.
PATH and installed app binaries are trusted local environment inputs. A webpage
cannot supply native paths, commands, non-loopback launch requests or defaults.
Untrusted filenames are data in argv and escaped labels, never shell fragments
or terminal sequences. Opening a file hands it to another program with its own
parsers/macros/associations; this feature is not a safe-file validator. It always
requires an explicit human key/click, or an explicitly authorized headless action.

Reuse the design of go-thinkt's AppConfig, BuildCommand, Launch, validateApps,
platform probes, handleOpenIn and project picker overlay. Do not copy ExecRun,
terminal detection, wizard or directory authorization wholesale. Read from
`../go-thinkt`; do not modify it. Its LICENSE.txt is MIT, copyright 2026
Neomantra Corp. If implementation copies substantial code, preserve that notice
in a checked-in license file and register it with gloss's notices generator.

## Verification and delivery

Fake-probe catalog tests cover compatibility, missing executables/bundles and
stable order. Stub-process tests cover argv, metacharacters, Start failure and
reaping. Preference tests cover bounds, malformed/trailing JSON, ID filtering,
round-trip, failed atomic replacement and concurrent access. Event-loop tests
cover all contexts, target snapshots, nil host, repeated Enter, stale results,
resize, cancel, graphics restoration, animation pause and save failure. A
browsetest script must precede the browser key change; test all three layouts,
filtered/empty selections, folders and popup precedence.

Update AGENTS.md, guide controls/using, SKILL.md (--apps/schema only), DEVELOP.md,
Unreleased changelog and a short README line when Phase 1 ships. Generate docs;
do not describe Phase 2/3 as available. Run task ci and demo-check, including
GOWORK=off and a compatible Go toolchain. Manually verify native GUI launch,
focus/return, file paths, terminal Alt+Enter, and Kitty/tmux overlay cleanup on
real hosts. Cross-compilation alone does not verify Windows/Linux desktop apps.
