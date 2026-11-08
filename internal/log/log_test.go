package log

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetVerbose(t *testing.T) {
	t.Cleanup(func() { defaultLogger.setLevel(infoLevel) })

	assert.False(t, defaultLogger.enabled(debugLevel), "Debug must be suppressed by default")
	SetVerbose()
	assert.True(t, defaultLogger.enabled(debugLevel), "SetVerbose must enable Debug")
}

func TestSetLevelInvalidPanics(t *testing.T) {
	testLogger := newLogger(io.Discard)
	assert.Panics(t, func() { testLogger.setLevel(debugLevel - 1) })
	assert.Panics(t, func() { testLogger.setLevel(warnLevel + 1) })
}
