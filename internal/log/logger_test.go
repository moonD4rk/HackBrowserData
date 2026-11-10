package log

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buffer bytes.Buffer
	saved := defaultLogger
	defaultLogger = newLogger(&buffer)
	t.Cleanup(func() { defaultLogger = saved })
	return &buffer
}

type baseTestCase struct {
	description   string
	message       string
	suffix        string
	wantedPattern string
}

var baseTestCases = []baseTestCase{
	{
		description: "without trailing newline, logger adds newline",
		message:     "hello, hacker!",
		suffix:      "",
	},
	{
		description: "with trailing newline, logger preserves newline",
		message:     "hello, hacker!",
		suffix:      "\n",
	},
}

func TestLoggerDebugf(t *testing.T) {
	for _, testCase := range baseTestCases {
		testCase := testCase
		message := testCase.message + testCase.suffix
		testCase.wantedPattern = fmt.Sprintf(`^\[DBG\] \w+\.go:\d+: %s\n$`, testCase.message)
		t.Run(testCase.description, func(t *testing.T) {
			buffer := captureLog(t)
			defaultLogger.setLevel(debugLevel)
			Debugf("%s", message)
			assert.Regexp(t, testCase.wantedPattern, buffer.String())
		})
	}
}

func TestLoggerInfof(t *testing.T) {
	for _, testCase := range baseTestCases {
		testCase := testCase
		message := testCase.message + testCase.suffix
		testCase.wantedPattern = fmt.Sprintf(`^\[INF\] %s\n$`, testCase.message)
		t.Run(testCase.description, func(t *testing.T) {
			buffer := captureLog(t)
			Infof("%s", message)
			assert.Regexp(t, testCase.wantedPattern, buffer.String())
		})
	}
}

func TestLoggerWarnf(t *testing.T) {
	for _, testCase := range baseTestCases {
		testCase := testCase
		message := testCase.message + testCase.suffix
		testCase.wantedPattern = fmt.Sprintf(`^\[WRN\] %s\n$`, testCase.message)
		t.Run(testCase.description, func(t *testing.T) {
			buffer := captureLog(t)
			Warnf("%s", message)
			assert.Regexp(t, testCase.wantedPattern, buffer.String())
		})
	}
}

func TestLoggerWithLowerLevels(t *testing.T) {
	levels := []logLevel{debugLevel, infoLevel, warnLevel}
	ops := []struct {
		name    string
		level   logLevel
		logFunc func()
	}{
		{"Debugf", debugLevel, func() { Debugf("hello") }},
		{"Infof", infoLevel, func() { Infof("hello") }},
		{"Warnf", warnLevel, func() { Warnf("hello") }},
	}

	for _, setLevel := range levels {
		for _, operation := range ops {
			buffer := captureLog(t)
			defaultLogger.setLevel(setLevel)

			expectedOutput := operation.level >= setLevel
			operation.logFunc()

			output := buffer.String()
			if expectedOutput {
				assert.NotEmpty(t, output, "setLevel=%s op=%s should produce output", setLevel, operation.name)
			} else {
				assert.Empty(t, output, "setLevel=%s op=%s should be suppressed", setLevel, operation.name)
			}
		}
	}
}

func TestDefaultLevelIsInfo(t *testing.T) {
	buffer := captureLog(t)

	Debugf("debug msg")
	assert.Empty(t, buffer.String(), "Debug should be suppressed at default infoLevel")

	Infof("info msg")
	assert.Contains(t, buffer.String(), "info msg")
}

func TestDebugIncludesFileLine(t *testing.T) {
	buffer := captureLog(t)
	defaultLogger.setLevel(debugLevel)
	Debugf("test location")
	assert.Regexp(t, `^\[DBG\] logger_test\.go:\d+: test location\n$`, buffer.String())
}

func TestMultilineMessageIndented(t *testing.T) {
	buffer := captureLog(t)
	Warnf("line1\nline2\nline3")
	assert.Equal(t, "[WRN] line1\n      line2\n      line3\n", buffer.String())
}

func TestSetLevelInvalidPanics(t *testing.T) {
	testLogger := newLogger(io.Discard)
	assert.Panics(t, func() { testLogger.setLevel(debugLevel - 1) })
	assert.Panics(t, func() { testLogger.setLevel(warnLevel + 1) })
}
