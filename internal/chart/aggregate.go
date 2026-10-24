package chart

import (
	"encoding/json"
	"fmt"
	"sort"
)

// aggregate folds a Flint document's rows for its aggregate encodings
// (sum, mean/average, count, min, max, median), grouping by every other
// encoded field, and drops the aggregate directives, so the compiler sees
// plain rows. The text backend of flint-ntcharts 0.3.1 writes an aggregate
// into the field it reads from, then reads field_op, which comes out zero;
// folding here draws the right bars whichever backend compiles the rest. A
// document without aggregates is returned untouched.
func aggregate(raw []byte) ([]byte, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil || top["chart_spec"] == nil {
		return raw, nil
	}
	var chartSpec map[string]json.RawMessage
	if err := json.Unmarshal(top["chart_spec"], &chartSpec); err != nil || chartSpec["encodings"] == nil {
		return raw, nil
	}
	var encodings map[string]json.RawMessage
	if err := json.Unmarshal(chartSpec["encodings"], &encodings); err != nil {
		return raw, nil
	}
	type encoding struct {
		Field     string `json:"field"`
		Aggregate string `json:"aggregate"`
	}
	var groupBy []string
	folds := map[string]string{} // Field to op, for the aggregated fields.
	decoded := map[string]map[string]any{}
	for channel, rawEnc := range encodings {
		var enc encoding
		var shorthand string
		if json.Unmarshal(rawEnc, &shorthand) == nil {
			enc.Field = shorthand
		} else if err := json.Unmarshal(rawEnc, &enc); err != nil {
			return raw, nil
		}
		var full map[string]any
		_ = json.Unmarshal(rawEnc, &full)
		decoded[channel] = full
		switch {
		case enc.Field == "":
		case enc.Aggregate != "":
			folds[enc.Field] = enc.Aggregate
		default:
			groupBy = append(groupBy, enc.Field)
		}
	}
	if len(folds) == 0 {
		return raw, nil
	}
	var data struct {
		Values []map[string]any `json:"values"`
	}
	if top["data"] == nil || json.Unmarshal(top["data"], &data) != nil {
		return raw, nil
	}
	sort.Strings(groupBy)
	type bucket struct {
		first   map[string]any
		numbers map[string][]float64
		count   int
	}
	var order []string
	buckets := map[string]*bucket{}
	for _, row := range data.Values {
		keyParts := make([]any, len(groupBy))
		for i, f := range groupBy {
			keyParts[i] = row[f]
		}
		key, _ := json.Marshal(keyParts)
		b := buckets[string(key)]
		if b == nil {
			b = &bucket{first: row, numbers: map[string][]float64{}}
			buckets[string(key)] = b
			order = append(order, string(key))
		}
		b.count++
		for field := range folds {
			if v, ok := row[field].(float64); ok {
				b.numbers[field] = append(b.numbers[field], v)
			}
		}
	}
	folded := make([]map[string]any, 0, len(order))
	for _, key := range order {
		b := buckets[key]
		row := map[string]any{}
		for _, f := range groupBy {
			row[f] = b.first[f]
		}
		for field, op := range folds {
			v, err := fold(op, b.numbers[field], b.count)
			if err != nil {
				return nil, err
			}
			row[field] = v
		}
		folded = append(folded, row)
	}
	for channel, full := range decoded {
		if full == nil {
			continue
		}
		delete(full, "aggregate")
		enc, err := json.Marshal(full)
		if err != nil {
			return nil, err
		}
		encodings[channel] = enc
	}
	var err error
	if chartSpec["encodings"], err = json.Marshal(encodings); err != nil {
		return nil, err
	}
	if top["chart_spec"], err = json.Marshal(chartSpec); err != nil {
		return nil, err
	}
	if top["data"], err = json.Marshal(map[string]any{"values": folded}); err != nil {
		return nil, err
	}
	return json.Marshal(top)
}

// fold reduces a bucket's numbers with op.
func fold(op string, numbers []float64, count int) (float64, error) {
	if op == "count" {
		return float64(count), nil
	}
	if len(numbers) == 0 {
		return 0, nil
	}
	switch op {
	case "sum", "mean", "average":
		total := 0.0
		for _, v := range numbers {
			total += v
		}
		if op == "sum" {
			return total, nil
		}
		return total / float64(len(numbers)), nil
	case "min", "max":
		best := numbers[0]
		for _, v := range numbers[1:] {
			if (op == "min" && v < best) || (op == "max" && v > best) {
				best = v
			}
		}
		return best, nil
	case "median":
		sorted := append([]float64(nil), numbers...)
		sort.Float64s(sorted)
		n := len(sorted)
		if n%2 == 1 {
			return sorted[n/2], nil
		}
		return (sorted[n/2-1] + sorted[n/2]) / 2, nil
	}
	return 0, fmt.Errorf("unknown aggregate %q: use sum, mean, average, count, min, max, or median", op)
}
