package chart

import "testing"

func TestColumnTypes(t *testing.T) {
	for _, tc := range []struct {
		name  string
		cells []string
		want  string
	}{
		{"revenue", []string{"48.5", "39.9", "1,240"}, TypeQuantity},
		{"price", []string{"$48.50", "$39.90", "$1,240.00"}, TypePrice},
		{"share", []string{"12%", "7.5%", "80%"}, TypePercentage},
		{"units", []string{"120", "95", "140"}, TypeCount},
		{"qty", []string{"1", "2"}, TypeCount},
		{"score", []string{"1", "2"}, TypeQuantity},
		{"booked", []string{"2026-03-31", "2026-06-30"}, TypeDate},
		{"at", []string{"2026-03-31 14:05", "2026-06-30 09:00:01"}, TypeDateTime},
		{"when", []string{"2026-03-31T14:05:00Z"}, TypeDateTime},
		{"month", []string{"2026-01", "2026-02"}, TypeYearMonth},
		{"month", []string{"Jan 2026", "Feb 2026"}, TypeYearMonth},
		{"period", []string{"2026-Q1", "2026-Q2"}, TypeYearQuarter},
		{"year", []string{"2024", "2025"}, TypeYear},
		{"total", []string{"2024", "2025"}, TypeQuantity},
		{"active", []string{"TRUE", "FALSE", "true"}, TypeBoolean},
		{"region", []string{"North", "South"}, TypeCategory},
		{"id", []string{"1001", "1002"}, TypeCategory},
		{"zip", []string{"02134", "90210"}, TypeCategory},
		{"user_id", []string{"1001", "1002"}, TypeCategory},
		{"UserID", []string{"1001", "1002"}, TypeCategory},
		{"Zip Code", []string{"02134", "90210"}, TypeCategory},
		{"width", []string{"12", "40"}, TypeQuantity},    // Not "id".
		{"barcode", []string{"12", "40"}, TypeQuantity},  // Not "code".
		{"discount", []string{"12", "40"}, TypeQuantity}, // Not "count".
		{"fiscal_year", []string{"2024", "2025"}, TypeYear},
		{"UnitsSold", []string{"120", "95"}, TypeCount},
		{"empty", []string{"", ""}, TypeCategory},
	} {
		if got := typeColumn(tc.name, tc.cells); got.Type != tc.want {
			t.Errorf("%s %v: %s, want %s", tc.name, tc.cells, got.Type, tc.want)
		}
	}
}

func TestColumnTypeTolerance(t *testing.T) {
	// A few blanks or dashes do not stop a column being a measure.
	c := typeColumn("revenue", []string{"1", "2", "", "-", "n/a", "3", "4", "5", "6", "7", "8", "9"})
	if c.Type != TypeQuantity || c.Values[2] != nil || c.Values[3] != nil || c.Values[5] != 3.0 {
		t.Fatalf("%+v", c)
	}
	// One number among words is still a category.
	if c := typeColumn("note", []string{"a", "b", "c", "4"}); c.Type != TypeCategory || c.Values[3] != "4" {
		t.Fatalf("%+v", c)
	}
	// NaN and infinities, as pandas and R write them, are blanks: JSON cannot carry them.
	c = typeColumn("v", []string{"1", "NaN", "inf", "2", "-Infinity", "nan", "3", "4", "5", "6"})
	if c.Type != TypeQuantity || c.Values[1] != nil || c.Values[2] != nil || c.Values[4] != nil || c.Values[5] != nil || c.Values[3] != 2.0 {
		t.Fatalf("%+v", c)
	}
	if _, _, _, ok := parseNumber("NaN"); ok {
		t.Fatal("NaN parsed as a number")
	}
	// Below the 90% threshold the column is a category.
	if c := typeColumn("mixed", []string{"1", "2", "3", "4", "5", "6", "7", "8", "x", "y"}); c.Type != TypeCategory {
		t.Fatalf("%+v", c)
	}
}

func TestParseNumber(t *testing.T) {
	for _, tc := range []struct {
		in      string
		v       float64
		symbol  bool
		percent bool
		ok      bool
	}{
		{"1,234.5", 1234.5, false, false, true},
		{"$1,234", 1234, true, false, true},
		{"€ 12", 12, true, false, true},
		{"12.5%", 0.125, false, true, true},
		{"-3", -3, false, false, true},
		{"(3)", 0, false, false, false},
		{"1.2.3", 0, false, false, false},
		{"", 0, false, false, false},
	} {
		v, symbol, percent, ok := parseNumber(tc.in)
		if v != tc.v || symbol != tc.symbol || percent != tc.percent || ok != tc.ok {
			t.Errorf("%q: %v %v %v %v", tc.in, v, symbol, percent, ok)
		}
	}
}
