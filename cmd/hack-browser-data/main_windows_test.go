//go:build windows

package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/sys/windows"
)

func assertEndedByInterrupt(t *testing.T, ps *os.ProcessState) {
	t.Helper()
	assert.Equal(t, uint32(windows.STATUS_CONTROL_C_EXIT), uint32(ps.ExitCode()))
}
