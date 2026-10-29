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
func exitBy(sig os.Signal) {
	signal.Reset(sig)
	if s, ok := sig.(syscall.Signal); ok && syscall.Kill(syscall.Getpid(), s) == nil {
		time.Sleep(time.Second) // the signal ends the process first unless it was ignored at start
	}
	os.Exit(1)
}
