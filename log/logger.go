package log

import (
	"fmt"
	"io"
	stdlog "log"
	"os"
	"runtime"
	"strings"
	"sync/atomic"

	"github.com/moond4rk/hackbrowserdata/log/level"
)

// NewLogger creates and returns a new instance of Logger.
// Log level is set to DebugLevel by default.
func NewLogger(base Base) *Logger {
	if base == nil {
		base = newBase(os.Stderr)
	}
	return &Logger{base: base, minLevel: level.WarnLevel}
}

// Logger logs message to io.Writer at various log levels.
type Logger struct {
	base Base

	// Minimum log level for this logger.
	// Message with level lower than this level won't be outputted.
	minLevel level.Level
}

// canLogAt reports whether logger can log at level v.
func (l *Logger) canLogAt(v level.Level) bool {
	return v >= level.Level(atomic.LoadInt32((*int32)(&l.minLevel)))
}

// SetLevel sets the logger level.
// It panics if v is less than DebugLevel or greater than FatalLevel.
func (l *Logger) SetLevel(v level.Level) {
	if v < level.DebugLevel || v > level.FatalLevel {
		panic("log: invalid log level")
	}
	atomic.StoreInt32((*int32)(&l.minLevel), int32(v))
}

func (l *Logger) Debug(args ...any) {
	if !l.canLogAt(level.DebugLevel) {
		return
	}
	l.base.Debug(args...)
}

func (l *Logger) Warn(args ...any) {
	if !l.canLogAt(level.WarnLevel) {
		return
	}
	l.base.Warn(args...)
}

func (l *Logger) Error(args ...any) {
	if !l.canLogAt(level.ErrorLevel) {
		return
	}
	l.base.Error(args...)
}

func (l *Logger) Fatal(args ...any) {
	if !l.canLogAt(level.FatalLevel) {
		return
	}
	l.base.Fatal(args...)
}

func (l *Logger) Debugf(format string, args ...any) {
	if !l.canLogAt(level.DebugLevel) {
		return
	}
	l.base.Debug(fmt.Sprintf(format, args...))
}

func (l *Logger) Warnf(format string, args ...any) {
	if !l.canLogAt(level.WarnLevel) {
		return
	}
	l.base.Warn(fmt.Sprintf(format, args...))
}

func (l *Logger) Errorf(format string, args ...any) {
	if !l.canLogAt(level.ErrorLevel) {
		return
	}
	l.base.Error(fmt.Sprintf(format, args...))
}

func (l *Logger) Fatalf(format string, args ...any) {
	if !l.canLogAt(level.FatalLevel) {
		return
	}
	l.base.Fatal(fmt.Sprintf(format, args...))
}

type Base interface {
	Debug(args ...any)
	Warn(args ...any)
	Error(args ...any)
	Fatal(args ...any)
}

// baseLogger is a wrapper object around log.Logger from the standard library.
// It supports logging at various log levels.
type baseLogger struct {
	*stdlog.Logger
	callDepth int
}

func newBase(out io.Writer) *baseLogger {
	prefix := "[hack-browser-data] "
	base := &baseLogger{
		Logger: stdlog.New(out, prefix, stdlog.Lshortfile),
	}
	base.callDepth = base.calculateCallDepth()
	return base
}

// calculateCallDepth returns the call depth for the logger.
func (l *baseLogger) calculateCallDepth() int {
	return l.getCallDepth()
}

func (l *baseLogger) prefixPrint(prefix string, args ...any) {
	args = append([]any{prefix}, args...)
	if err := l.Output(l.callDepth, fmt.Sprint(args...)); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "log output error: %v\n", err)
	}
}

func (l *baseLogger) getCallDepth() int {
	var defaultCallDepth = 2
	pcs := make([]uintptr, 10)
	n := runtime.Callers(defaultCallDepth, pcs)
	frames := runtime.CallersFrames(pcs[:n])
	for i := 0; i < n; i++ {
		frame, more := frames.Next()
		if !l.isLoggerPackage(frame.Function) {
			return i + 1
		}
		if !more {
			break
		}
	}
	return defaultCallDepth
}

func (l *baseLogger) isLoggerPackage(funcName string) bool {
	const loggerFuncName = "hackbrowserdata/log"
	return strings.Contains(funcName, loggerFuncName)
}

// Debug logs a message at Debug level.
func (l *baseLogger) Debug(args ...any) {
	l.prefixPrint("DEBUG: ", args...)
}

// Warn logs a message at Warning level.
func (l *baseLogger) Warn(args ...any) {
	l.prefixPrint("WARN: ", args...)
}

// Error logs a message at Error level.
func (l *baseLogger) Error(args ...any) {
	l.prefixPrint("ERROR: ", args...)
}

var osExit = os.Exit

// Fatal logs a message at Fatal level
// and process will exit with status set to 1.
func (l *baseLogger) Fatal(args ...any) {
	l.prefixPrint("FATAL: ", args...)
	osExit(1)
}
