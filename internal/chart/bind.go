package chart

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

func sortedStrings(s []string) []string {
	sort.Strings(s)
	return s
}

// Bind fills a Flint document's empty data.values with a table's rows,
// typed as Analyze types them, and adds semantic types for fields the
// document does not name. A bound document, or any other kind, is returned
// as it is. At most MaxRows rows are bound; a Note says when rows were left
// out.
func Bind(doc Document, header []string, rows [][]string, visible []int) (Document, []Note, error) {
	if doc.Kind != KindFlint || !doc.Unbound {
		return doc, nil, nil
	}
	var notes []Note
	if len(rows) > MaxRows {
		notes = append(notes, Note(fmt.Sprintf("first %d rows bound; %d rows left out", MaxRows, len(rows)-MaxRows)))
		rows = rows[:MaxRows]
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(doc.Raw, &top); err != nil {
		return doc, nil, err
	}
	types := map[string]json.RawMessage{}
	if top["semantic_types"] != nil {
		if err := json.Unmarshal(top["semantic_types"], &types); err != nil {
			return doc, nil, fmt.Errorf("semantic_types: %w", err)
		}
	}
	names := map[string]int{}
	distinct := func(name string) string {
		base := name
		for i := 2; names[name] > 0; i++ {
			name = fmt.Sprintf("%s %d", base, i)
		}
		names[name]++
		return name
	}
	values := make([]map[string]any, len(rows))
	for r := range values {
		values[r] = map[string]any{}
	}
	for _, c := range visible {
		name := ""
		if c < len(header) {
			name = clean(strings.TrimSpace(header[c]))
		}
		if name == "" {
			name = fmt.Sprintf("column %d", c+1)
		}
		name = distinct(name)
		cells := make([]string, len(rows))
		for r, row := range rows {
			if c < len(row) {
				cells[r] = row[c]
			}
		}
		col := typeColumn(name, cells)
		if _, named := types[name]; !named {
			kind, _ := json.Marshal(col.Type)
			types[name] = kind
		}
		for r := range rows {
			if col.Values[r] != nil {
				values[r][name] = col.Values[r]
			}
		}
	}
	// The compiler charts a field the table lacks as nothing at all; say so
	// here, by name, with the columns there are.
	if top["chart_spec"] != nil {
		var chartSpec struct {
			Encodings map[string]json.RawMessage `json:"encodings"`
		}
		if err := json.Unmarshal(top["chart_spec"], &chartSpec); err != nil {
			return doc, nil, fmt.Errorf("chart_spec: %w", err)
		}
		channels := make([]string, 0, len(chartSpec.Encodings))
		for channel := range chartSpec.Encodings {
			channels = append(channels, channel)
		}
		for _, channel := range sortedStrings(channels) { // The first missing field, by channel, every time.
			raw := chartSpec.Encodings[channel]
			var field string
			var enc struct {
				Field string `json:"field"`
			}
			if json.Unmarshal(raw, &field) != nil && json.Unmarshal(raw, &enc) == nil {
				field = enc.Field
			}
			if field != "" && names[field] == 0 {
				columns := make([]string, 0, len(names))
				for name := range names {
					columns = append(columns, name)
				}
				shown := strings.Join(sortedStrings(columns), ", ")
				if shown == "" {
					shown = "no column is shown"
				}
				return doc, nil, fmt.Errorf("the %s encoding names %q, which the table's visible columns lack: %s", channel, field, shown)
			}
		}
	}
	data, err := json.Marshal(map[string]any{"values": values})
	if err != nil {
		return doc, nil, err
	}
	kinds, err := json.Marshal(types)
	if err != nil {
		return doc, nil, err
	}
	top["data"], top["semantic_types"] = data, kinds
	raw, err := json.Marshal(top)
	if err != nil {
		return doc, nil, err
	}
	return Document{Kind: KindFlint, Raw: raw}, notes, nil
}
