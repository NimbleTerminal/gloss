# Adopt ntopenwith Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** gloss's open-with runs on `nimbleterminal.dev/ntopenwith` v0.1.0 (the OS's own apps and defaults), keeps people's saved choices, and gains an Edit verb: `E` edits, and Tab in the chooser switches between "Open with" and "Edit with"; `--apps --json` says which verb each row is for.

**Architecture:** `internal/openwith` keeps its role as gloss's seam (the `Host` interface the pager uses, nil in browser and serve hosts) but its insides become an adapter over ntopenwith: gloss's kinds, its corrections and aliases, its preference file, and a `Resolver`. The compiled catalog, argv launcher, preference parser and `$EDITOR` parser are deleted. The pager gains a verb on its chooser; terminal editors come back from `Launch` as a command and run through the existing `tea.ExecProcess` handoff.

**Tech Stack:** Go 1.26.9+, Bubble Tea v2, `nimbleterminal.dev/ntopenwith` v0.1.0 (+ its `system` package).

**Spec:** the request (adopt ntopenwith v0.1.0; replace internal/openwith; keep saved choices through aliases and the old preferences file; an Edit key and chooser with Tab between "Open with" and "Edit with"; `verb` in `--apps --json`), the "Moving gloss onto it" section of `../ntopenwith/docs/superpowers/specs/2026-10-20-ntopenwith-design.md`, and gloss's original `docs/superpowers/specs/2026-10-11-open-with-design.md` for behaviour that stays (keys, Copy path, Reveal, temporary-file notes, Manage apps).

## Global Constraints

- Require `nimbleterminal.dev/ntopenwith v0.1.0` in the root `go.mod`; the `examples/demo` module follows via `go mod tidy`. Regenerate `THIRD_PARTY_NOTICES.md` (`task notices`); `GOWORK=off task ci` must pass, and `GOWORK=off task demo-check`.
- Keys: `w` opens with the resolved app, `W` opens the chooser on "Open with", **`E` edits with the resolved app**, Tab inside the chooser switches verb. `e` stays export. Browser Alt+Enter (`ntfilepicker.OpenWithMsg`) opens the chooser on "Open with".
- Preferences: read and write `<theme.Dir()>/open-with-v1.json`; when it does not exist, read `<theme.Dir()>/open-with.json` with `ntopenwith.LoadLegacyPreferences`; never write `open-with.json`.
- Aliases for gloss's old IDs (`ntopenwith.Alias`), on macOS: `vscode`→`mac:com.microsoft.VSCode`, `cursor`→`mac:com.todesktop.230313mzl4w4u92`, `zed`→`mac:dev.zed.Zed`, `sublime`→`mac:com.sublimetext.4`, `preview`→`mac:com.apple.Preview`, `skim`→`mac:net.sourceforge.skim-app.skim`; on Linux/BSD: `vscode`→`xdg:code.desktop`, `cursor`→`xdg:cursor.desktop`, `zed`→`xdg:dev.zed.Zed.desktop`, `sublime`→`xdg:sublime_text.desktop`.
- gloss's correction: `ntopenwith.HideFor([]ntopenwith.Verb{ntopenwith.Edit}, []string{"image/*"}, "mac:com.apple.QuickTimePlayerX")`.
- Source: `ntopenwith.Layered(system.System(), <aliases>, <correction>, ntopenwith.List(ntopenwith.TerminalEditor()), ntopenwith.List(ntopenwith.TerminalEditors()...))`.
- `Target.Kind` is gloss's canonical kind (`openwith.Kind`); `Target.Text` is `openwith.Editable(kind)`; a kind-only question (no path) uses `Ext` from `openwith.ExtOf(kind)`.
- Nothing launches in serve, pick-web, info, text, grep or export modes (`configureOpenWith` keeps `opts.OpenWith = nil` there), in previews, or in the browser.
- `--apps --json` keeps every existing field; `verb` is an addition (`"open"` or `"edit"`), one row per kind and verb.
- Untrusted names from the OS go through `safe()` before drawing (as today).

