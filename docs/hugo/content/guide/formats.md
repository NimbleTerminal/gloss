---
title: "Formats and limits"
weight: 60
bookToC: false
---

# Formats and limits

- PNG, JPEG, WebP, BMP, TIFF: first frame/page, up to 32 megapixels.
  AVIF isn't supported.
- GIF and animated WebP: automatic playback with transparency, partial frames,
  disposal, and the file's loop count. **Space** pauses/resumes (or restarts after the last loop);
  `<`/`>` select 0.25×, 0.5×, 1×, 2×, or 4× speed; Backspace restores 1×.
  `r` reloads from the beginning at 1×. The status bar shows the frame, speed,
  and playback state. `e` exports the current full frame as PNG; zoom/pan still work.
  Canvas size is limited to 32 MP and animation to 1,000 frames. A GIF's
  indexed frames are decoded up front, within 64 Mi frame pixels in total,
  checked before decoding. A WebP's frames stay compressed and are decoded as
  they are shown, so a long full-canvas clip costs its canvas and one frame,
  not every frame; a large lossy frame can take longer to decode than its
  delay, in which case playback runs slower rather than skipping. A WebP frame
  that fails to decode is skipped. Alpha, blend and dispose-to-background
  settings are honored.
  Delays below 20 ms use 100 ms before the speed multiplier is applied; other
  delays are scaled directly (minimum 5 ms at 4×). Rendering can
  slow playback on large images or slow terminals; frames are not queued up.
  File-list previews, notebook images, and headless `--output`
  use the first frame on the full logical canvas. Frames are not `--page`s.
  Embedded Markdown GIFs and WebPs animate when visible: at most eight distinct
  destinations and 64 MiB of animation storage per document (decoded GIF
  frames; a WebP's canvas plus its retained compressed container); excess animations
  remain still.
- Video: MP4, M4V, MOV, WebM, MKV, Ogg (`.ogv`, Theora), AVI, MPEG, WMV and
  FLV, by name or by their first bytes (Ogg audio is not a video). The
  viewer plays them through [mpv](https://mpv.io)'s library, found when a
  video is opened (`brew install mpv` or your system's mpv package); it opens
  paused on the first frame. Details (`i`, `--info`) come from `ffprobe`, and
  the first frame, for previews, thumbnails, `--output` and `--print`, from
  `ffmpeg`, both from FFmpeg. Without mpv a video does not play and says so;
  without FFmpeg it plays but has no details or first frame. A video is never
  read into gloss: it may be up to 2 GiB and 4096×4096, from a local file
  only, not a URL. In the browser demo and app, a video plays in the
  browser's own video player, laid over gloss's cells, with the same keys:
  whatever the browser can decode, within the app's 128 MiB file limit.
  There it has no details, preview or thumbnail, which come from FFmpeg.
  Subtitles: files beside a video and named after it (`clip.srt`,
  `clip.en.srt`, `clip.vtt`, `clip.fr.ass`), up to 16 of 2 MiB each, the one
  with no language first, then by name; and the video's own subtitle streams.
  mpv draws them into the picture; drawn with glyphs, where they could not be
  read, they are terminal text over it instead. In the browser, SRT is
  converted to WebVTT and ASS/SSA are left out. `--text` prints a video's
  subtitles with their times: the first SRT or WebVTT file beside it, else
  its own text subtitles (FFmpeg); `--info` lists those. `--at` and
  `--frames` export frames at chosen times; see
  [Frames of a video](../using/#frames-of-a-video).
- HEIC/HEIF: primary still image, up to 32 megapixels, with container rotation
  and mirroring. Uses the [pure-Go h265 decoder](https://github.com/gen2brain/h265),
  without CGO or external converters. Unsupported HEVC features report decoder
  errors. HEIC works in Markdown, previews, PNG exports, and the browser demo.
- JPEG XL: the first frame of a bare codestream or a `.jxl` container, lossless
  or lossy, up to 32 megapixels, oriented as the file asks. Uses the
  [pure-Go jxl decoder](https://github.com/gen2brain/jxl), without CGO or
  external converters; embedded ICC profiles are not applied, and animated
  files show their first frame. Exif in the container feeds the info box. JPEG
  XL works in Markdown, previews, PNG exports, and the browser demo.
- SVG: ntsvg's pure-Go SVG renderer, rasterized to a 2400-pixel maximum edge.
  SVG support follows the underlying oksvg renderer, not a full browser engine.
  Rendering runs in an isolated worker that is stopped after 15 seconds.
- Mermaid: a `.mmd` or `.mermaid` file is one diagram, drawn as Markdown's
  `mermaid` fences are (see [Markdown](../markdown/#mermaid-diagrams)), in the
  theme's light or dark palette, and shown like a picture. Recognized by
  name only: the same text in a `.txt` file stays text. The source is limited
  to 64 KiB. `--text` gives the source; a diagram that does not parse is an
  error that names the line.
- PDF: PDFium via embedded WebAssembly; no Poppler, MuPDF, CGO, or external
  runtime installation. Pages render at 150 DPI by default (`--dpi 36..600`),
  with a 32-megapixel raster budget and a 10,000-page limit. Password-protected
  PDFs aren't supported. Decoded streams are limited to 256 MiB per decoding
  stage and 1 GiB of total decoding work. Stream lengths, chained filters, and
  predictor buffers are checked before opening. Ambiguous lengths, indirect
  filters/decode parameters, and unbudgeted filters (including LZW, JPEG 2000,
  and JBIG2) are rejected. PDF metadata and text parsing run in isolated
  workers with a 15-second deadline; timed-out workers are stopped.
  `/` searches the text layer and jumps to matching
  pages with excerpts; it does not highlight the raster or perform OCR.
- STL: ASCII and binary, flat-shaded triangles, up to 932,067 faces: nt3d's
  mesh limit. Normals are recomputed from vertex winding; binary STL face
  colors using the VisCAM/SolidView or Materialise Magics convention are
  honored. GPU
  rendering falls back to software and then wireframe. Software draws at full
  size for Kitty output, smaller only while its frames are slow, and samples
  meshes above 20,000 triangles; wireframe samples above 2,000 triangles, so
  large models can lose detail in fallback modes.
- Mesh color: an STL, and the faces of a 3MF, OBJ or PLY its file leaves plain, are drawn
  in one default blue. `C` opens a color picker over the mesh: a palette of
  swatches, one slider per channel, or a hex number typed in, each change
  painted as it is made; `Enter` chooses the swatch you have moved to (or keeps
  the color set) and closes, changing nothing if you have not moved; `Esc`
  undoes it. A 3MF with colors of its own is recolored whole, and the picker's
  Reset button (or `r`) gives its colors back; `s` limits the paint to the faces
  its file left plain. `--color #rrggbb`, or a name such as `orange`,
  paints the viewer and exports alike.
- 3MF parts: what the build places is listed by name, from the model or the
  slicer's settings, and `c` opens that list over the mesh: `Space` shows or
  hides a part, `Enter` shows it alone with the camera fitted to it, `n` keeps
  only it, `a` and `X` bring all back. `--parts name,name` and `--partn 2,4-6`
  choose parts for the viewer, an export, or `--info`, so an agent can pose
  one part of an assembly. The triangle limit applies to the parts shown, so a
  project too large to draw whole can be seen part by part.
- 3MF: the core specification's meshes, components, and build transforms;
  colors from base materials and color groups; objects kept in separate parts
  of the package, as slicers write them. Projects from Bambu Studio and its
  relatives are drawn in their filament colors, which those programs keep in
  settings of their own; colors painted onto faces are not read. Textures, beam lattices, slices, and
  encrypted content are not read. The same 932,067-face limit applies: a larger
  model is shown by its embedded thumbnail, and described by `i`. A package may
  hold 4,096 entries and unpack to 128 MiB; one that unpacks to more is shown
  by its thumbnail too, and described by what its root model says.
- OBJ: Wavefront OBJ triangles and polygons (concave ones split), polylines,
  vertex colors, the diffuse colors (`Kd`) of the materials its `.mtl`
  libraries give, and their PNG or JPEG textures (`map_Kd`), repeating as OBJ
  textures do. Side files are read only from beside the model or a folder
  below it, never for standard input: at most 32, libraries of up to 1 MiB,
  textures of up to 16 MiB and 16 megapixels each, 64 megapixels together.
  A texture that is refused or cannot be read leaves its faces in the
  material's color, and `i` says why. Textured faces keep their texture when
  the mesh is painted with `C`. Faces with their own normals are
  smooth-shaded. The same 932,067-face limit applies.
- PLY: Stanford PLY, ASCII or binary: faces with vertex or face colors, a
  scanner's range grid joined into a surface, or, with no faces, a cloud of up
  to 500,000 points drawn as discs sized to the spacing of their neighbors.
  A texture the file names is not read.
- OBJ and PLY files do not say which way is up; gloss takes Y as up, as most
  tools write them, and turns them to the Z-up that STL, 3MF and the named
  views (`--view top`) use. A model that comes out lying on its side was
  written the other way: `--up z` reads an OBJ or PLY as Z-up, and `--up y`
  stands up a Y-up STL. `i` says which axis was taken. 3MF is Z-up by its
  specification.
- GeoJSON (`.geojson`) and GPX (`.gpx`, or any file whose root element is
  `<gpx>`): what the file holds is drawn on a plain background, framed to fit,
  and nothing is fetched. Points and GPX waypoints are pins, lines, routes and
  track segments are lines (one per segment), and polygons are drawn as
  thinner outlines, holes included. GeoJSON is known only by its name, so a
  `.json` file stays JSON; `--type geojson` reads one as a map. A file may
  hold up to a million positions and 100,000 features, nested at most 64
  deep; elevation, times and other properties are not read. A file with no
  positions in it is an error.
- PMTiles (`.pmtiles`, or a file that begins as one): an archive of map tiles,
  shown as a map you can pan and zoom, framed on the bounds it declares. It
  is not read whole, as other files are: its tiles are read from the file as
  the map needs them, so the 128 MiB limit does not apply (except to a
  downloaded or piped archive). Only raster archives, of PNG or JPEG tiles,
  with uncompressed or gzip directories, can be drawn; a vector (MVT) archive
  is described and refused. Each read is bounded: a directory or the metadata
  at most 16 MiB, a tile at most 8 MiB and 4096 pixels a side. The archive's
  attribution is drawn along the map's foot.
- Plain text (`.txt`, `.text`, `.log`, and any file that is not binary):
  shown as it is, in the document view, so that no line of it is read as a
  heading, a list, or emphasis. Returns and a byte order mark are dropped,
  bytes that are not UTF-8 stand as �, and a file longer than 2 MiB is cut.
  A file of no known kind is read as text when its first 8 KiB hold no NUL
  byte, are UTF-8, and are mostly printable, as `less` would show it; source
  code and configuration open that way, highlighted by
  [chroma](https://github.com/alecthomas/chroma) in the language its name or
  first line says. Only binary files are unsupported.
- JSON (`.json`, `.jsonl`, `.ndjson`): pretty-printed and highlighted as a
  fenced block; `s` shows it as it is in the file. A file with one value to a
  line, such as a training set, a batch request, or a log export, is shown as
  numbered records, whatever its name. Malformed JSON is refused with the line
  it fails on. Extensionless JSON and stdin are recognised by parsing; JSON
  Lines is recognised when its first two lines each hold an object or array.
  `--type jsonl` (or `ndjson`) is the same as `--type json`. The
  view is cut at 2 MiB of pretty-printed text; `i` says how many records
  there are.
- Jupyter notebooks (`.ipynb`, nbformat 4): Markdown cells as they are, code
  cells fenced in the kernel's language after their `In [n]` count, and each
  output after its cell: text as it was printed, errors without their colour
  codes, and PNG, JPEG, GIF, and SVG outputs as pictures. Widgets, HTML, and
  LaTeX outputs fall back to their text.
- HTML (`.html`, `.htm`): converted to Markdown with
  [html-to-markdown](https://github.com/JohannesKaufmann/html-to-markdown), so
  saved pages and wiki exports read as documents: headings, lists, tables,
  links, code, and pictures beside the file are kept; scripts, styles, and
  layout are not. Nothing is fetched. A page may be 8 MiB.
- Word (`.docx`, `.docm`): turned into Markdown and shown as such, so `s` shows
  the Markdown. Headings, lists, tables, links, pictures, and bold, italic, and
  struck text are kept; page layout, headers and footers, footnotes, comments,
  and text boxes are not. A document longer than 2 MiB of Markdown is cut.
- Excel (`.xlsx`, `.xlsm`): each sheet is a grid that scrolls by row and column,
  with `n` and `p` turning between sheets. Cells show their values, formulas
  by their last result, and dates and times where a cell's style says so. Up to
  100,000 rows and 1,024 columns of a sheet are read; the rest are counted.
  Formatting, merged cells, charts, and pictures are not shown.
- Grist (`.grist`): a Grist document is a SQLite database, and each of its
  tables is shown as a sheet is, with `n` and `p` turning between tables. The
  first row holds the columns' labels, and rows and columns stand in the order
  Grist keeps them in; summary tables come after the others, titled as Grist
  titles them (`Sightings [by Site]`), what they group by shown as the table
  summed shows it. Cells show what
  is stored: formulas by their last result, as Grist saved it, and an error
  by its name (`#TypeError`). Dates and times are written out, a reference
  shows what Grist shows for it, or `Table[row]` where it shows the row,
  lists are joined with commas, and attachments are named, not opened. A
  hyperlink cell shows its words and leads to its address, as an Excel link
  does. Row
  ids, the positions rows are sorted by, Grist's helper columns, and a
  summary's list of the rows behind each line are left out. Nothing is worked out anew: no formulas, access rules, widgets, or
  number formats, and nothing is written. Up to 100,000 rows and 1,024
  columns of a table are read. A document in SQLite's WAL mode is not read,
  nor is any other SQLite database: gloss is not a database browser.
- CSV (`.csv`, `.tsv`): shown as a sheet is. The separator is read from the
  file: a comma, tab, semicolon, or pipe, whichever the first lines agree on;
  a `.tsv` is read as tabs. Quoted values may hold the separator and line
  breaks. Only the file's name says it is a table: text is not sniffed.
- Columns of a table: `x` hides the one under the cursor, `X` brings them all
  back, and `c` lists them to tick and untick. Hidden columns keep their
  letters, and one column always stays. `--cols Name,City` shows only the
  columns with those headers (or letters), and `--coln 2,4-6` those numbered
  so, in every table opened; a table with none of them is shown whole.
- Web addresses in tables: a cell that holds an address or, in Excel and
  Grist, links to one is marked 🔗, and is a link to the terminal as well:
  click it as your terminal has links clicked (often with Cmd, Ctrl, or
  Shift held, since gloss takes plain clicks) and the terminal opens it in
  your browser. gloss itself starts no browser. A sheet has a cursor, and the
  status bar shows the address under it. With `--fetch`, `Enter` on such
  a cell downloads what it names and opens it like a dropped file; a picture
  is shown as one, and `Esc` closes it and returns to the cell. Only http and
  https are fetched, of no more than 128 MiB, and only on `Enter` or a URL
  drop/paste, or for an address named on the command line: gloss never
  fetches on its own, and `GLOSS_NO_FETCH` forbids it altogether. Fetched
  files are removed when closed or when gloss exits, unless they were picked.
- Input files and stdin are limited to 128 MiB. One active document and, when
  enabled, one independent preview are kept open.
  Images, SVGs, and PDF pages zoom by cropping the existing raster, up to 64×;
  use a higher PDF DPI for more detail. There is no file watching, and no
  fetching but of an address named on the command line or, with `--fetch`,
  chosen by `Enter` on a cell or an explicit URL drop/paste.
