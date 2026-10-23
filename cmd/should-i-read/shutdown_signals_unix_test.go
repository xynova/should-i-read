//go:build unix

package main

import (
	"os"
	"syscall"
	"testing"
)

func TestShutdownSignals_includesTermAndHangup(t *testing.T) {
	got := shutdownSignals()
	want := map[os.Signal]bool{
		os.Interrupt:    false,
		syscall.SIGTERM: false,
		syscall.SIGHUP:  false,
	}
	for _, sig := range got {
		if _, ok := want[sig]; ok {
			want[sig] = true
		}
	}
	for sig, seen := range want {
		if !seen {
			t.Fatalf("missing %v", sig)
		}
	}
}
