package log

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

type baseTestCase struct {
	description   string
	message       string
	suffix        string
	level         logLevel
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

func TestLoggerDebug(t *testing.T) {
	for _, testCase := range baseTestCases {
		testCase := testCase
		testCase.level = debugLevel
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

func TestLoggerInfo(t *testing.T) {
	for _, testCase := range baseTestCases {
		testCase := testCase
		testCase.level = infoLevel
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

func TestLoggerWarn(t *testing.T) {
	for _, testCase := range baseTestCases {
		testCase := testCase
		testCase.level = warnLevel
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

type formatTestCase struct {
	description   string
	format        string
	args          []interface{}
	level         logLevel
	wantedPattern string
}

var formatTestCases = []formatTestCase{
	{
		description: "message with string format",
		format:      "hello, %s!",
		args:        []any{"Hacker"},
	},
	{
		description: "message with int format",
		format:      "hello, %d,%d,%d!",
		args:        []any{1, 2, 3},
	},
	{
		description: "message with mixed format",
		format:      "hello, %s,%d,%d!",
		args:        []any{"Hacker", 2, 3},
	},
}

func TestLoggerDebugf(t *testing.T) {
	for _, testCase := range formatTestCases {
		testCase := testCase
		testCase.level = debugLevel
		message := fmt.Sprintf(testCase.format, testCase.args...)
		testCase.wantedPattern = fmt.Sprintf(`^\[DBG\] \w+\.go:\d+: %s\n$`, message)
		t.Run(testCase.description, func(t *testing.T) {
			var buffer bytes.Buffer
			testLogger := newLogger(&buffer)
			testLogger.setLevel(debugLevel)
			testLogger.Debugf(testCase.format, testCase.args...)
			got := buffer.String()
			assert.Regexp(t, testCase.wantedPattern, got)
		})
	}
}

func TestLoggerInfof(t *testing.T) {
	for _, testCase := range formatTestCases {
		testCase := testCase
		testCase.level = infoLevel
		message := fmt.Sprintf(testCase.format, testCase.args...)
		testCase.wantedPattern = fmt.Sprintf(`^\[INF\] %s\n$`, message)
		t.Run(testCase.description, func(t *testing.T) {
			var buffer bytes.Buffer
			testLogger := newLogger(&buffer)
			testLogger.Infof(testCase.format, testCase.args...)
			got := buffer.String()
			assert.Regexp(t, testCase.wantedPattern, got)
		})
	}
}

func TestLoggerWarnf(t *testing.T) {
	for _, testCase := range formatTestCases {
		testCase := testCase
		testCase.level = warnLevel
		message := fmt.Sprintf(testCase.format, testCase.args...)
		testCase.wantedPattern = fmt.Sprintf(`^\[WRN\] %s\n$`, message)
		t.Run(testCase.description, func(t *testing.T) {
			var buffer bytes.Buffer
			testLogger := newLogger(&buffer)
			testLogger.Warnf(testCase.format, testCase.args...)
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

func TestInfoHasLabel(t *testing.T) {
	var buffer bytes.Buffer
	testLogger := newLogger(&buffer)
	testLogger.Infof("clean message")
	assert.Equal(t, "[INF] clean message\n", buffer.String())
}

func TestMultilineMessageIndented(t *testing.T) {
	var buffer bytes.Buffer
	testLogger := newLogger(&buffer)
	testLogger.Warnf("line1\nline2\nline3")
	got := buffer.String()
	assert.Equal(t, "[WRN] line1\n      line2\n      line3\n", got)
}
