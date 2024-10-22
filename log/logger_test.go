package log

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	level2 "github.com/moond4rk/hackbrowserdata/log/level"
)

const (
	pattern = `^\[hack\-browser\-data] \w+\.go:\d+:`
)

type baseTestCase struct {
	description   string
	message       string
	suffix        string
	level         level2.Level
	wantedPattern string
}

var (
	baseTestCases = []baseTestCase{
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
)

func TestLoggerDebug(t *testing.T) {
	for _, tc := range baseTestCases {
		tc := tc
		tc.level = level2.DebugLevel
		message := tc.message + tc.suffix
		tc.wantedPattern = fmt.Sprintf("%s %s: %s\n$", pattern, tc.level, tc.message)
		t.Run(tc.description, func(t *testing.T) {
			var buf bytes.Buffer
			logger := NewLogger(newBase(&buf))
			logger.SetLevel(level2.DebugLevel)
			logger.Debug(message)
			got := buf.String()
			assert.Regexp(t, tc.wantedPattern, got)
		})
	}
}

func TestLoggerWarn(t *testing.T) {
	for _, tc := range baseTestCases {
		tc := tc
		tc.level = level2.WarnLevel
		message := tc.message + tc.suffix
		tc.wantedPattern = fmt.Sprintf("%s %s: %s\n$", pattern, tc.level, tc.message)
		t.Run(tc.description, func(t *testing.T) {
			var buf bytes.Buffer
			logger := NewLogger(newBase(&buf))
			logger.Warn(message)
			got := buf.String()
			assert.Regexp(t, tc.wantedPattern, got)
		})
	}
}

func TestLoggerError(t *testing.T) {
	for _, tc := range baseTestCases {
		tc := tc
		tc.level = level2.ErrorLevel
		message := tc.message + tc.suffix
		tc.wantedPattern = fmt.Sprintf("%s %s: %s\n$", pattern, tc.level, tc.message)
		t.Run(tc.description, func(t *testing.T) {
			var buf bytes.Buffer
			logger := NewLogger(newBase(&buf))
			logger.Error(message)
			got := buf.String()
			assert.Regexp(t, tc.wantedPattern, got)
		})
	}
}

func TestLoggerFatal(t *testing.T) {
	originalOsExit := osExit
	defer func() { osExit = originalOsExit }()

	for _, tc := range baseTestCases {
		tc := tc
		tc.level = level2.FatalLevel
		message := tc.message + tc.suffix
		tc.wantedPattern = fmt.Sprintf("%s %s: %s\n$", pattern, tc.level, tc.message)
		t.Run(tc.description, func(t *testing.T) {
			var buf bytes.Buffer
			exitCalled := false
			exitCode := 0
			osExit = func(code int) {
				exitCalled = true
				exitCode = code
			}
			logger := NewLogger(newBase(&buf))
			logger.Fatal(message)
			got := buf.String()
			assert.Regexp(t, tc.wantedPattern, got)
			assert.True(t, exitCalled)
			assert.Equal(t, 1, exitCode)
		})
	}
}

type formatTestCase struct {
	description   string
	format        string
	args          []interface{}
	level         level2.Level
	wantedPattern string
}

var (
	formatTestCases = []formatTestCase{
		{
			description: "message with format prefix",
			format:      "hello, %s!",
			args:        []any{"Hacker"},
		},
		{
			description: "message with format prefix",
			format:      "hello, %d,%d,%d!",
			args:        []any{1, 2, 3},
		},
		{
			description: "message with format prefix",
			format:      "hello, %s,%d,%d!",
			args:        []any{"Hacker", 2, 3},
		},
	}
)

func TestLoggerDebugf(t *testing.T) {
	for _, tc := range formatTestCases {
		tc := tc
		tc.level = level2.DebugLevel
		message := fmt.Sprintf(tc.format, tc.args...)
		tc.wantedPattern = fmt.Sprintf("%s %s: %s\n$", pattern, tc.level, message)
		t.Run(tc.description, func(t *testing.T) {
			var buf bytes.Buffer
			logger := NewLogger(newBase(&buf))
			logger.SetLevel(level2.DebugLevel)
			logger.Debugf(tc.format, tc.args...)
			got := buf.String()
			assert.Regexp(t, tc.wantedPattern, got)
		})
	}
}

func TestLoggerWarnf(t *testing.T) {
	for _, tc := range formatTestCases {
		tc := tc
		tc.level = level2.WarnLevel
		message := fmt.Sprintf(tc.format, tc.args...)
		tc.wantedPattern = fmt.Sprintf("%s %s: %s\n$", pattern, tc.level, message)
		t.Run(tc.description, func(t *testing.T) {
			var buf bytes.Buffer
			logger := NewLogger(newBase(&buf))
			logger.Warnf(tc.format, tc.args...)
			got := buf.String()
			assert.Regexp(t, tc.wantedPattern, got)
		})
	}
}

func TestLoggerErrorf(t *testing.T) {
	for _, tc := range formatTestCases {
		tc := tc
		tc.level = level2.ErrorLevel
		message := fmt.Sprintf(tc.format, tc.args...)
		tc.wantedPattern = fmt.Sprintf("%s %s: %s\n$", pattern, tc.level, message)
		t.Run(tc.description, func(t *testing.T) {
			var buf bytes.Buffer
			logger := NewLogger(newBase(&buf))
			logger.Errorf(tc.format, tc.args...)
			got := buf.String()
			assert.Regexp(t, tc.wantedPattern, got)
		})
	}
}

func TestLoggerFatalf(t *testing.T) {
	originalOsExit := osExit
	defer func() { osExit = originalOsExit }()
	for _, tc := range formatTestCases {
		tc := tc
		tc.level = level2.FatalLevel
		message := fmt.Sprintf(tc.format, tc.args...)
		tc.wantedPattern = fmt.Sprintf("%s %s: %s\n$", pattern, tc.level, message)
		t.Run(tc.description, func(t *testing.T) {
			var buf bytes.Buffer
			exitCalled := false
			exitCode := 0
			osExit = func(code int) {
				exitCalled = true
				exitCode = code
			}
			logger := NewLogger(newBase(&buf))
			logger.Fatalf(tc.format, tc.args...)
			got := buf.String()
			assert.Regexp(t, tc.wantedPattern, got)
			assert.True(t, exitCalled)
			assert.Equal(t, 1, exitCode)
		})
	}
}

func TestLoggerWithLowerLevels(t *testing.T) {
	// Logger should not log messages at a level
	// lower than the specified level.
	levels := []level2.Level{level2.DebugLevel, level2.WarnLevel, level2.ErrorLevel, level2.FatalLevel}
	ops := []struct {
		op       string
		level    level2.Level
		logFunc  func(*Logger)
		expected bool
	}{
		{"Debug", level2.DebugLevel, func(l *Logger) { l.Debug("hello") }, false},
		{"Warn", level2.WarnLevel, func(l *Logger) { l.Warn("hello") }, false},
		{"Error", level2.ErrorLevel, func(l *Logger) { l.Error("hello") }, false},
		{"Fatal", level2.FatalLevel, func(l *Logger) { l.Fatal("hello") }, false},
	}

	for _, setLevel := range levels {
		for _, op := range ops {
			var buf bytes.Buffer
			logger := NewLogger(newBase(&buf))
			logger.SetLevel(setLevel)

			expectedOutput := op.level >= setLevel
			exitCalled := false
			exitCode := 0
			osExit = func(code int) {
				exitCalled = true
				exitCode = code
			}
			op.logFunc(logger)

			output := buf.String()
			if expectedOutput {
				assert.NotEmpty(t, output)
			} else {
				assert.Empty(t, output)
			}
			if op.op == "Fatal" {
				assert.True(t, exitCalled)
				assert.Equal(t, 1, exitCode)
			}
		}
	}
}
