package chart

import (
	"math"
	"strconv"
	"strings"
	"time"
)

// Flint semantic types gloss infers.
const (
	TypePrice       = "Price"
	TypePercentage  = "Percentage"
	TypeCount       = "Count"
	TypeQuantity    = "Quantity"
	TypeDate        = "Date"
	TypeDateTime    = "DateTime"
	TypeYearMonth   = "YearMonth"
	TypeYearQuarter = "YearQuarter"
	TypeYear        = "Year"
	TypeBoolean     = "Boolean"
	TypeCategory    = "Category"
)

// Column is a typed column of a sheet.
type Column struct {
	Name   string
	Type   string
	Values []any // float64 for a measure, string otherwise; nil for an empty cell.
}

// Measure reports whether the column holds numbers to plot.
func (c Column) Measure() bool {
	switch c.Type {
	case TypePrice, TypePercentage, TypeCount, TypeQuantity:
		return true
	}
	return false
}

// Temporal reports whether the column holds points or periods in time.
func (c Column) Temporal() bool {
	switch c.Type {
	case TypeDate, TypeDateTime, TypeYearMonth, TypeYearQuarter, TypeYear:
		return true
	}
	return false
}

// agreement is the share of non-empty cells that must fit a type.
const agreement = 0.9

var (
	dateLayouts     = []string{"2006-01-02"}
	dateTimeLayouts = []string{"2006-01-02 15:04", "2006-01-02 15:04:05", time.RFC3339, "2006-01-02T15:04", "2006-01-02T15:04:05"}
	monthLayouts    = []string{"2006-01", "Jan 2006", "January 2006"}
)

func headerHas(name string, words ...string) bool {
	name = strings.ToLower(name)
	for _, w := range words {
		if strings.Contains(name, w) {
			return true
		}
	}
	return false
}

// typeColumn decides what a column holds from its cells, excluding empty
// ones and placeholders. Numbers win when nine in ten cells parse, unless
// the header says the numbers are labels (id, code, zip, phone).
func typeColumn(name string, cells []string) Column {
	c := Column{Name: name, Type: TypeCategory, Values: make([]any, len(cells))}
	var filled, numbers, symbols, percents, integers, dates, dateTimes, months, quarters, years, booleans int
	nums := make([]float64, len(cells))
	for i, cell := range cells {
		s := strings.TrimSpace(cell)
		switch strings.ToLower(s) {
		case "", "-", "—", "n/a", "na", "null", "none", "nan", "inf", "-inf", "+inf", "infinity", "-infinity", "+infinity":
			continue
		}
		filled++
		c.Values[i] = s
		if v, symbol, percent, ok := parseNumber(s); ok {
			numbers++
			nums[i] = v
			if symbol {
				symbols++
			}
			if percent {
				percents++
			}
			if v == float64(int64(v)) && !strings.ContainsAny(s, ".%") {
				integers++
				if len(s) == 4 && v >= 1000 && v <= 9999 {
					years++
				}
			}
		}
		switch {
		case parses(s, dateLayouts):
			dates++
		case parses(s, dateTimeLayouts):
			dateTimes++
		case parses(s, monthLayouts):
			months++
		case len(s) == 7 && s[4] == '-' && (s[5] == 'Q' || s[5] == 'q') && s[6] >= '1' && s[6] <= '4' && parses(s[:4], []string{"2006"}):
			quarters++
		}
		switch strings.ToLower(s) {
		case "true", "false", "yes", "no":
			booleans++
		}
	}
	if filled == 0 {
		return c
	}
	fits := func(n int) bool { return float64(n) >= agreement*float64(filled) }
	switch {
	case fits(dates):
		c.Type = TypeDate
	case fits(dateTimes):
		c.Type = TypeDateTime
	case fits(months):
		c.Type = TypeYearMonth
	case fits(quarters):
		c.Type = TypeYearQuarter
	case fits(booleans):
		c.Type = TypeBoolean
	case fits(years) && headerHas(name, "year"):
		c.Type = TypeYear
	case fits(numbers) && !headerHas(name, "id", "code", "zip", "phone"):
		switch {
		case symbols > filled/2:
			c.Type = TypePrice
		case percents > filled/2:
			c.Type = TypePercentage
		case integers == numbers && headerHas(name, "count", "qty", "quantity", "units", "number"):
			c.Type = TypeCount
		default:
			c.Type = TypeQuantity
		}
		for i := range c.Values {
			if c.Values[i] == nil {
				continue
			}
			if _, _, _, ok := parseNumber(c.Values[i].(string)); ok {
				c.Values[i] = nums[i]
			} else {
				c.Values[i] = nil
			}
		}
	}
	return c
}

func parses(s string, layouts []string) bool {
	for _, l := range layouts {
		if _, err := time.Parse(l, s); err == nil {
			return true
		}
	}
	return false
}

// parseNumber reads a number as a sheet shows it: thousands separators, one
// currency symbol before or after, or a trailing percent sign, which scales
// the value down by 100.
func parseNumber(s string) (v float64, symbol, percent bool, ok bool) {
	s = strings.TrimSpace(s)
	for _, sym := range []string{"$", "€", "£", "¥"} {
		if strings.HasPrefix(s, sym) || strings.HasSuffix(s, sym) {
			s, symbol = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(s, sym), sym)), true
			break
		}
	}
	if strings.HasSuffix(s, "%") {
		s, percent = strings.TrimSpace(strings.TrimSuffix(s, "%")), true
	}
	s = strings.ReplaceAll(s, ",", "")
	if s == "" {
		return 0, false, false, false
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, false, false, false
	}
	if percent {
		v /= 100
	}
	return v, symbol, percent, true
}
