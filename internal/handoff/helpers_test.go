package handoff

import (
	"net/http"
	"testing"

	"nimbleterminal.dev/gloss/internal/handoff/handofftest"
)

// The page, driven from outside, as the tests of the command drive it too.
type terminal = handofftest.Terminal

var (
	get     = handofftest.Get
	picture = handofftest.Picture
)

// upload is a file to drop, written as {name, data}.
type upload struct {
	name string
	data []byte
}

func drop(t *testing.T, target string, edit func(*http.Request), files ...upload) (int, []string) {
	t.Helper()
	return handofftest.Drop(t, target, edit, uploads(files)...)
}

func uploads(files []upload) []handofftest.Upload {
	out := make([]handofftest.Upload, len(files))
	for i, f := range files {
		out[i] = handofftest.Upload{Name: f.name, Data: f.data}
	}
	return out
}

func connect(t *testing.T, s *Server) *terminal {
	t.Helper()
	return handofftest.Connect(t, s.URL)
}
