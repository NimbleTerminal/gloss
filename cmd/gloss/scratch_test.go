package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScratchNeverOverwrites(t *testing.T) {
	var s scratchFolder
	defer s.remove()
	a, err := s.write("table.csv", []byte("a"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.write("table.csv", []byte("b"))
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatalf("both writes went to %s", a)
	}
	for path, want := range map[string]string{a: "a", b: "b"} {
		if !s.holds(path) {
			t.Errorf("folder does not hold %s", path)
		}
		if got, err := os.ReadFile(path); err != nil || string(got) != want {
			t.Errorf("%s = %q, %v; want %q", path, got, err, want)
		}
	}
	if s.holds(filepath.Join(filepath.Dir(a), "..", "other.csv")) {
		t.Error("folder holds a path outside it")
	}
	dir := filepath.Dir(a)
	s.remove()
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("folder still there after remove: %v", err)
	}
}

func TestScratchHoldsNothingBeforeUse(t *testing.T) {
	var s scratchFolder
	if s.holds("/anything") {
		t.Error("an unused folder holds a path")
	}
	s.remove() // Harmless when never made.
}

func TestScratchUsesOnlyTheName(t *testing.T) {
	var s scratchFolder
	defer s.remove()
	path, err := s.write("../../escape.txt", []byte("x"))
	if err != nil {
		t.Fatal(err)
	}
	if !s.holds(path) {
		t.Errorf("%s escaped the folder", path)
	}
}
