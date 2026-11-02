package log

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetVerbose(t *testing.T) {
	t.Cleanup(func() { defaultLogger.SetLevel(InfoLevel) })

	assert.False(t, defaultLogger.canLogAt(DebugLevel), "Debug must be suppressed by default")
	SetVerbose()
	assert.True(t, defaultLogger.canLogAt(DebugLevel), "SetVerbose must enable Debug")
}

func TestSetLevelInvalidPanics(t *testing.T) {
	testLogger := newLogger(io.Discard)
	assert.Panics(t, func() { testLogger.SetLevel(DebugLevel - 1) })
	assert.Panics(t, func() { testLogger.SetLevel(WarnLevel + 1) })
}
