package main

import (
	"io"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestParseTimes(t *testing.T) {
	got, err := parseTimes("1:23, 83.5,1:02:03,1m30s,0,1500ms")
	if err != nil {
		t.Fatal(err)
	}
	want := []time.Duration{83 * time.Second, 83500 * time.Millisecond, 3723 * time.Second, 90 * time.Second, 0, 1500 * time.Millisecond}
	if !slices.Equal(got, want) {
		t.Fatalf("parseTimes = %v, want %v", got, want)
	}
	for _, bad := range []string{"", "1:", "1:75", "1.5:20", "-3", "1e3", "abc", "-1s", "1:2:3:4", "1,,2", strings.Repeat("1,", 17) + "1"} {
		if _, err := parseTimes(bad); err == nil {
			t.Errorf("parseTimes(%q) took it", bad)
		}
	}
}

func TestVideoOptions(t *testing.T) {
	for _, tc := range []struct {
		args []string
		bad  string
	}{
		{[]string{"--at", "1:23", "clip.mp4"}, ""},
		{[]string{"--at", "1,2", "-o", "sheet.png", "clip.mp4"}, ""},
		{[]string{"--frames", "9", "--print", "clip.mp4"}, ""},
		{[]string{"--at", "1,2", "clip.mp4"}, "several frames need"},
		{[]string{"--frames", "4", "clip.mp4"}, "several frames need"},
		{[]string{"--frames", "17", "-o", "x.png", "clip.mp4"}, "1..16"},
		{[]string{"--at", "1", "--frames", "2", "-o", "x.png", "clip.mp4"}, "choose --at or --frames"},
		{[]string{"--at", "1", "--text", "clip.mp4"}, "--text gives"},
	} {
		opts, _, err := parse(tc.args, io.Discard)
		switch {
		case tc.bad == "" && err != nil:
			t.Errorf("%v: %v", tc.args, err)
		case tc.bad != "" && (err == nil || !strings.Contains(err.Error(), tc.bad)):
			t.Errorf("%v: err=%v, want %q", tc.args, err, tc.bad)
		case tc.bad == "" && tc.args[0] == "--at" && len(opts.Times) == 0:
			t.Errorf("%v: no times", tc.args)
		}
	}
}
