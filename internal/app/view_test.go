package app

import "testing"

func TestSelectHint(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "ghostty")
	if got := selectHint(); got != "Shift-drag select" {
		t.Fatalf("ghostty: %q", got)
	}
	t.Setenv("TERM_PROGRAM", "iTerm.app")
	if got := selectHint(); got != "⌥-drag select" {
		t.Fatalf("iTerm2: %q", got)
	}
}