## Review Focus

1. A legacy default whose app is not installed (e.g. `skim`) falls back to the OS default with the note "saved app unavailable", it does not error. Test: Task 2 `TestLegacyDefaultUnavailableFallsBack`.
2. When both `open-with-v1.json` and `open-with.json` exist, v1 wins and the legacy file is never modified (byte-identical after a SetDefault). Test: Task 2 `TestV1WinsAndLegacyUntouched`.
3. A corrupt `open-with-v1.json` leaves open-with working with no saved choices, and SetDefault/SetEnabled refuse with "repair open-with-v1.json before saving" rather than overwrite it. Test: Task 2 `TestCorruptPreferencesRefuseSaving`.
4. `E` on a binary file (a PNG) with `$EDITOR` set never runs the terminal editor; with no editor app it says "No app can edit <name>". Test: Task 3 `TestEditBinaryNeverTerminal`.
5. Tab on a folder's chooser (Reveal only) says "Folders cannot be edited" and stays on Open; Tab while the chooser is busy does nothing. Test: Task 3 `TestTabOnFolderAndBusy`.

---

### Task 1: The Host over ntopenwith

**Files:**
- Modify: `go.mod`, `go.sum`, `examples/demo/go.mod`, `examples/demo/go.sum`, `THIRD_PARTY_NOTICES.md`
- Rewrite: `internal/openwith/types.go`, `internal/openwith/host.go`
- Create: `internal/openwith/source.go` (gloss's Layered source, aliases, correction), `internal/openwith/kinds.go` (move `Kinds`, `Kind`, `Editable` here; add `ExtOf`)
- Delete: `internal/openwith/catalog.go`, `catalog_test.go`, `launch.go`, `launch_test.go`, `editor.go`, `editor_test.go` (the `Editable` list moves to kinds.go)
- Modify (to compile, keeping today's behaviour on Open): `internal/app/open_with.go`, `internal/app/editor_native.go`, `internal/app/open_with_test.go`, `cmd/gloss/apps.go`, `cmd/gloss/apps_test.go` if present
- Test: `internal/openwith/host_test.go`, `internal/openwith/kinds_test.go`

**Interfaces:**
- Produces:
  - `type Verb = ntopenwith.Verb`; `const Open, Edit = ntopenwith.Open, ntopenwith.Edit`
  - `type Target struct { Path, Kind string; Directory, Temporary bool }` (unchanged fields)
  - `type Choice struct { ID, Name string; Default, Available, Terminal bool }`
  - `type Launched struct { Name string; Cmd *exec.Cmd }` — `Cmd` non-nil only for a terminal app, unstarted, for the pager to run attached.
  - `type Host interface { Choices(Target, Verb) ([]Choice, error); Launch(Target, Verb, string) (Launched, error); SetDefault(kind string, v Verb, id string) error }` — `Launch` with id `""` resolves; it returns errors wrapping `ntopenwith.ErrChoose` / `ErrNoEditor` unchanged.
  - `type AppPreference`, `type Manager` unchanged (`Apps() []AppPreference`, `SetEnabled(id string, enabled bool) error`).
  - `func NewHost(src ntopenwith.Source, prefs ntopenwith.Preferences, save func(ntopenwith.Preferences) error) Host` — Task 2 supplies `prefs` and `save`; tests pass a fake source, `ntopenwith.Preferences{}` and a recording `save`. The returned value also implements `Manager`.
  - `func Source() ntopenwith.Source` — the Global Constraints source for this OS.
  - `func ExtOf(kind string) string` — `image/*`→`.png`, `svg`→`.svg`, `pdf`→`.pdf`, `stl`→`.stl`, `3mf`→`.3mf`, `obj`→`.obj`, `ply`→`.ply`, `docx`→`.docx`, `xlsx`→`.xlsx`, `grist`→`.grist`, `csv`→`.csv`, `json`→`.json`, `ipynb`→`.ipynb`, `html`→`.html`, `text`→`.txt`, `markdown`→`.md`, `mermaid`→`.mmd`; unknown → `""`.

  Host behaviour: ntopenwith target is `{Path, Ext: ExtOf(kind) when Path == "", Kind, Directory, Text: Editable(kind)}`. `Choices(t, Open)` = `Resolver.Choices` plus a final `reveal` row (Name "Reveal in Finder" on darwin, "Reveal in Explorer" on windows, "Open containing folder" elsewhere); a directory gets only `reveal`. `Choices(t, Edit)` = `Resolver.Choices` (never `reveal`; a directory gets none). `Available` is true for every listed app. `Launch(t, v, "reveal")` → `ntopenwith.Reveal(t.Path)`, Name as listed. Otherwise `Launch` resolves (`id == ""`) or finds the listed app, calls `Resolver.Launch`, returns `Launched{Name, Cmd}`, and appends " (saved app unavailable)" to Name when the resolve result has `Fallback`. `SetDefault(kind, v, id)` → `Resolver.SetDefault` on `{Kind: kind, Ext: ExtOf(kind)}`, then `save(Preferences())`; `system` on Open clears. `Apps()` → `Resolver.AllApps` over `Kinds`/`ExtOf`, with `Enabled: !IsHidden`, sorted by Name. `SetEnabled` → `Resolver.SetHidden` then `save`.

- [ ] **Step 1: Write the failing tests** (`host_test.go`, with a test-only `fakeSource` implementing `ntopenwith.Source`: per-verb app lists, a default flag, a terminal app returning `exec.Command("true")`, recorded launches)

```go
func TestChoicesOpenEndsWithReveal(t *testing.T)    // Open: fake apps then "reveal"; directory → only "reveal"
func TestChoicesEditHasNoReveal(t *testing.T)
func TestLaunchResolvesDefault(t *testing.T)        // id "" → the source's default; Launched.Name is its Name
func TestLaunchTerminalReturnsCmd(t *testing.T)     // terminal app → Launched.Cmd != nil, Cmd.Process == nil
func TestLaunchEditChoose(t *testing.T)             // Edit with two apps, no default → errors.Is(err, ntopenwith.ErrChoose)
func TestSetDefaultSavesAndSystemClears(t *testing.T) // save called with Defaults[Edit]["pdf"]=="x"; then "system" on Open clears Defaults[Open]["pdf"]
func TestKindOnlyUsesExt(t *testing.T)              // Choices(Target{Kind:"pdf"}, Open) asks the source with Ext ".pdf"
func TestAppsAndSetEnabled(t *testing.T)            // Apps lists fake apps across kinds; SetEnabled(id,false) hides it from Choices and calls save
```

`kinds_test.go`: `TestExtOfEveryKind` — every entry of `Kinds` has a non-empty `ExtOf`.

- [ ] **Step 2: Run** `GOWORK=off go test ./internal/openwith/` — expect build failures (undefined `NewHost` signature, `Verb`, `Launched`, `ExtOf`).
- [ ] **Step 3: Add the dependency** `GOWORK=off go get nimbleterminal.dev/ntopenwith@v0.1.0`, then implement the files above and delete the listed ones. `Source()` builds aliases by `runtime.GOOS` (darwin; linux/freebsd/openbsd/netbsd) from the Global Constraints table.
- [ ] **Step 4: Adapt the callers so the tree builds with today's behaviour.** The pager calls `Choices(t, openwith.Open)`, `Launch(t, openwith.Open, id)` (using `Launched.Name`) and `SetDefault(kind, openwith.Open, id)`. The old `terminal-editor` row is gone from Open; a chosen row whose `Launched.Cmd` is non-nil goes into the existing editor handoff (`editor_native.go` takes the `*exec.Cmd` instead of calling `Editor`; remove `terminalEditorHost`). `cmd/gloss/apps.go`: `writeApps` asks with `openwith.Open`; `nativeOpenWith` returns `openwith.NewHost(openwith.Source(), ntopenwith.Preferences{}, func(ntopenwith.Preferences) error { return nil })` until Task 2. Adapt `fakeOpenHost`/`managingOpenHost` in `internal/app/open_with_test.go` to the new interface without changing what the tests assert.
- [ ] **Step 5: Run** `GOWORK=off go vet ./... && GOWORK=off go test ./internal/openwith/ ./internal/app/ ./cmd/gloss/` — PASS.
- [ ] **Step 6: Commit** `feat: build open-with on ntopenwith`

### Task 2: Preferences file and legacy migration

**Files:**
- Rewrite: `internal/openwith/preferences.go`
- Test: `internal/openwith/preferences_test.go` (replace)
- Modify: `cmd/gloss/apps.go` (`nativeOpenWith`)

**Interfaces:**
- Consumes: `NewHost`, `Source` (Task 1).
- Produces:
  - `func LoadPreferences(dir string) (prefs ntopenwith.Preferences, invalid error, err error)` — `open-with-v1.json` if it exists (a parse error is `invalid`, prefs empty); else `open-with.json` via `ntopenwith.LoadLegacyPreferences` (a parse error there is also `invalid`); neither → empty. `err` only for I/O other than not-exist.
  - `func Saver(dir string, invalid error) func(ntopenwith.Preferences) error` — returns `fmt.Errorf("repair open-with-v1.json before saving: %w", invalid)` when invalid came from v1; when invalid came from the legacy file, saving writes v1 (the legacy file is left as is); otherwise `Preferences.Save(filepath.Join(dir, "open-with-v1.json"))`.
  - `nativeOpenWith()` in cmd/gloss: `prefs, invalid, err := openwith.LoadPreferences(dir)`; on `err` return it; `return openwith.NewHost(openwith.Source(), prefs, openwith.Saver(dir, invalid)), invalid` (the invalid error is reported as today, not fatal).

- [ ] **Step 1: Write the failing tests**

```go
func TestLegacyMigrates(t *testing.T)                 // only open-with.json {"defaults":{"pdf":"skim"},"hidden":["zed"]} → prefs.Defaults[Open]["pdf"]=="skim", Hidden==["zed"]
func TestV1WinsAndLegacyUntouched(t *testing.T)       // both files; v1 read; after Saver(...)(p) the legacy file's bytes are unchanged and v1 holds p
func TestCorruptPreferencesRefuseSaving(t *testing.T) // v1 = "{" → invalid != nil, prefs empty; Saver returns error containing "repair open-with-v1.json"; v1 bytes unchanged
func TestCorruptLegacyStillSavesV1(t *testing.T)      // legacy = "{" → invalid != nil; Saver writes v1
func TestLegacyDefaultUnavailableFallsBack(t *testing.T) // NewHost(fakeSource without "skim", prefs from legacy {"pdf":"skim"}) → Launch(pdf, Open, "") uses the default, Name ends " (saved app unavailable)"
func TestLegacyAliasResolves(t *testing.T)            // fake source + ntopenwith.Alias("vscode","mac:x") layered; legacy {"markdown":"vscode"} → Launch(markdown, Open, "") launches "mac:x" without the fallback suffix
```

- [ ] **Step 2: Run** `GOWORK=off go test ./internal/openwith/` — FAIL.
- [ ] **Step 3: Implement** preferences.go and the `nativeOpenWith` change.
- [ ] **Step 4: Run** the package tests — PASS.
- [ ] **Step 5: Commit** `feat: keep saved open-with choices across the move to ntopenwith` (or together with Task 1).

### Task 3: The pager: verbs, E and Tab

**Files:**
- Modify: `internal/app/open_with.go`, `internal/app/open_with_js.go`, `internal/app/open_with_native.go`, `internal/app/keys.go:58-59`, the help screen's key list
- Test: `internal/app/open_with_test.go` (adapt `fakeOpenHost`/`managingOpenHost` to the new `Host`; keep every existing test's intent)

**Interfaces:**
- Consumes: `openwith.Host`, `Choice`, `Launched`, `Verb`, `Open`, `Edit`, `ntopenwith.ErrChoose`, `ntopenwith.ErrNoEditor`.
- Produces (pager behaviour):
  - `openWithPicker` gains `verb openwith.Verb`; `requestOpenWith(path, kind string, directory, chooser bool, verb openwith.Verb) tea.Cmd`; `openWithCurrent(chooser bool, verb openwith.Verb) tea.Cmd`.
  - keys.go: `w` → `openWithCurrent(false, Open)`, `W` → `openWithCurrent(true, Open)`, `E` → `openWithCurrent(false, Edit)`, all only at `layerNone` as today.
  - A non-chooser launch that returns `ErrChoose` opens the chooser for that verb (same file, layerOpenWith); `ErrNoEditor` → note `"No app can edit " + safe(base)`; other errors → note `"Open with: …"` / `"Edit with: …"` by verb.
  - Chooser header `"Open with — <file>"` / `"Edit with — <file>"`; rows show `(default)` and, for terminal apps, `(terminal)`. Copy path is appended to Open only (not Edit, not folders).
  - Tab (not busy): switch verb and reload `Choices` for the same target; on a directory note `"Folders cannot be edited"` and stay; if the other verb lists nothing, note `"No app can edit <file>"` and stay.
  - Enter on a row: `host.Launch(target, verb, id)`; a `Launched.Cmd` goes into the editor handoff Task 1 adapted. `d` saves `host.SetDefault(target.Kind, verb, id)`; refuse `copy`, `reveal`, empty kind ("This action cannot be a default"). Saved note: `"Default saved for <kind> (<verb>)"`.
  - After launch note: `"Opened in <Name>"` (Open) / `"Editing in <Name>"` (Edit); terminal: `"Editor closed"` as today; temporary suffixes as today.
  - Status hint: `" ↑/↓ choose · Enter open · Tab edit with · d default · Esc close"` (Open) / `" ↑/↓ choose · Enter edit · Tab open with · d default · Esc close"` (Edit), with `· m manage` inserted before `Esc` when the host is a Manager, as today.
  - `?` help / guide text for `E` and Tab: update the help screen's key list where `w`/`W` are listed.

- [ ] **Step 1: Write/adapt the failing tests**

```go
func TestEditKeyLaunchesEditDefault(t *testing.T)   // E on note.md: fake Launch receives verb Edit, id ""; note "Editing in Editor"
func TestEditChooseOpensChooser(t *testing.T)       // fake Launch(Edit,"") returns ntopenwith.ErrChoose → layerOpenWith, header "Edit with — note.md"
func TestTabSwitchesVerb(t *testing.T)              // W → "Open with — note.md"; Tab → "Edit with — note.md", fake Choices got Edit; Tab → back to Open
func TestTabOnFolderAndBusy(t *testing.T)           // folder chooser: Tab → note "Folders cannot be edited", still Open; busy picker: Tab ignored
func TestEditBinaryNeverTerminal(t *testing.T)      // fake Launch(Edit,"") on a .png returns ErrNoEditor → note "No app can edit pic.png", no ExecProcess
func TestTerminalAppRunsAttached(t *testing.T)      // fake Launch returns Launched{Cmd: exec.Command("true")} → the returned tea.Cmd is the ExecProcess handoff; editorFinished reloads
func TestSaveDefaultPerVerb(t *testing.T)           // Tab to Edit, down, d → fake saved "markdown:edit:editor"
```

Keep the existing tests (picker and launch, Copy path, Reveal for folders, temporary notes, manage apps, preview has no keys, browser host nil → "not available") passing against the adapted fakes.

- [ ] **Step 2: Run** `GOWORK=off go test ./internal/app/ -run 'OpenWith|Edit|Tab|Terminal|SaveDefault'` — FAIL.
- [ ] **Step 3: Implement** the pager changes.
- [ ] **Step 4: Run** `GOWORK=off go test ./internal/app/` — PASS.
- [ ] **Step 5: Commit** `feat: E edits, and Tab switches the chooser between Open with and Edit with`

### Task 4: `--apps` with verbs

**Files:**
- Modify: `cmd/gloss/apps.go` (`listedApp`, `appsListing`, `writeApps`), `cmd/gloss/schema.go` if the description names formats only
- Test: `cmd/gloss/apps_test.go`, `cmd/gloss/schema_test.go` (golden/expectations)

**Interfaces:**
- Consumes: `Host.Choices(Target{Kind: k}, v)`.
- Produces: `appsListing` gains ``Verb string `json:"verb" doc:"Which action the row is for." enum:"open,edit"` `` placed after `Kind`; `listedApp` gains ``Terminal bool `json:"terminal" doc:"Runs in the terminal, which it takes over until it exits."` ``. `writeApps` emits, for each kind in `Kinds`, an `open` row then an `edit` row. Text output: `"<kind> <verb> (default: <id>)"`. The `listedApp.ID` doc becomes "Stable app identifier: system, reveal, terminal-editor, mac:…, xdg:…, win:…". `reveal` appears in open rows only.

- [ ] **Step 1: Write the failing tests**: `TestAppsJSONHasVerbs` (fake host; every kind has exactly one `open` and one `edit` row in that order; `edit` rows have no `reveal`; a terminal app has `"terminal": true`); update the schema test for the new fields.
- [ ] **Step 2: Run** `GOWORK=off go test ./cmd/gloss/ -run 'Apps|Schema'` — FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `GOWORK=off go test ./cmd/gloss/` — PASS.
- [ ] **Step 5: Commit** `feat: say which verb each --apps row is for`

### Task 5: Documentation, notices, and the gate

**Files:**
- Modify: `AGENTS.md` ("Native open-with" paragraph and the `--apps` bullet), `DEVELOP.md` (internal/openwith description), `docs/hugo/content/guide/using.md` and `browser.md` (keys `w`, `W`, `E`, Tab, `d`, `m`, Alt+Enter), `skills/gloss/SKILL.md` (`--apps` JSON fields), `CHANGELOG.md` (Unreleased), `THIRD_PARTY_NOTICES.md` (`task notices`), generated docs (`task docs`)

- [ ] **Step 1:** Update the docs to say: apps come from the OS (Launch Services, XDG, Windows registry) through ntopenwith; `E` edits (text files default to `$VISUAL`/`$EDITOR`, other files to the OS's default editor, else the chooser); Tab switches verbs; defaults are saved per kind and verb in `open-with-v1.json`; the old `open-with.json` is read once and left untouched; `--apps --json` rows carry `verb` and `terminal`. CHANGELOG lines: "Open with now lists the apps your system knows for each file, with its defaults" and "`E` edits a file in its editor; Tab in the chooser switches between Open with and Edit with; saved choices carry over."
- [ ] **Step 2: Run** `GOWORK=off task notices && GOWORK=off task docs && GOWORK=off task ci && GOWORK=off task demo-check` — all PASS.
- [ ] **Step 3: Manual check on this Mac:** `go run ./cmd/gloss --apps` lists Preview for pdf/image open rows and `GOWORK=off go run ./cmd/gloss examples/landscape.png` then `W` shows the system's apps, Tab shows "Edit with" without QuickTime; Esc. (No launch needed.)
- [ ] **Step 4: Commit** `docs: open-with on ntopenwith, E and Tab`
