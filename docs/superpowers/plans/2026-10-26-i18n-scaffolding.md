# i18n Scaffolding Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give gloss an `internal/i18n` package with English, Spanish and Simplified Chinese catalogs, a `--lang` option, a locale handed to renderers, and a first translated slice (help, status notes, handoff screen and pages, two guide pages).

**Architecture:** go-i18n v2 bundle over embedded TOML catalogs, with English defaults inline at each call (`i18n.T(id, default)`), as in go-thinkt. `cmd/gloss` resolves one user tag (`--lang` > env for the interactive viewer, `en` for headless, the browser's `Accept-Language` for detached handoff pages) and passes it as `app.Options.Lang`; the app turns it into a `document.Locale` on each `Request`. `internal/document` imports neither `internal/i18n` nor the environment.

**Tech Stack:** Go 1.26, `github.com/nicksnyder/go-i18n/v2` v2.6.1, `github.com/BurntSushi/toml` v1.6.0, `golang.org/x/text/language` and `golang.org/x/text/message`, html/template, Hugo multilingual (hugo-book).

**Spec:** `docs/superpowers/specs/2026-10-26-i18n-scaffolding-design.md`

## Global Constraints

- Catalog tags are exactly `en`, `es`, `zh-Hans`; English lives in source defaults, so there is no `en.toml`.
- Option names, option values, JSON keys, enum values, exit codes, `--info` field labels (they are JSON keys) and OSC 7501 program-status messages never change with the language.
- Headless modes (`--output`, `--output-dir`, `--print`, `--text`, `--info`, `--grep`, `--apps`, `skill`) use `en` unless `--lang` is given and never read `GLOSS_LANG`/`LC_*`/`LANG`.
- Interactive resolution: `--lang` > `GLOSS_LANG` > `LC_ALL` > `LC_MESSAGES` > `LANG` > `en`; unparseable env values are skipped, never an error.
- `--lang` that does not parse is a usage error (exit 1); a valid tag without a catalog shows English text and formats numbers by the tag.
- Catalog match requires `language.High` confidence or better (`es-MX`→`es`, `zh-CN`/`zh`/`zh-SG`→`zh-Hans`, `zh-TW`/`zh-Hant`→`en`, `fr`→`en`).
- Message IDs are literal, dotted, grouped `app.*`, `handoff.*`, `web.*`, `cmd.*`.
- Display widths are measured with `ansi.StringWidth`, never `len`.
- `es.toml` and `zh-Hans.toml` start with `# Draft translation, awaiting native review.`
- `task ci` stays green; `GOOS=js` and Windows builds still pass; notices regenerated.
- The browser demo (`examples/demo`) stays English this round.

## Review Focus

1. **Odd locale environments** (`LANG=C.UTF-8`, `LANG=`, `LC_ALL=zh_CN.GB18030`, `LANG=en_US.UTF-8@euro`, `LANG=garbage!!`): the viewer starts, in `en` or the right catalog, with no error. Pinned in Task 1 (`TestFromEnv`).
2. **Agent output under a foreign `LANG`**: `--info --json`, `--text --json` and export manifests are byte-identical with `LANG=es_ES.UTF-8` and `LANG=C`. Pinned in Task 3 (`TestHeadlessIgnoresLANG`).
3. **Double-width text in narrow status lines**: the zh-Hans search status at width 40 never exceeds 40 cells. Pinned in Task 6 (`TestSearchStatusFitsInChinese`).
4. **Translations whose `%` verbs don't match the default** print `%!d(string=…)`. Pinned in Task 2 (`TestCatalogVerbsMatch`).
5. **Hostile or odd `Accept-Language`** (`*`, `xx;q=abc`, 10 KiB of junk, `zh-CN;q=0.1, es;q=0.9`): the page renders, picks `es` in the last case, falls back to `en` otherwise, and the header never reaches the HTML except as the matched tag. Pinned in Task 7 (`TestPageLanguageFromHeader`).

---

### Task 1: The `internal/i18n` package

**Files:**
- Create: `internal/i18n/i18n.go`, `internal/i18n/locale.go`, `internal/i18n/locales/es.toml`, `internal/i18n/locales/zh-Hans.toml`
- Test: `internal/i18n/i18n_test.go`, `internal/i18n/locale_test.go`
- Modify: `go.mod`, `go.sum`, `THIRD_PARTY_NOTICES.md` (via `task notices`)

**Interfaces:**
- Produces (locale.go):
  - `func Parse(s string) (language.Tag, error)`: accepts BCP 47 (`es-MX`, `zh-Hans`) and POSIX (`zh_CN.UTF-8`, `es_ES@euro`); strips `.codeset` and `@modifier`, maps `_`→`-`; `C` and `POSIX` give `language.English`; empty or unparseable is an error.
  - `func FromEnv(getenv func(string) string) string`: the first of `GLOSS_LANG`, `LC_ALL`, `LC_MESSAGES`, `LANG` that `Parse`s, as `tag.String()`; else `"en"`.
  - `func FromAcceptLanguage(header string) string`: the best user tag that matches a catalog with High confidence, as `tag.String()` (e.g. `"es-MX"`); else `"en"`. Ignores headers over 4 KiB.
  - `func Match(tag string) string`: `"en"`, `"es"` or `"zh-Hans"`.
- Produces (i18n.go):
  - `func SetLanguage(tag string)`: sets the process-wide localizer to `Match(tag)`; `""` means `en`. Safe for concurrent use with `T`.
  - `func Language() string`: the active catalog tag.
  - `func T(id, def string) string`, `func Tf(id, def string, args ...any) string`, `func Tn(id, one, other string, n int) string`: on the active localizer. `one`/`other` are templates using `{{.Count}}`; with no localizer or a missing ID, `Tn` executes the English template, never `"%d %s"`.
  - `type Localizer struct` (unexported fields); `func For(tag string) *Localizer`; methods `T`, `Tf`, `Tn` (same signatures as above), `Tag() string`, `Messages(prefix string) map[string]string` (the catalog's own `other` strings for IDs with that prefix; empty for `en`).
  - `type Lang struct{ Tag, Name, English string }`; `func Languages() []Lang`: `en`, `es` (`Español`), `zh-Hans` (`简体中文`), in that order.
- The bundle is built once (`sync.Once`) from `//go:embed locales/*.toml`.

- [ ] **Step 1: Write the failing tests**

```go
func TestParse(t *testing.T) {
	for in, want := range map[string]string{
		"es-MX": "es-MX", "zh_CN.UTF-8": "zh-CN", "es_ES@euro": "es-ES",
		"C": "en", "POSIX": "en", "C.UTF-8": "en", "zh-Hans": "zh-Hans",
	} { /* Parse(in).String() == want */ }
	for _, bad := range []string{"", "garbage!!", "en_US.UTF-8@@"} { /* Parse(bad) errors */ }
}

func TestFromEnv(t *testing.T) {
	// map-backed getenv
	// {LANG: "es_ES.UTF-8"} -> "es-ES"
	// {LANG: "es_ES.UTF-8", LC_MESSAGES: "zh_CN.UTF-8"} -> "zh-CN"
	// {LANG: "es_ES", LC_ALL: "C"} -> "en"
	// {GLOSS_LANG: "zh-Hans", LC_ALL: "es"} -> "zh-Hans"
	// {LANG: "garbage!!", LC_MESSAGES: ""} -> "en"
	// {LC_ALL: "zh_CN.GB18030"} -> "zh-CN"
	// {} -> "en"
}

func TestMatch(t *testing.T) {
	// es-MX, es-419, es -> es; zh-CN, zh, zh-SG, zh-Hans -> zh-Hans;
	// zh-TW, zh-Hant, fr, de-DE, en-GB, "" -> en
}

func TestTFallsBackToDefault(t *testing.T) {
	SetLanguage("es")
	if got := T("test.missing", "Hello"); got != "Hello" { t.Fatal(got) }
}

func TestTnEnglishTemplate(t *testing.T) {
	SetLanguage("en")
	// Tn("test.files", "{{.Count}} file", "{{.Count}} files", 1) == "1 file"; 3 -> "3 files"
}

func TestTranslated(t *testing.T) {
	// For("es").T("app.help.title", "a visual pager") != "a visual pager"
	// For("zh-Hans").T(...) likewise; For("en") returns the default.
	// For("es").Messages("web.") contains only keys starting "web."
}
```

- [ ] **Step 2: Run them, expect failure**

Run: `go test ./internal/i18n/`
Expected: FAIL, package does not exist.

- [ ] **Step 3: Add dependencies and implement**

`go get github.com/nicksnyder/go-i18n/v2@v2.6.1 github.com/BurntSushi/toml@v1.6.0 golang.org/x/text`. `Match` uses `language.NewMatcher([]language.Tag{English, Spanish, SimplifiedChinese})` and returns `en` below `language.High`. Seed both TOML files with the draft header and `app.help.title` only; later tasks add their own IDs.

- [ ] **Step 4: Run tests, expect pass**

Run: `go test ./internal/i18n/ && GOOS=js GOARCH=wasm go build ./internal/i18n/ && GOOS=windows go build ./internal/i18n/`
Expected: PASS, builds succeed.

- [ ] **Step 5: Regenerate notices, commit**

```bash
task notices
git add internal/i18n go.mod go.sum THIRD_PARTY_NOTICES.md
git commit -m "feat: internal/i18n: go-i18n catalogs for en, es and zh-Hans, locale parsing and matching"
```

---

### Task 2: Coverage test and `task i18n:coverage`

**Files:**
- Create: `internal/i18n/coverage_test.go`
- Modify: `Taskfile.yml` (add `i18n:coverage`)

**Interfaces:**
- Consumes: the catalogs from Task 1.
- Scans `cmd/`, `internal/` (`*.go`, skipping `_test.go`) and `web/` (`*.html`, `*.mjs`, skipping `*.test.mjs` and `dist/`) from the module root (`../..`). It recognizes:
  - Go: `T("id", "def")`, `Tf("id", "def"`, `Tn("id", "one", "other"`, with or without a receiver or package qualifier;
  - templates: `{{t "id" "def"}}`;
  - JS: `msg("id", "def"`.
  - IDs must match `^[a-z][A-Za-z0-9]*(\.[A-Za-z0-9]+)+$`.

- [ ] **Step 1: Write the tests**

- `TestCatalogCoverage`: every scanned ID is present in `es.toml` and `zh-Hans.toml`. It fails, listing the missing IDs per file. This differs from go-thinkt, where the test only logs.
- `TestCatalogNoStale`: every catalog ID is used somewhere in the scan.
- `TestCatalogVerbsMatch`: for each `Tf` call, each translation's `%` verbs (by `regexp.MustCompile("%[-+# 0]*[0-9]*(\\.[0-9]+)?[a-zA-Z%]")`) equal the default's as a multiset. For `Tn`, the `{{.Count}}` presence matches.
- `TestCatalogPlurals`: each `Tn` ID has `one` and `other` in `es`, and `other` in `zh-Hans`.
- `TestScannerFindsAllForms`: run the scanner on an in-memory sample of each form and assert the IDs it finds.

- [ ] **Step 2: Run them**

Run: `go test ./internal/i18n/ -run Catalog -v`
Expected: PASS. The only ID so far is `app.help.title`, and both files have it. To check the test can fail, temporarily delete it from `es.toml`: expect FAIL naming `app.help.title`. Then restore it.

- [ ] **Step 3: Add the task**

```yaml
  i18n:coverage:
    desc: Fail when a translation is missing, stale, or has different format verbs
    cmds: ['go test ./internal/i18n/ -run Catalog -v']
```

- [ ] **Step 4: Commit**

```bash
git add internal/i18n/coverage_test.go Taskfile.yml
git commit -m "test: i18n coverage fails on missing, stale or mismatched translations"
```

---

### Task 3: `--lang` and where the language comes from

**Files:**
- Modify: `cmd/gloss/main.go` (flag, validation, resolution in `run`), `cmd/gloss/domains.go` (`lang` in the `view` domain's `Flags`), `internal/app/model.go` (`Options.Lang`), `skills/gloss/SKILL.md`
- Create: `cmd/gloss/lang.go`
- Test: `cmd/gloss/lang_test.go`

**Interfaces:**
- Consumes: `i18n.Parse`, `i18n.FromEnv`, `i18n.SetLanguage`.
- Produces:
  - `app.Options.Lang string`: the user tag (`"es-MX"`); `""` means English, and leaves handoff pages to the browser.
  - `func language(opts options, stdinTTY bool, getenv func(string) string) string` in `lang.go`:
    - returns `opts.Lang` when set;
    - returns `""` for headless (exporting, `Info`, `Apps`, `Grep`, `Text`, `Print`, `SkillDetect`/`SkillInstall`);
    - returns `""` for a detached start or server (`opts.Detached != ""`, or `(Pick||Serve||PickWeb) && !stdinTTY`);
    - else returns `i18n.FromEnv(getenv)`.
- The flag: `f.StringVar(&opts.Lang, "lang", "", "language of gloss's own words and number formats (BCP 47 or POSIX, e.g. es, zh-Hans, de_DE); options stay English")`. In `parse`, a set value that `i18n.Parse` rejects returns `fmt.Errorf("--lang %q: not a language tag", v)`, and the value is stored canonicalized.
- In `run`, after the detach decision: `lang := language(opts, stdinTTY, os.Getenv); opts.Options.Lang = lang; i18n.SetLanguage(lang)`. A detached start re-executes with its own arguments, so `--lang` reaches the server unchanged.

- [ ] **Step 1: Write the failing tests**

- `TestLangValidation`: `parse([]string{"--lang", "garbage!!", "x.png"})` errors with "not a language tag", and `--lang zh_CN.UTF-8` stores `zh-CN`.
- `TestLanguageResolution`, a table over `language()`:
  - `--info` with getenv `LANG=es_ES` gives `""`;
  - `--info --lang es` gives `"es"`;
  - the viewer with a TTY and `LANG=es_ES` gives `"es-ES"`;
  - `--serve` with no TTY and `LANG=es_ES` gives `""`;
  - `--serve` with a TTY and `LANG=es_ES` gives `"es-ES"`;
  - `Detached` set gives `""`.
- `TestHeadlessIgnoresLANG`: runs the built binary (the pattern `cmd/gloss` tests already use for end-to-end runs) on `examples/` fixtures: `--info --json` on a CSV and a PNG, `--text --json` on a Markdown file, and an `--output-dir --json` export. With `LANG=es_ES.UTF-8 LC_ALL=` and with `LANG=C`, stdout must be byte-identical, apart from the export output path.
- `TestLangHasDomain`: covered by the existing `checkDomains` test once `lang` is added.

- [ ] **Step 2: Run them, expect failure**

Run: `go test ./cmd/gloss/ -run 'Lang|Headless'`
Expected: FAIL, unknown flag `--lang`.

- [ ] **Step 3: Implement as in Interfaces**

- [ ] **Step 4: Document the option in SKILL.md** (English)

Add one paragraph under Guardrails: headless output is English and `.`-decimal unless `--lang`; JSON keys, enums and exit codes never change; a bad `--lang` exits 1; detached pages follow the human's browser.

- [ ] **Step 5: Run tests, expect pass**

Run: `go test ./cmd/gloss/ ./internal/app/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add cmd/gloss internal/app/model.go skills/gloss/SKILL.md
git commit -m "feat: --lang, and the viewer's language from GLOSS_LANG, LC_ALL, LC_MESSAGES or LANG; headless stays English"
```

---

### Task 4: `document.Locale` and localized counts in the info box

**Files:**
- Create: `internal/document/locale.go`
- Modify: `internal/document/loader.go` (`Request.Locale`; pass it to `fileFields`), `internal/document/info.go` (`fileFields` takes `Locale`), `internal/document/csv.go:115` (Table rows and columns), `internal/document/pmtiles.go:95`, `internal/app/theme.go` or a new `internal/app/locale.go` (`DocumentLocale`), `internal/app/model.go:408` (set `Locale`), `cmd/gloss/export.go:21` and the info/print request sites (set `Locale: app.DocumentLocale(opts.Lang)`)
- Test: `internal/document/locale_test.go`

**Interfaces:**
- Produces:
  - `type Locale struct{ Tag string }`: the zero value formats exactly as today.
  - `func (l Locale) Count(n int) string`: grouped digits by `golang.org/x/text/message`. With `Tag == ""` it returns `grouped(n)`.
  - `func (l Locale) Bytes(n int) string`: `byteSize`'s shape with localized numbers. The units (`GiB`, `MiB`, `KiB`) and the word `bytes` stay as they are this round.
  - `func DocumentLocale(tag string) document.Locale` in `internal/app`.
- `grouped` and `byteSize` stay for error messages, which stay English.

- [ ] **Step 1: Write the failing tests**

```go
func TestLocaleCount(t *testing.T) {
	// Locale{}.Count(1234567) == "1,234,567"
	// Locale{"es-ES"}.Count(1234567) == "1.234.567"
	// Locale{"de-DE"}.Count(1234567) == "1.234.567"
	// Locale{"zh-Hans"}.Count(1234567) == "1,234,567"
}
func TestLocaleBytes(t *testing.T) {
	// Locale{}.Bytes(1572864) == byteSize(1572864) == "1.5 MiB (1,572,864 bytes)"
	// Locale{"es-ES"}.Bytes(1572864) == "1,5 MiB (1.572.864 bytes)"
	// Locale{}.Bytes(1) == "1 byte"
}
func TestInfoUsesRequestLocale(t *testing.T) {
	// Load a CSV fixture with Request{Locale: Locale{"es-ES"}}; File/Size uses ","
	// as the decimal mark, Table/Rows is grouped by "." when ≥ 1000 rows
	// (build a 1,200-row CSV in t.TempDir()). The zero Locale gives today's strings.
}
```

- [ ] **Step 2: Run them, expect failure**

Run: `go test ./internal/document/ -run Locale`
Expected: FAIL, `Locale` undefined.

- [ ] **Step 3: Implement as in Interfaces, and set `Locale` at every `Request` the app and cmd build**

- [ ] **Step 4: Run tests, expect pass**

Run: `go test ./internal/document/ ./internal/app/ ./cmd/gloss/`
Expected: PASS. `TestHeadlessIgnoresLANG` from Task 3 still passes.

- [ ] **Step 5: Commit**

```bash
git add internal/document internal/app cmd/gloss
git commit -m "feat: document.Locale: the info box groups digits and marks decimals by the user's language"
```

---

### Task 5: Translated help screen

**Files:**
- Modify: `internal/app/help.go`. `helpLeft` and `helpRight` become `func helpLeft() []helpSection` and `func helpRight() []helpSection`, built with `i18n.T`. The heading `" — a visual pager"`, `"? or Esc closes"` and the footer `"gloss --help lists options · docs: "` are translated, and so is `helpScreen()`. `browserHelpLeft` and `browserHelpRight` stay English this round. Update callers and the existing tests.
- Modify: `internal/i18n/locales/{es,zh-Hans}.toml`
- Test: `internal/app/help_test.go`

**Interfaces:**
- Consumes: `i18n.SetLanguage`, `i18n.T`.
- IDs: `app.help.<section>.title`, `app.help.<section>.<n>` for each action, `app.help.title`, `app.help.closes`, `app.help.footer`. Key glyphs (`q  Q`, `Esc`) are not translated.

- [ ] **Step 1: Write the failing test**

```go
func TestHelpFitsInEveryLanguage(t *testing.T) {
	for _, tag := range []string{"en", "es", "zh-Hans"} {
		i18n.SetLanguage(tag)
		t.Cleanup(func() { i18n.SetLanguage("en") })
		col := (80 - 4) / 2
		for _, s := range append(helpLeft(), helpRight()...) {
			// ansi.StringWidth(s.title) <= col
			// rows with keys: ansi.StringWidth(r.action) <= col-helpKeyWidth-1
			// notes: ansi.StringWidth(r.action) <= col
		}
		// m.helpView(80, 24) has no "…" in it, and its heading row's width is <= 80
	}
	// es and zh-Hans help differ from en (translations are actually used)
}
```

- [ ] **Step 2: Run it, expect failure**

Run: `go test ./internal/app/ -run Help`
Expected: FAIL, `helpLeft` is not a function.

- [ ] **Step 3: Convert to functions and add the translations.** Write them to the width budget. Shorten a translation rather than change the layout.

- [ ] **Step 4: Run tests, expect pass**

Run: `go test ./internal/app/ -run Help && go test ./internal/i18n/`
Expected: PASS. Coverage includes the new IDs.

- [ ] **Step 5: Commit**

```bash
git add internal/app/help.go internal/app/help_test.go internal/i18n/locales
git commit -m "feat: the help screen in Spanish and Simplified Chinese, fitted to 80×24"
```

---

### Task 6: Status notes and the handoff screen

**Files:**
- Modify:
  - `internal/app/view.go:96`: `"Loading %s…"`.
  - `internal/app/search.go:395-415`: `Enter search · Esc cancel`, `searching… · Esc cancel`, `no matches`, the `s.unit` words, `n/N · Esc clear`.
  - `internal/app/export.go:65,91,112,136`.
  - `internal/app/handoff.go:104-120`: the screen lines only. The `report(...)` program-status strings at `:134-142` and in `status.go` stay English.
- Modify: `internal/i18n/locales/{es,zh-Hans}.toml`
- Test: `internal/app/search_test.go`, `internal/app/handoff_test.go`

**Interfaces:**
- Consumes: `i18n.T`, `i18n.Tf`.
- IDs: `app.loading`, `app.search.prompt`, `app.search.running`, `app.search.none`, `app.search.unit.<unit>`, `app.search.keys`, `app.export.running`, `app.export.textView`, `app.export.sheet`, `handoff.screen.enlarge`, `handoff.screen.scan`, `handoff.screen.closing`, `handoff.screen.waiting`.
- Every truncation already in these paths stays on `ansi.Truncate` and display width.

- [ ] **Step 1: Write the failing tests**

- `TestSearchStatusFitsInChinese`: with `zh-Hans`, a search model with matches and one without. At widths 20, 40 and 80, `ansi.StringWidth(status) <= width`, and the text contains `"无匹配"` (or whatever the catalog's translation of `app.search.none` is, read through `i18n.For("zh-Hans").T`).
- `TestHandoffScreenTranslated`: with `es`, the handoff view contains `i18n.For("es").T("handoff.screen.waiting", …)`.
- `TestProgramStatusStaysEnglish`: with `es`, the status record still says `"Waiting for files"`.

- [ ] **Step 2: Run them, expect failure**

Run: `go test ./internal/app/ -run 'SearchStatusFits|HandoffScreenTranslated|ProgramStatusStaysEnglish'`
Expected: the first two FAIL.

- [ ] **Step 3: Replace the strings with `i18n.T`/`Tf` and add the translations**

- [ ] **Step 4: Run tests, expect pass**

Run: `go test ./internal/app/ ./internal/i18n/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app internal/i18n/locales
git commit -m "feat: status notes, search line and handoff screen in Spanish and Simplified Chinese"
```

---

### Task 7: The `--serve` and `--pick-web` pages

**Files:**
- Modify:
  - `internal/handoff/serve.go:46,322`: template funcs, page data, and the session's language at `:159`.
  - `internal/handoff/pick_web.go:28,176`.
  - `web/serve.html`, `web/pick.html`: `<html lang="{{.Lang}}">`, chrome via `{{t "id" "default"}}`, and `<script type="application/json" id="gloss-messages">{{.Messages}}</script>`.
  - `web/pick.mjs`, `web/serve.mjs`: visible strings through `msg`.
- Create: `web/i18n.mjs`, `web/i18n.test.mjs`
- Test: `internal/handoff/i18n_test.go`

**Interfaces:**
- Consumes: `i18n.For`, `i18n.FromAcceptLanguage`, `i18n.Match`, `Localizer.Messages("web.")`, `i18n.SetLanguage`, `app.Options.Lang`.
- Page language: `cfg.Options.Lang` if set, else `i18n.FromAcceptLanguage(r.Header.Get("Accept-Language"))`. The server keeps the last page's tag. When a viewer session starts with `opts.Lang == ""`, it sets `opts.Lang` to that tag and calls `i18n.SetLanguage`, so a detached viewer's pager matches its page.
- Template data gains `Lang string` (the `Match`ed catalog tag) and `Messages template.JS` (`json.Marshal` of `Messages("web.")`; html/template escapes it).
- Template func: `"t": func(id, def string) string` bound to the request's localizer. Templates are parsed once, with the func map, and cloned per language via `template.Clone` + `Funcs`.
- `web/i18n.mjs`: `export function msg(id, fallback, vars = {})`. It reads the `#gloss-messages` JSON once, returns the translation or the fallback, and replaces `{{.Name}}` with `vars.Name`. A missing or invalid element means the fallbacks.
- IDs: `web.serve.*`, `web.pick.*`. These cover the headings, the **Choose files** / **Done viewing** / **Open another file** / **Send files** / **Cancel** / **Remove** buttons, the **Message to requester** label, the limits line, and the drop overlay. Startup/status/resume JSON is untouched.

- [ ] **Step 1: Write the failing tests**

- `TestPageLanguageFromHeader`, run against a `--serve` page and a `--pick-web` page, using `handofftest` or the existing serve test helpers:
  - `Accept-Language: es-MX,es;q=0.9` gives `lang="es"` and the Spanish Done-viewing label;
  - `zh-CN;q=0.1, es;q=0.9` gives `lang="es"`;
  - `*`, `xx;q=abc`, an empty header, and 10 KiB of `a,` all give `lang="en"` and status 200;
  - the raw header text never appears in the body.
- `TestPageLanguageFromConfig`: with `cfg.Options.Lang = "zh-CN"`, `Accept-Language: es` still gives `lang="zh-Hans"`.
- `TestMessagesAreEscaped`: a stubbed message value containing `</script>` (inject it through a test seam on the message map) doesn't end the script element.
- `web/i18n.test.mjs`:
  - `msg` returns the fallback with no element;
  - it returns the translation from a JSON element;
  - it substitutes `{{.Name}}`;
  - invalid JSON falls back.

- [ ] **Step 2: Run them, expect failure**

Run: `go test ./internal/handoff/ -run 'PageLanguage|MessagesAreEscaped' && node --test web/i18n.test.mjs`
Expected: FAIL.

- [ ] **Step 3: Implement as in Interfaces and add the `web.*` translations**

- [ ] **Step 4: Run tests, expect pass**

Run: `go test ./internal/handoff/ ./internal/i18n/ && task web-check`
Expected: PASS. The existing handoff tests still pass, because English pages are unchanged text.

- [ ] **Step 5: Commit**

```bash
git add internal/handoff web internal/i18n/locales
git commit -m "feat: --serve and --pick-web pages speak the terminal's language, or the browser's when detached"
```

---

### Task 8: Multilingual docs site

**Files:**
- Modify: `docs/hugo/hugo.toml`. Add `defaultContentLanguage = "en"`, `defaultContentLanguageInSubdir = false`, and a `[languages]` table for `en` (English), `es` (Español) and `zh-hans` (简体中文, `languageCode = "zh-Hans"`). Each gets `weight` and `languageName`, and its own `[[module.mounts]]` with `lang`: `content` → en, `content.es` → es, `content.zh-hans` → zh-hans.
- Create: `docs/hugo/content.es/_index.md`, `docs/hugo/content.es/guide/_index.md`, `docs/hugo/content.es/guide/using.md`, and the same three under `docs/hugo/content.zh-hans/`. Each translated page's front matter has the same `weight` as the English one, plus a one-line draft-translation note.
- Modify: `internal/tools/docsite/main_test.go`
- Test: the new `TestTranslatedGuideLinksResolve` there

**Interfaces:**
- Links in a translated `guide/_index.md` point to the translated page when one exists. Otherwise they point to the English page by its site path (`/gloss/docs/guide/controls/`), and the entry is marked `(English)` / `（英文）`.
- Generated pages (`command`, `for-llms`, `development`) are never translated.

- [ ] **Step 1: Write the failing test**

- `TestTranslatedGuideLinksResolve`: for each `content.<lang>` folder, every Markdown link in it resolves either to a file under that folder or, for `/gloss/docs/...` paths, to a file under `content/`.
- The same test checks that each translated page has an English counterpart with the same `weight`.

- [ ] **Step 2: Run it, expect failure**

Run: `go test ./internal/tools/docsite/ -run Translated`
Expected: FAIL, no `content.es` folder.

- [ ] **Step 3: Add the config and the six pages**

- [ ] **Step 4: Verify**

Run: `go test ./internal/tools/docsite/ && task docs-check`. Also run `task docs:hugo:build` if `hugo` is installed.
Expected: PASS. Hugo builds `/es/guide/using/` and `/zh-hans/guide/using/`, and the hugo-book language menu lists all three languages. If Hugo isn't installed, say that the check was skipped.

- [ ] **Step 5: Commit**

```bash
git add docs/hugo internal/tools/docsite
git commit -m "docs: the guide's index and Using gloss in Spanish and Simplified Chinese"
```

---

### Task 9: Developer docs, changelog, full gate

**Files:**
- Modify:
  - `DEVELOP.md`: a new "Internationalization" section covering how to add a string (`i18n.T` with a literal dotted ID and an English default), how to add a language (a TOML file, `Languages()`, `Match`'s supported list), the coverage task, the rules from Global Constraints, and the library gaps (ntcharts/Flint, ntmaps, nttypeset, ntvideo, ntfilepicker and ntopenwith take no locale yet; typeset fonts have no CJK).
  - `AGENTS.md`: one bullet in Repository layout for `internal/i18n`, and a Capabilities paragraph on languages and `--lang`.
  - `CHANGELOG.md` (Unreleased): one line.
  - `docs/hugo/content/guide/using.md`: a short Languages section with `GLOSS_LANG` and `--lang`. Mirror it in the translated `using.md` from Task 8.
- Run: `task docs`

- [ ] **Step 1: Write the docs**

- [ ] **Step 2: Regenerate and run the gate**

Run: `task docs && task ci`
Expected: all steps pass. If staticcheck fails on Go 1.27 export data, rerun that step with `GOTOOLCHAIN=go1.26.9`, then the later steps directly, and say so.

- [ ] **Step 3: Manual check in a real terminal**

Run: `GLOSS_LANG=zh-Hans ./gloss examples/` and press `?`. Then `--lang es`, press `/` and search for something absent. Then `GLOSS_LANG=es ./gloss --serve examples/sample.csv`.
Expected: Chinese help with nothing cut off, the Spanish "no matches" text, and a Spanish page with `lang="es"`.

- [ ] **Step 4: Commit**

```bash
git add DEVELOP.md AGENTS.md CHANGELOG.md docs
git commit -m "docs: internationalization: languages, --lang, and how to add strings and languages"
```
