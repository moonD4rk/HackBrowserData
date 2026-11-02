//go:build !windows

package main

import (
	"os"
	"os/signal"
	"syscall"
	"time"
)

func configureDoubleClickMode() {}

// exitBy ends the process by sig itself, as the signal would have, so a calling shell or script sees it was stopped.
func exitBy(received os.Signal) {
	signal.Reset(received)
	if syscallSignal, ok := received.(syscall.Signal); ok && syscall.Kill(syscall.Getpid(), syscallSignal) == nil {
		time.Sleep(time.Second) // the signal ends the process first unless it was ignored at start
	}
	os.Exit(1)
}
