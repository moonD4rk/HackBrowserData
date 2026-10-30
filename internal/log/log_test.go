package log

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetVerbose(t *testing.T) {
	t.Cleanup(func() { defaultLogger.SetLevel(InfoLevel) }) // restore package default

	assert.False(t, defaultLogger.canLogAt(DebugLevel), "Debug must be suppressed by default")
	SetVerbose()
	assert.True(t, defaultLogger.canLogAt(DebugLevel), "SetVerbose must enable Debug")
}

func TestSetLevelInvalidPanics(t *testing.T) {
	logger := NewLogger(newBase(io.Discard))
	assert.Panics(t, func() { logger.SetLevel(DebugLevel - 1) })
	assert.Panics(t, func() { logger.SetLevel(WarnLevel + 1) })
}
