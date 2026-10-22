package main

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"nimbleterminal.dev/gloss/internal/document"
)

// scratchFolder is the viewer's folder of its own temporaries, such as a
// table written out so another program can open it. It is made when a file
// is first written and removed when the session ends. The viewer writes
// from commands while the model asks what is temporary, so it is locked.
type scratchFolder struct {
	mu  sync.Mutex
	dir string
}

// write puts data in a new file called name, or the nearest free name, and
// gives its path. Only the last element of name is used.
func (s *scratchFolder) write(name string, data []byte) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dir == "" {
		dir, err := os.MkdirTemp("", "gloss-scratch-")
		if err != nil {
			return "", err
		}
		s.dir = dir
	}
	name = filepath.Base(name)
	if name == "." || name == ".." || name == string(filepath.Separator) {
		name = "scratch"
	}
	return document.WriteNew(filepath.Join(s.dir, name), data)
}

// holds reports whether a path is in the folder, that is, a temporary.
func (s *scratchFolder) holds(path string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dir == "" {
		return false
	}
	rel, err := filepath.Rel(s.dir, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// remove deletes the folder and what is in it.
func (s *scratchFolder) remove() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dir != "" {
		_ = os.RemoveAll(s.dir)
		s.dir = ""
	}
}
