package chart

import (
	"strings"
	"unicode"

	"github.com/NimbleMarkets/ntcharts/v2/spec"
)

// clean strips what a terminal must never see from text that came from a
// sheet or a chart file: control, format and other non-printable runes.
// Line breaks and tabs become spaces, as gloss's status bar treats them.
func clean(s string) string {
	if s == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case '\n', '\r', '\t', '\v', '\f':
			b.WriteByte(' ')
			continue
		}
		if unicode.IsPrint(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// cleanSpec cleans every string the terminal will draw from a Spec: titles,
// axis labels, series names and category values. The one choke point for
// inferred charts and chart files alike.
func cleanSpec(s *spec.Spec) {
	s.Title, s.Subtitle = clean(s.Title), clean(s.Subtitle)
	cleanAxis := func(title *string, labels []string) {
		*title = clean(*title)
		for i := range labels {
			labels[i] = clean(labels[i])
		}
	}
	cleanAxis(&s.XAxis.Title, s.XAxis.Labels)
	cleanAxis(&s.YAxis.Title, s.YAxis.Labels)
	if s.Y2Axis != nil {
		y2 := *s.Y2Axis
		cleanAxis(&y2.Title, y2.Labels)
		s.Y2Axis = &y2
	}
	for i := range s.Data.XAxisData {
		if text, ok := s.Data.XAxisData[i].(string); ok {
			s.Data.XAxisData[i] = clean(text)
		}
	}
	for i := range s.Data.Series {
		series := &s.Data.Series[i]
		series.Name = clean(series.Name)
		for j := range series.Values {
			if text, ok := series.Values[j].X.(string); ok {
				series.Values[j].X = clean(text)
			}
		}
	}
}
