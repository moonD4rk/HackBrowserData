package log

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetVerbose(t *testing.T) {
	t.Cleanup(func() { defaultLogger.setLevel(infoLevel) })

	assert.False(t, defaultLogger.enabled(debugLevel), "Debug must be suppressed by default")
	SetVerbose()
	assert.True(t, defaultLogger.enabled(debugLevel), "SetVerbose must enable Debug")
}
