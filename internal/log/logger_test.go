package log

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

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
			var buffer bytes.Buffer
			testLogger := newLogger(&buffer)
			testLogger.setLevel(debugLevel)
			testLogger.Debugf("%s", message)
			got := buffer.String()
			assert.Regexp(t, testCase.wantedPattern, got)
		})
	}
}

func TestLoggerInfof(t *testing.T) {
	for _, testCase := range baseTestCases {
		testCase := testCase
		message := testCase.message + testCase.suffix
		testCase.wantedPattern = fmt.Sprintf(`^\[INF\] %s\n$`, testCase.message)
		t.Run(testCase.description, func(t *testing.T) {
			var buffer bytes.Buffer
			testLogger := newLogger(&buffer)
			testLogger.Infof("%s", message)
			got := buffer.String()
			assert.Regexp(t, testCase.wantedPattern, got)
		})
	}
}

func TestLoggerWarnf(t *testing.T) {
	for _, testCase := range baseTestCases {
		testCase := testCase
		message := testCase.message + testCase.suffix
		testCase.wantedPattern = fmt.Sprintf(`^\[WRN\] %s\n$`, testCase.message)
		t.Run(testCase.description, func(t *testing.T) {
			var buffer bytes.Buffer
			testLogger := newLogger(&buffer)
			testLogger.Warnf("%s", message)
			got := buffer.String()
			assert.Regexp(t, testCase.wantedPattern, got)
		})
	}
}

func TestLoggerWithLowerLevels(t *testing.T) {
	levels := []logLevel{debugLevel, infoLevel, warnLevel}
	ops := []struct {
		name    string
		level   logLevel
		logFunc func(*logger)
	}{
		{"Debugf", debugLevel, func(testLogger *logger) { testLogger.Debugf("hello") }},
		{"Infof", infoLevel, func(testLogger *logger) { testLogger.Infof("hello") }},
		{"Warnf", warnLevel, func(testLogger *logger) { testLogger.Warnf("hello") }},
	}

	for _, setLevel := range levels {
		for _, operation := range ops {
			var buffer bytes.Buffer
			testLogger := newLogger(&buffer)
			testLogger.setLevel(setLevel)

			expectedOutput := operation.level >= setLevel
			operation.logFunc(testLogger)

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
	var buffer bytes.Buffer
	testLogger := newLogger(&buffer)

	testLogger.Debugf("debug msg")
	assert.Empty(t, buffer.String(), "Debug should be suppressed at default infoLevel")

	testLogger.Infof("info msg")
	assert.Contains(t, buffer.String(), "info msg")
}

func TestDebugIncludesFileLine(t *testing.T) {
	var buffer bytes.Buffer
	testLogger := newLogger(&buffer)
	testLogger.setLevel(debugLevel)
	testLogger.Debugf("test location")
	got := buffer.String()
	assert.Regexp(t, `^\[DBG\] logger_test\.go:\d+: test location\n$`, got)
}

func TestMultilineMessageIndented(t *testing.T) {
	var buffer bytes.Buffer
	testLogger := newLogger(&buffer)
	testLogger.Warnf("line1\nline2\nline3")
	got := buffer.String()
	assert.Equal(t, "[WRN] line1\n      line2\n      line3\n", got)
}

func TestSetLevelInvalidPanics(t *testing.T) {
	testLogger := newLogger(io.Discard)
	assert.Panics(t, func() { testLogger.setLevel(debugLevel - 1) })
	assert.Panics(t, func() { testLogger.setLevel(warnLevel + 1) })
}
