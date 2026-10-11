# Themes

The catalog and preview/confirm/cancel design adapt go-thinkt's theme system.
Palette colors originate in its embedded themes (introduced in commit 0705db8),
which were generated from iTerm2 palettes. See LICENSE.thinkt and the generated
third-party notices. Gloss adds an explicit page background and Chroma style;
conversation-specific roles are mapped to gloss's document and chrome roles.

Built-ins are embedded JSON. User files have the same schema, inherit omitted
colors from their explicit light/dark base, and are capped at 64 KiB. Load checks
names, hex colors, syntax style names, unknown fields, and trailing JSON. The
catalog holds immutable snapshots; content-hash references let asynchronous
rendering keep the exact selected palette even if a file changes. Get/Resolve
never access disk. There is no global active theme.

The native host loads user choices and supplies a save callback. Browser hosts
use embedded choices and session-only selection. Native preferences store a
self-contained accepted snapshot through temporary-file rename; exports never
consult that preference. Fonts and layout are independent of this package.
