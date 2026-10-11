//go:build !js

package app

import (
	tea "charm.land/bubbletea/v2"
	"fmt"
	"nimbleterminal.dev/gloss/internal/browse"
	"nimbleterminal.dev/gloss/internal/document"
	"nimbleterminal.dev/gloss/internal/openwith"
	"os"
	"path/filepath"
)

func openWithTarget(path, kind string, directory bool) (openwith.Target, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return openwith.Target{}, err
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return openwith.Target{}, err
	}
	if info.IsDir() != directory || (!directory && !info.Mode().IsRegular()) {
		return openwith.Target{}, fmt.Errorf("not a regular file or selected directory")
	}
	if kind == "" && !directory {
		kind, _ = document.Probe(absolute, "")
	}
	return openwith.Target{Path: filepath.Clean(absolute), Kind: openwith.Kind(kind), Directory: directory}, nil
}
func (m *Model) openWithBrowse(msg tea.Msg) (tea.Cmd, bool) {
	v, ok := msg.(browse.OpenWithMsg)
	if !ok {
		return nil, false
	}
	if !m.browsing() || m.opener.find != nil || m.opener.going {
		return nil, true
	}
	return m.requestOpenWith(filepath.FromSlash(v.Path), "", v.Directory, true), true
}
