# Internationalization scaffolding: English, Spanish, Simplified Chinese

## Intent

Make gloss usable by people who do not read English, starting with Spanish
(`es`) and Simplified Chinese (`zh-Hans`) beside English (`en`). This round
builds the scaffolding and proves it end to end with a small, visible slice
of translated strings; full coverage comes in later rounds. The bar is
"usable", not "every string translated".

The shape follows go-thinkt's `internal/i18n` (`~/projects/go-thinkt/internal/i18n`,
`docs/I18N.md`), which the owner wrote and wants reused.

Two halves, kept distinct in code and docs:

- **i18n** (the mechanism): the `internal/i18n` package, string extraction
  with inline English defaults, plurals, locale resolution, the locale handed
  to renderers, the coverage test, the docs-site multilingual setup.
- **l10n** (the content): the `es` and `zh-Hans` message files and translated
  guide pages.

## Decided by the owner

- **Translated:** the guides on the documentation site, the in-app help, and
  gloss's own UX: status line, notes and errors shown to a person, menus,
  file-chooser labels gloss supplies, the info box, theme/font pickers, the
  handoff screen, the `--serve` and `--pick-web` pages.
- **Never translated:** command-line options and their values (`--type`,
  `--accept`, `--view`, `--theme` names, …), and anything else a person types
  as an argument.
- **Renderers get the locale.** Locale-specific aspects of the medium — the
  decimal mark (`,` vs `.`), digit grouping, dates, month names — are decided
  by the renderer from a locale it is handed, not hard-coded.
- **Headless defaults to English** (`en`, `.` decimal point) regardless of
  `LANG`, so agent results do not depend on the machine. A new `--lang TAG`
  option changes that (and also overrides the viewer's language).
- **Detached handoffs follow the human's browser.** A detached `--serve`,
  `--pick` or `--pick-web` (started by an agent, no terminal) passes no tag
  unless `--lang` is given, and the page uses `navigator.language`. The
  JSON returned to the agent (startup, status, resume) stays English; a
  `message` is the human's own words and is passed through untouched. A
  foreground handoff passes the terminal's resolved tag to the page.
- **Only one formatting site switches this round:** the info box's counts and
  sizes, to prove the locale reaches a renderer end to end. Everything else
  (Excel, Grist, charts, video labels, maps) gets `document.Locale` plumbed
  but keeps today's formatting until later rounds.

## Goals

1. An `internal/i18n` package modeled on go-thinkt's: go-i18n v2
   (`github.com/nicksnyder/go-i18n/v2`), TOML message files embedded from
   `internal/i18n/locales/{en,es,zh-Hans}.toml`, `T(id, default)`,
   `Tf(id, default, args...)`, `Tn(id, one, other, n)`, inline English
   defaults so an untranslated ID shows English, `ActiveTag`,
   `AvailableLanguages` with native/English names and coverage.
2. Locale resolution for the native viewer: `--lang` > `GLOSS_LANG` >
   `LC_ALL` > `LC_MESSAGES` > `LANG` > `en`, normalized (`zh_CN.UTF-8` →
   `zh-Hans`, `es_MX` → `es`, `C`/`POSIX` → `en`), matched to the available
   catalogs with `golang.org/x/text/language` so unsupported languages fall
   back to English.
3. Headless modes (`--output`, `--output-dir`, `--print`, `--text`, `--info`,
   `--grep`, `--apps`, `skill`) use `en` unless `--lang` is given; they never
   read `GLOSS_LANG` or `LANG`.
4. A `document.Locale` (BCP 47 tag string, or a small struct wrapping one)
   on `document.Request`, beside `Theme`, set by the app from the resolved
   locale; `internal/document` still imports neither `internal/i18n` nor any
   environment. It governs only values gloss itself formats.
5. A coverage test like go-thinkt's (`coverage_test.go`): scans `cmd/` and
   `internal/` for literal `i18n.T/Tf/Tn` IDs, fails if `es` or `zh-Hans`
   lacks one, checks that `Tf` translations keep the same verbs as the
   default, and that TOML parses. A `task i18n:coverage` target.
6. A first translated slice, all three languages:
   - the help screen (`internal/app/help.go`: section titles and actions),
   - the status-line notes most people will see (loading, not found, export
     saved, search results/no matches, "Waiting for files"),
   - the handoff terminal screen (prompt chrome, QR caption, cancel hint),
   - the `--pick-web` page's chrome (headings, buttons, the message box).
7. The browser pages get their language from `navigator.language`, unless
   the server passes the terminal's resolved tag (for `--serve`/`--pick-web`,
   the page should match the terminal that launched it). Their strings live
   in a small JS catalog (`web/i18n.mjs` plus per-language data) with the
   same IDs as the Go catalogs where a string is shared, Node-tested like the
   other helpers. The WASM pager uses the Go catalogs.
8. The docs site in Hugo multilingual mode (hugo-book supports it):
   `en` at the current URLs, `es` and `zh-Hans` under `/es/` and `/zh-hans/`,
   a language switcher, English as the fallback for untranslated pages. This
   round translates the guide's `_index.md` and `using.md` only; generated
   pages (`command`, `for-llms`, `development`) stay English-only.
