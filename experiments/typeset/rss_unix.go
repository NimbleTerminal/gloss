//go:build darwin || linux

package main

import (
	"runtime"
	"syscall"
)

func peakRSS() uint64 {
	var r syscall.Rusage
	if syscall.Getrusage(syscall.RUSAGE_SELF, &r) != nil {
		return 0
	}
	rss := uint64(r.Maxrss)
	if runtime.GOOS == "linux" {
		rss *= 1024
	}
	return rss
}
