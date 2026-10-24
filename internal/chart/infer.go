package chart

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	MaxRows   = 10000 // Rows handed to Flint.
	MaxSeries = 16    // Measures charted at once.
	// MaxValues bounds the long-form rows several measures make, one per
	// row and measure: the compiler takes at most 65,535, and fewer keep a
	// resize's recompile quick.
	MaxValues = 20000
)

// Templates the flint-ntcharts text backend draws, by Flint's names.
const (
	TemplateBar              = "Bar Chart"
	TemplateStackedBar       = "Stacked Bar Chart"
	TemplateLollipop         = "Lollipop Chart"
	TemplateLine             = "Line Chart"
	TemplateArea             = "Area Chart"
	TemplateScatter          = "Scatter Plot"
	TemplateConnectedScatter = "Connected Scatter Plot"
	TemplateHeatmap          = "Heatmap"
	TemplateHistogram        = "Histogram"
	TemplateECDF             = "ECDF Plot"
	TemplateSparkline        = "Sparkline"
	TemplateCandlestick      = "Candlestick Chart"
)

// Note tells the person what inference set aside.
type Note string

// Shape is what a sheet's visible columns offer a chart.
type Shape struct {
	X        *Column    // The first temporal, else the first category column; nil when none.
	Y2       *Column    // A second category, for a heatmap.
	Measures []Column   // Numeric columns, in order, at most MaxSeries.
	OHLC     [4]*Column // open, high, low, close when all four are named and X is temporal.
	Rows     int        // Rows charted.
	Notes    []Note
	// Long form's own fields, named so they collide with no column.
	seriesField, valueField string
}

// Analyze types the visible columns of a table whose first row is the
// header, keeping at most MaxRows rows and MaxSeries measures.
func Analyze(header []string, rows [][]string, visible []int) Shape {
	var s Shape
	if len(rows) > MaxRows {
		s.Notes = append(s.Notes, Note(fmt.Sprintf("first %d rows charted; %d rows left out", MaxRows, len(rows)-MaxRows)))
		rows = rows[:MaxRows]
	}
	s.Rows = len(rows)
	names := map[string]int{}
	// distinct returns name, or the first "name n" no column has taken.
	distinct := func(name string) string {
		base := name
		for i := 2; names[name] > 0; i++ {
			name = fmt.Sprintf("%s %d", base, i)
		}
		names[name]++
		return name
	}
	var columns []Column
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
		columns = append(columns, typeColumn(name, cells))
	}
	s.seriesField, s.valueField = distinct("series"), distinct("value")
	var categories []*Column
	full := false
	for i := range columns {
		col := &columns[i]
		switch {
		case col.Temporal():
			if s.X == nil {
				s.X = col
			} else {
				s.Notes = append(s.Notes, Note(col.Name+" skipped: one time column is charted"))
			}
		case col.Measure():
			if full {
				continue
			}
			if len(s.Measures) < MaxSeries {
				s.Measures = append(s.Measures, *col)
			} else {
				s.Notes = append(s.Notes, Note(fmt.Sprintf("%s and later measures skipped: at most %d series", col.Name, MaxSeries)))
				full = true
			}
		case col.Type == TypeCategory:
			categories = append(categories, col)
		default:
			s.Notes = append(s.Notes, Note(col.Name+" skipped: "+strings.ToLower(col.Type)+" columns are not charted"))
		}
	}
	if s.X == nil && len(categories) > 0 {
		s.X, categories = categories[0], categories[1:]
	}
	if len(categories) > 0 {
		s.Y2 = categories[0]
		for _, c := range categories[1:] {
			s.Notes = append(s.Notes, Note(c.Name+" skipped: two category columns at most"))
		}
	}
	if s.X != nil && len(s.Measures) > 1 {
		if limit := MaxValues / len(s.Measures); s.Rows > limit {
			s.Notes = append(s.Notes, Note(fmt.Sprintf("first %d rows charted; %d rows left out: %d series share %d values", limit, s.Rows-limit, len(s.Measures), MaxValues)))
			s.Rows = limit
		}
	}
	if s.X != nil && s.X.Temporal() {
		for i, part := range []string{"open", "high", "low", "close"} {
			for m := range s.Measures {
				if strings.EqualFold(s.Measures[m].Name, part) {
					s.OHLC[i] = &s.Measures[m]
				}
			}
		}
		if s.OHLC[0] == nil || s.OHLC[1] == nil || s.OHLC[2] == nil || s.OHLC[3] == nil {
			s.OHLC = [4]*Column{}
		}
	}
	return s
}

