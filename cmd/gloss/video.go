package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"nimbleterminal.dev/gloss/internal/document"
)

// parseTimes reads --at: moments of a video, separated by commas, each as
// a clock (1:23, 1:02:03, 1:23.5), seconds (83, 83.5) or a Go duration
// (83s, 1m23s, 1500ms).
func parseTimes(s string) ([]time.Duration, error) {
	var times []time.Duration
	for _, field := range strings.Split(s, ",") {
		field = strings.TrimSpace(field)
		t, err := parseTime(field)
		if err != nil {
			return nil, fmt.Errorf("--at %q: %w", field, err)
		}
		times = append(times, t)
	}
	if len(times) > document.MaxFrames {
		return nil, fmt.Errorf("--at names %d times; at most %d", len(times), document.MaxFrames)
	}
	return times, nil
}

func parseTime(s string) (time.Duration, error) {
	if s == "" {
		return 0, fmt.Errorf("no time")
	}
	if strings.ContainsAny(s, "hmsµun") {
		t, err := time.ParseDuration(s)
		if err != nil || t < 0 {
			return 0, fmt.Errorf("not a time: write 1:23, 83.5 or 1m23s")
		}
		return t, nil
	}
	parts := strings.Split(s, ":")
	if len(parts) > 3 {
		return 0, fmt.Errorf("not a time: write 1:23, 83.5 or 1m23s")
	}
	var total float64
	for i, part := range parts {
		n, err := strconv.ParseFloat(part, 64)
		last := i == len(parts)-1
		// Only the seconds have a fraction; minutes and seconds after a
		// colon are under 60.
		if err != nil || n < 0 || strings.ContainsAny(part, "eE+-") || !last && strings.Contains(part, ".") || i > 0 && n >= 60 {
			return 0, fmt.Errorf("not a time: write 1:23, 83.5 or 1m23s")
		}
		total = total*60 + n
	}
	if total > float64(1<<62)/float64(time.Second) {
		return 0, fmt.Errorf("too long")
	}
	return time.Duration(total * float64(time.Second)), nil
}
