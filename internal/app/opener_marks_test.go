package app

import (
	"slices"
	"testing"

	"nimbleterminal.dev/gloss/internal/document"
)

// The chooser has no marks of its own: every extension gloss opens must have
// one here, and every mark a kind in the file-types menu, or a new format is
// drawn blank and cannot be shown alone.
func TestEveryFormatHasAMarkAndAKind(t *testing.T) {
	for _, ext := range document.Extensions {
		m, ok := kindMarks[ext]
		if !ok {
			t.Errorf("%s has no mark", ext)
			continue
		}
		if !slices.ContainsFunc(typeFilters, func(f typeFilter) bool { return slices.Contains(f.marks, m) }) {
			t.Errorf("%s (%s) is in no kind of file", ext, m)
		}
	}
	for ext := range kindMarks {
		if !slices.Contains(document.Extensions, ext) {
			t.Errorf("%s has a mark but gloss does not open it", ext)
		}
	}
}
