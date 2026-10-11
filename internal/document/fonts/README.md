# Embedded typeset fonts

The viewer embeds `Gloss*.ttf`: 28 static faces (regular, bold, italic and bold
italic), approximately 1.2 MB in total. The original Go fonts below remain as
reference assets, but are no longer in the viewer's font embed pattern.

| Look | Body | Code | Subset identifiers |
| --- | --- | --- | --- |
| Readable (default) | Literata | Iosevka | A / E |
| Editorial | Newsreader | Iosevka | B / E |
| Technical | IBM Plex Sans | IBM Plex Mono | C / F |
| Compact | Source Serif 4 | JetBrains Mono | D / G |

All seven families are OFL-1.1; each family's full license is alongside these
files. Google Fonts sources are pinned to commit
`bd8f81ddb5c74d5c8897b36ad88b440266245103`; Iosevka is the official v34.9.0
TTF release. `sources.json` records exact URLs and SHA-256 hashes.
`subsets.sha256` records the resulting embedded files.

Build-time generation only (Python with `fonttools==4.60.2`):

```sh
python scripts/subset-fonts.py /tmp/gloss-font-sources --download
```

The script verifies downloads, instances variable fonts at weight 400/700,
uses their default width, and fixes optical sizes: Literata 12, Newsreader 16,
Source Serif 4 16. It strips hints and discretionary ligature features, retaining
composition, mark positioning, localization and kerning. The subset covers
ASCII, Latin-1, combining marks U+0300–036F, general punctuation U+2000–206F,
and U+FFFD when present in the source. It does not add glyphs absent upstream.
The internal families are renamed `Gloss Text A` through `Gloss Text G` to
avoid reserved font names; the viewer identifies their source families.
Runtime shaping also explicitly disables liga, clig, dlig and calt. No runtime
font discovery, downloads, or system-font dependencies.

## Previous Go-font reference assets


Unmodified TTF bytes from `golang.org/x/image v0.46.0/font/gofont` (the exported
TTF byte arrays), created by Bigelow & Holmes for the Go project. The proportional
and monospace regular, bold, italic, and bold-italic faces are pinned here.
See LICENSE for the font-specific BSD license. No runtime font downloads.

SHA-256:

```text
c18494baa7ea35b8dbfac3861787d360b9c0ab91232562371a09e6fbf09aaa40  Go-Bold.ttf
d6c33857c7ea7d740a8e1d81d37261782f698e9c17a2c9bf3bb0abbaf99e3c90  Go-BoldItalic.ttf
6efef00d080c8fe68bbe2bf08526294c83d52d7b90b16f938953025c0868cc57  Go-Italic.ttf
197d9f3703b4c00af609178876a8d73e396f64fe438b2c871778566632374be3  Go-Regular.ttf
b165f1a142dc88d3c388de2c89d3ab62dd04620ef21ad07f43fb51f77857edc3  GoMono-Bold.ttf
cb43aca5ab36631c0b4679831017cf249c33db4f721daad7379e04bba383ef1f  GoMono-BoldItalic.ttf
10d5f73519ea32b4882897a9bec6c42ba2ca27d6c06ba9a0e55ae38a383cf7e6  GoMono-Italic.ttf
8bc66a0154bbf69cd24e5bde41a12ec9495c8a242c5def2255e4a164900f1ed7  GoMono-Regular.ttf
```
