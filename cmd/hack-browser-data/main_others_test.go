//go:build !windows

package main

import (
	"os"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func assertEndedByInterrupt(t *testing.T, ps *os.ProcessState) {
	t.Helper()
	status, ok := ps.Sys().(syscall.WaitStatus)
	require.True(t, ok)
	assert.True(t, status.Signaled(), "ended by the signal, not by an exit code")
	assert.Equal(t, syscall.SIGINT, status.Signal())
}
