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
	level         Level
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
	for _, tc := range baseTestCases {
		tc := tc
		tc.level = DebugLevel
		message := tc.message + tc.suffix
		tc.wantedPattern = fmt.Sprintf(`^\[DBG\] \w+\.go:\d+: %s\n$`, tc.message)
		t.Run(tc.description, func(t *testing.T) {
			var buf bytes.Buffer
			logger := NewLogger(newBase(&buf))
			logger.SetLevel(DebugLevel)
			logger.Debugf("%s", message)
			got := buf.String()
			assert.Regexp(t, tc.wantedPattern, got)
		})
	}
}

func TestLoggerInfo(t *testing.T) {
	for _, tc := range baseTestCases {
		tc := tc
		tc.level = InfoLevel
		message := tc.message + tc.suffix
		tc.wantedPattern = fmt.Sprintf(`^\[INF\] %s\n$`, tc.message)
		t.Run(tc.description, func(t *testing.T) {
			var buf bytes.Buffer
			logger := NewLogger(newBase(&buf))
			logger.Infof("%s", message)
			got := buf.String()
			assert.Regexp(t, tc.wantedPattern, got)
		})
	}
}

func TestLoggerWarn(t *testing.T) {
	for _, tc := range baseTestCases {
		tc := tc
		tc.level = WarnLevel
		message := tc.message + tc.suffix
		tc.wantedPattern = fmt.Sprintf(`^\[WRN\] %s\n$`, tc.message)
		t.Run(tc.description, func(t *testing.T) {
			var buf bytes.Buffer
			logger := NewLogger(newBase(&buf))
			logger.Warnf("%s", message)
			got := buf.String()
			assert.Regexp(t, tc.wantedPattern, got)
		})
	}
}

type formatTestCase struct {
	description   string
	format        string
	args          []interface{}
	level         Level
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
	for _, tc := range formatTestCases {
		tc := tc
		tc.level = DebugLevel
		message := fmt.Sprintf(tc.format, tc.args...)
		tc.wantedPattern = fmt.Sprintf(`^\[DBG\] \w+\.go:\d+: %s\n$`, message)
		t.Run(tc.description, func(t *testing.T) {
			var buf bytes.Buffer
			logger := NewLogger(newBase(&buf))
			logger.SetLevel(DebugLevel)
			logger.Debugf(tc.format, tc.args...)
			got := buf.String()
			assert.Regexp(t, tc.wantedPattern, got)
		})
	}
}

func TestLoggerInfof(t *testing.T) {
	for _, tc := range formatTestCases {
		tc := tc
		tc.level = InfoLevel
		message := fmt.Sprintf(tc.format, tc.args...)
		tc.wantedPattern = fmt.Sprintf(`^\[INF\] %s\n$`, message)
		t.Run(tc.description, func(t *testing.T) {
			var buf bytes.Buffer
			logger := NewLogger(newBase(&buf))
			logger.Infof(tc.format, tc.args...)
			got := buf.String()
			assert.Regexp(t, tc.wantedPattern, got)
		})
	}
}

func TestLoggerWarnf(t *testing.T) {
	for _, tc := range formatTestCases {
		tc := tc
		tc.level = WarnLevel
		message := fmt.Sprintf(tc.format, tc.args...)
		tc.wantedPattern = fmt.Sprintf(`^\[WRN\] %s\n$`, message)
		t.Run(tc.description, func(t *testing.T) {
			var buf bytes.Buffer
			logger := NewLogger(newBase(&buf))
			logger.Warnf(tc.format, tc.args...)
			got := buf.String()
			assert.Regexp(t, tc.wantedPattern, got)
		})
	}
}

func TestLoggerWithLowerLevels(t *testing.T) {
	levels := []Level{DebugLevel, InfoLevel, WarnLevel}
	ops := []struct {
		op      string
		level   Level
		logFunc func(*Logger)
	}{
		{"Debugf", DebugLevel, func(l *Logger) { l.Debugf("hello") }},
		{"Infof", InfoLevel, func(l *Logger) { l.Infof("hello") }},
		{"Warnf", WarnLevel, func(l *Logger) { l.Warnf("hello") }},
	}

	for _, setLevel := range levels {
		for _, op := range ops {
			var buf bytes.Buffer
			logger := NewLogger(newBase(&buf))
			logger.SetLevel(setLevel)

			expectedOutput := op.level >= setLevel
			op.logFunc(logger)

			output := buf.String()
			if expectedOutput {
				assert.NotEmpty(t, output, "setLevel=%s op=%s should produce output", setLevel, op.op)
			} else {
				assert.Empty(t, output, "setLevel=%s op=%s should be suppressed", setLevel, op.op)
			}
		}
	}
}

func TestDefaultLevelIsInfo(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(newBase(&buf))

	// Debug should be suppressed at default level (InfoLevel).
	logger.Debugf("debug msg")
	assert.Empty(t, buf.String(), "Debug should be suppressed at default InfoLevel")

	// Info should be visible at default level.
	logger.Infof("info msg")
	assert.Contains(t, buf.String(), "info msg")
}

func TestDebugIncludesFileLine(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(newBase(&buf))
	logger.SetLevel(DebugLevel)
	logger.Debugf("test location")
	got := buf.String()
	assert.Regexp(t, `^\[DBG\] logger_test\.go:\d+: test location\n$`, got)
}

func TestInfoHasLabel(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(newBase(&buf))
	logger.Infof("clean message")
	assert.Equal(t, "[INF] clean message\n", buf.String())
}

func TestMultilineMessageIndented(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(newBase(&buf))
	logger.Warnf("line1\nline2\nline3")
	got := buf.String()
	assert.Equal(t, "[WRN] line1\n      line2\n      line3\n", got)
}
