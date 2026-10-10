//go:build !darwin && !linux

package main

// Unsupported hosts report zero rather than a Go-heap estimate of process RSS.
func peakRSS() uint64 { return 0 }
