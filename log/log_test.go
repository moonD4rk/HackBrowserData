package log

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetLevel(t *testing.T) {
	t.Cleanup(func() { SetLevel(InfoLevel) }) // restore package default

	SetLevel(FatalLevel)
	assert.False(t, defaultLogger.canLogAt(InfoLevel), "Info must be suppressed at FatalLevel")
	assert.False(t, defaultLogger.canLogAt(WarnLevel), "Warn must be suppressed at FatalLevel")
	assert.False(t, defaultLogger.canLogAt(ErrorLevel), "Error must be suppressed at FatalLevel")
	assert.True(t, defaultLogger.canLogAt(FatalLevel), "Fatal must still log at FatalLevel")

	SetVerbose()
	assert.True(t, defaultLogger.canLogAt(DebugLevel), "SetVerbose must enable Debug")
}

func TestSetLevelInvalidPanics(t *testing.T) {
	t.Cleanup(func() { SetLevel(InfoLevel) })
	assert.Panics(t, func() { SetLevel(FatalLevel + 1) })
}