9. A short `docs/I18N.md`-style section in DEVELOP.md: how to add a string,
   how to add a language, the coverage test, the rules below.

## Calls made by the designer (check these)

- **Two tags, not one.** The renderer gets the full user tag (`es-MX`,
  `de-DE`); the message catalog is matched from it separately. `--lang de-DE`
  gives English text with German decimal commas. `LC_NUMERIC`/`LC_TIME` are
  not read separately this round; the one chain in goal 2 supplies both.
- **`--lang` validation.** A value that does not parse as a BCP 47 / POSIX
  locale (after normalizing `_`→`-` and stripping `.UTF-8`/`@…`) is a usage
  error, exit 1. A valid tag with no catalog falls back to English text, no
  error. `SKILL.md` documents both.
- **No silent clipping in help.** `helpColumn` already truncates actions with
  `ansi.Truncate`, so an overflow test would always pass. The help test
  instead fails if any row in any of the three languages is truncated at
  80×24; translations are written to that width budget.
- **No separate JS catalog.** The `--serve` and `--pick-web` pages are Go
  templates, so the server translates them from the same TOML catalogs and
  embeds the `web.*` messages the page scripts need as JSON in the page.
  A detached page takes its language from the browser's `Accept-Language`.
  Nothing is generated, so nothing can drift. The public browser demo
  (`examples/demo`) stays English this round.
- **go-thinkt code is rewritten, not copied** where it falls short: its
  coverage test only logs missing keys (gloss's fails), its `ResolveLocale`
  lacks `LC_MESSAGES` and region matching, and its `Tn` fallback
  (`"%d %s"`) breaks plural templates (gloss's executes the `{{.Count}}`
  template on the English default).

## Non-goals (this round)

- Translating every string. Unconverted strings stay English and are
  converted in later rounds.
- Translating the skill (`skills/gloss/SKILL.md`), the man page, `--help`
  option descriptions, or the generated command reference.
- Changing sibling libraries (ntcharts/Flint, ntmaps, nttypeset, ntvideo,
  ntfilepicker, ntopenwith). Where one already accepts a locale or label
  strings, pass them; where none does, list the gap in DEVELOP.md and leave
  the library alone.
- Right-to-left languages, CJK typeset fonts (ntfonts ships Latin subsets),
  a language picker inside the viewer, saving a language preference.

## Anti-goals (a failure even if it works)

- **Agent output that varies with the machine.** JSON keys, enum values,
  exit codes, `state`/`kind` values, the schema, and every headless result
  stay identical across `LANG` unless `--lang` is passed. Even with `--lang`,
  JSON keys and enum values never change; only human prose (e.g. an
  `error` message, `--print`'s rendered UI text) may.
- **Rewriting the user's data.** Values a file states itself — CSV cells,
  PDF text, Markdown prose, JSON — are shown as written. Locale applies only
  to values gloss formats: Excel styled numbers/dates, Grist values,
  chart axes/tooltips, info-box counts and sizes, video time labels, map
  scale/coordinates.
- **Broken layout.** The help screen is laid out for 80×24
  (`helpLeft`/`helpRight`). Translations, especially double-width Chinese,
  must not overflow it; widths are measured with display width
  (`ansi.StringWidth`/lipgloss), never `len`, and a test renders the help in
  each language at 80×24.
- **Translated CLI.** No translated option names or values, ever.

## Constraints

- go-i18n is a new direct dependency (it and BurntSushi/toml go through
  `task notices-check` and `THIRD_PARTY_NOTICES.md`); `golang.org/x/text` is
  already indirect and becomes direct.
- Builds for the browser (`GOOS=js`) and Windows must still pass; the
  catalogs are embedded, nothing read from disk at run time.
- `task ci` (including `docs-check`) must stay green; AGENTS.md, DEVELOP.md,
  the guide and the Unreleased changelog are updated in the same change.
  AGENTS.md's agent-contract rule applies: `--lang` is a new agent-facing
  option, so `skills/gloss/SKILL.md` documents it (in English) and it gets a
  domain in `cmd/gloss/domains.go`.
- Message IDs are literal, dotted (`app.help.everywhere`), and grouped by
  surface: `app.*`, `handoff.*`, `web.*`, `cmd.*`.

## Left to the builder

- Exact message IDs, the struct-vs-string shape of `document.Locale`, and
  which number/date formatter (`golang.org/x/text/number`/`message` or a
  small hand-written one) the document layer uses.
- How the server hands the tag to the browser page (query parameter, a
  field in the page's existing boot data, or a header).
- Which ~30–60 strings make up the first slice, within the surfaces in goal 6.
- Whether `T` keeps go-thinkt's package-global localizer (likely, since
  gloss runs one locale per process) or takes a localizer value.
- The Spanish and Chinese translations themselves (machine-drafted is fine
  for this round; mark the files as drafts awaiting native review).