// Templates lists what the shape can be drawn as, the default first.
func (s Shape) Templates() []string {
	switch {
	case s.Rows == 0:
		return nil
	case s.OHLC[0] != nil:
		return []string{TemplateCandlestick}
	case s.X != nil && s.X.Temporal() && len(s.Measures) > 0:
		return []string{TemplateLine, TemplateArea}
	case s.X != nil && s.Y2 != nil && len(s.Measures) == 1:
		return []string{TemplateHeatmap}
	case s.X != nil && len(s.Measures) == 1:
		return []string{TemplateBar, TemplateLollipop}
	case s.X != nil && len(s.Measures) > 1:
		return []string{TemplateStackedBar}
	case s.X == nil && len(s.Measures) >= 2:
		return []string{TemplateScatter, TemplateConnectedScatter}
	case s.X == nil && len(s.Measures) == 1:
		return []string{TemplateHistogram, TemplateECDF, TemplateSparkline}
	}
	return nil
}

type encoding struct {
	Field     string `json:"field"`
	Aggregate string `json:"aggregate,omitempty"`
}

type flintDocument struct {
	Data struct {
		Values []map[string]any `json:"values"`
	} `json:"data"`
	SemanticTypes map[string]string `json:"semantic_types"`
	ChartSpec     struct {
		ChartType string              `json:"chartType"`
		Encodings map[string]encoding `json:"encodings"`
	} `json:"chart_spec"`
}

// Document writes the Flint document for template, which must be one of
// Templates. Several measures become long-form rows (series, value) so one
// field feeds each channel.
func (s Shape) Document(template string) (Document, error) {
	valid := false
	for _, t := range s.Templates() {
		valid = valid || t == template
	}
	if !valid {
		return Document{}, fmt.Errorf("%s does not fit these columns", template)
	}
	var doc flintDocument
	doc.SemanticTypes = map[string]string{}
	doc.ChartSpec.ChartType = template
	doc.ChartSpec.Encodings = map[string]encoding{}
	use := func(c *Column) { doc.SemanticTypes[c.Name] = c.Type }
	long := len(s.Measures) > 1 && (template == TemplateLine || template == TemplateArea || template == TemplateStackedBar)
	for r := 0; r < s.Rows; r++ {
		emit := func(fields map[string]any) {
			if s.X != nil {
				fields[s.X.Name] = s.X.Values[r]
			}
			if s.Y2 != nil {
				fields[s.Y2.Name] = s.Y2.Values[r]
			}
			doc.Data.Values = append(doc.Data.Values, fields)
		}
		if long {
			for _, m := range s.Measures {
				if m.Values[r] != nil {
					emit(map[string]any{s.seriesField: m.Name, s.valueField: m.Values[r]})
				}
			}
			continue
		}
		fields := map[string]any{}
		for _, m := range s.Measures {
			fields[m.Name] = m.Values[r]
		}
		emit(fields)
	}
	if s.X != nil {
		use(s.X)
		doc.ChartSpec.Encodings["x"] = encoding{Field: s.X.Name}
	}
	switch {
	case template == TemplateCandlestick:
		for i, channel := range []string{"open", "high", "low", "close"} {
			use(s.OHLC[i])
			doc.ChartSpec.Encodings[channel] = encoding{Field: s.OHLC[i].Name}
		}
	case long:
		doc.SemanticTypes[s.seriesField], doc.SemanticTypes[s.valueField] = TypeCategory, s.Measures[0].Type
		doc.ChartSpec.Encodings["y"] = encoding{Field: s.valueField}
		doc.ChartSpec.Encodings["color"] = encoding{Field: s.seriesField}
	case template == TemplateHeatmap:
		use(s.Y2)
		use(&s.Measures[0])
		doc.ChartSpec.Encodings["y"] = encoding{Field: s.Y2.Name}
		doc.ChartSpec.Encodings["color"] = encoding{Field: s.Measures[0].Name, Aggregate: "sum"}
	case s.X == nil && len(s.Measures) >= 2:
		use(&s.Measures[0])
		use(&s.Measures[1])
		doc.ChartSpec.Encodings["x"] = encoding{Field: s.Measures[0].Name}
		doc.ChartSpec.Encodings["y"] = encoding{Field: s.Measures[1].Name}
	case s.X == nil:
		// A histogram or ECDF bins the measure along x; a sparkline runs it along y.
		use(&s.Measures[0])
		channel := "x"
		if template == TemplateSparkline {
			channel = "y"
		}
		doc.ChartSpec.Encodings[channel] = encoding{Field: s.Measures[0].Name}
	default:
		use(&s.Measures[0])
		y := encoding{Field: s.Measures[0].Name}
		if (template == TemplateBar || template == TemplateLollipop) && s.repeatsX() {
			y.Aggregate = "sum"
		}
		doc.ChartSpec.Encodings["y"] = y
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		return Document{}, err
	}
	return Document{Kind: KindFlint, Raw: raw}, nil
}

func (s Shape) repeatsX() bool {
	seen := map[any]bool{}
	for r := 0; r < s.Rows; r++ {
		if seen[s.X.Values[r]] {
			return true
		}
		seen[s.X.Values[r]] = true
	}
	return false
}
