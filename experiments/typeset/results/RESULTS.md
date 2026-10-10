# Typesetting comparison

go1.27.1; darwin/arm64; 5 fresh-process samples per row. Medians.

| Fixture / backend | First viewport ms | Resize ms | Repaint ms | First allocated MiB | Live Go heap MiB | Peak process RSS MiB |
|---|---:|---:|---:|---:|---:|---:|
| typeset.md / direct | 9.47 | 5.99 | 5.84 | 24.69 | 16.69 | 60.03 |
| typeset.md / pdf | 1108.94 | 14.83 | 0.90 | 352.56 | 271.02 | 331.67 |
| long.md / direct | 11.51 | 5.54 | 5.06 | 24.40 | 13.24 | 56.45 |
| long.md / pdf | 1136.90 | 40.54 | 0.94 | 463.72 | 270.11 | 340.20 |

First viewport includes font/input preparation, layout and rasterization, but not PNG encoding or process launch. PDF must compile the whole document and initialize PDFium. Direct preflights/parses the whole input but lays out blocks only through the viewport. Resize is 800 → 600 px with warm font/image/shaping data, after completing direct layout. Repaint draws an unchanged viewport: direct rasterizes again; PDF copies its existing raster. Neither includes Kitty encoding/transmission. Peak RSS covers first render, remaining layout, resize and repaint (before PNG artifacts), not only startup; it includes native/WASM memory. Live Go heap is a point sample before an explicit GC, not retained size. Both use the same eight pinned Go TTFs, 25 px body text and 50 px margins. Both paths use Chroma with the GitHub code theme; document styles and pagination still differ.
