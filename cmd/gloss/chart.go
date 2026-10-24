package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"

	"nimbleterminal.dev/gloss/internal/chart"
	"nimbleterminal.dev/gloss/internal/document"
)

// readChart reads and sniffs the --chart file.
func readChart(path string) (*chart.Document, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, chart.MaxDocumentBytes+1))
	if err != nil {
		return nil, err
	}
	doc, err := chart.Sniff(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &doc, nil
}

// chartFor is the chart to draw for a loaded document: its own when it is
// a chart file, else --chart bound to the sheet's visible columns; nil
// when there is none.
func chartFor(opts options, r document.Result) (*chart.Document, []chart.Note, error) {
	switch {
	case r.Chart != nil:
		return r.Chart, nil, nil
	case opts.Chart == nil || r.Sheet == nil:
		return nil, nil, nil
	}
	hidden := opts.Columns.Hidden(r.Sheet)
	var visible []int
	for c := 0; c < r.Sheet.Columns; c++ {
		if hidden == nil || c >= len(hidden) || !hidden[c] {
			visible = append(visible, c)
		}
	}
	rows := r.Sheet.Rows
	var header []string
	if len(rows) > 0 {
		header, rows = rows[0], rows[1:]
	}
	bound, notes, err := chart.Bind(*opts.Chart, header, rows, visible)
	if err != nil {
		return nil, nil, err
	}
	return &bound, notes, nil
}

// chartFields describes what a chart draws, for --info: without a heading,
// so a chart file's own Chart section can take them, and with Source only
// when the chart came from --chart.
func chartFields(ctx context.Context, doc chart.Document, source string, rows int, notes []chart.Note) []document.Field {
	var fields []document.Field
	if source != "" {
		fields = append(fields, document.Field{Label: "Source", Value: source})
	}
	res, err := chart.Default.Compile(ctx, doc, 80, 24)
	if err != nil {
		fields = append(fields, document.Field{Label: "Problem", Value: err.Error()})
	} else {
		fields = append(fields, document.Field{Label: "Type", Value: string(res.Spec.Type)}, document.Field{Label: "Series", Value: strconv.Itoa(len(res.Spec.Data.Series))})
		if len(res.Warnings) > 0 {
			fields = append(fields, document.Field{Label: "Warning", Value: res.Warnings[0].Message})
		}
	}
	if rows > 0 {
		fields = append(fields, document.Field{Label: "Rows", Value: strconv.Itoa(rows)})
	}
	if len(notes) > 0 {
		fields = append(fields, document.Field{Label: "Note", Value: string(notes[0])})
	}
	return fields
}
