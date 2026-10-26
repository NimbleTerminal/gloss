---
title: "Charts"
weight: 65
bookToC: false
---

# Charts

Press `C` on a CSV, Excel or Grist table to chart its visible columns. With
120 columns or more the chart stands beside the grid; narrower terminals
show it in the grid's place, and the grid's keys keep working underneath.
Press `C` again to close it.

## What gets charted

gloss reads the header row and types each visible column from its cells:
prices (a currency symbol), percentages, counts, other numbers, dates and
times in the forms gloss itself writes, months, quarters, years, and
categories. The chart follows the shape of the visible columns:

| Visible columns | Chart |
| --- | --- |
| a date column and one number | line, summed per date (`t`: area) |
| a date column and several numbers | line, one series per number, summed per date |
| a category and one number | bar, summed per category (`t`: lollipop) |
| a category and several numbers | stacked bar |
| two categories and one number | heatmap |
| two or more numbers and no category | scatter (`t`: connected) |
| one number alone | histogram (`t`: ECDF, sparkline) |
| open, high, low, close and a date | candlesticks |

Hide columns with `x`, or choose them with `c`, to change what is charted;
`X` brings every column back. `t` cycles the chart types that fit the same
columns. The status bar names the chart, its series and rows, and the first
warning the chart compiler raised; `i` lists them in the info box. A chart
that fails to compile leaves the last good one on screen and says why.

## Chart files

A JSON file holding a [Flint](https://github.com/microsoft/flint-chart)
`ChartAssemblyInput`, a flint-ntcharts envelope, or an ntcharts spec opens as
a chart on its own. Flint documents need their rows inline (`data.values`);
`data.url` is never fetched. `--type chart` forces the kind.

## Charting from outside

`--chart FILE` draws a Flint document over the table being opened. Leave
`data.values` empty and gloss binds the table's rows, typed from their text,
to the fields the document names by header; a field the visible columns
lack is an error that names it. `aggregate` on an encoding (sum, mean,
average, count, min, max, median) folds the rows before the chart is built.
`R` reads the file again. Hiding columns re-binds the chart; `t` does
nothing while a document is in charge, and `C` closes and reopens it.

Under `--serve`, push a document into the running viewer:

```sh
curl -X POST -d @revenue.json http://127.0.0.1:PORT/TOKEN/chart
```

The reply says what was drawn, `{"type":"bar","series":1,"rows":8,"warnings":[]}`,
or carries `{"error":"…"}`: 400 for a body that is not a chart, 413 over
16 MiB, 422 for a document that does not compile, 503 while the viewer is
busy or when it does not answer within 20 seconds. A chart refused with 503
is dropped, never drawn late, so it is safe to send again. The push renews the idle
window of a detached viewer.

## Pictures

`e` saves the chart pane as `<name>-chart.png` (a chart file as
`<name>.png`); a text chart cell for cell at 12×24 pixels: the terminal chart, braille
lines and box axes included, in the theme's colours. `--output` on a table
with `--chart`, or on a chart file, draws 40 to 120 columns from
`--max-edge` in dark on white; `--info` adds a Chart section with the type,
series, rows and the first compiler warning.

## Graphics

`ctrl+g` draws the chart as a picture instead of terminal text, with
[go-analyze/charts](https://github.com/go-analyze/charts) through
flint-ntcharts: grouped bars, pies and the other Flint templates the text
chart only approximates, in Kitty graphics where the terminal has them and
half-blocks otherwise. `ctrl+g` again returns to text; the choice lasts the
session, across charts and files. A chart the renderer cannot draw (heatmaps,
box plots, waterfalls) stays text, and the status bar says so. Graphics are
native only: the browser demo has `ctrl+o`'s page instead. `e` saves the
picture, drawn again at 12×24 pixels a cell; `--output` stays the text chart.

## Browser

`ctrl+o` on a chart writes it as a page of its own, `<name>-chart.html` (a
chart file as `<name>.html`), into the session's temporary folder and opens
it in the default browser. The page draws the chart with
[Apache ECharts](https://echarts.apache.org/), which gloss embeds: tooltips,
legends and Flint's whole template set, including charts the terminal cannot
draw, such as grouped bars. It fetches nothing. The file lasts the session:
the status bar names its path, and the folder goes when gloss quits, so
save the page from the browser to keep or send it.
A Flint document gives Flint's own ECharts chart; a bare ntcharts spec is
drawn by ntcharts' ECharts module, which knows bar and timeseries charts, and
other specs stay terminal-only, which `ctrl+o` says. The page is not
available in the browser demo or in a preview.

Under `--serve`, `GET /<token>/chart` is the same page for whatever the
viewer shows, linked under **More** on the viewer page; `ctrl+o` there names
the address instead of opening it. It answers 404 while no chart is shown
and 422, naming the type, for a chart the browser cannot draw.

## Limits

At most 10,000 rows and 16 numeric columns are charted, and several
numbers share 20,000 values between them; the rest are left out and the
status bar says so. A chart file may be 16 MiB. Compiles are
cancelled after 15 seconds. The Flint compiler runs inside gloss as
WebAssembly; nothing is installed or fetched. The terminal draws bar, line,
area, scatter, heatmap, histogram, sparkline and candlestick charts; bars of
several series are stacked, never grouped.
