package log

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
)

func NewLogger(base Base) *Logger {
	if base == nil {
		base = newBase(os.Stderr)
	}
	return &Logger{base: base, minLevel: InfoLevel}
}

type Logger struct {
	base Base

	minLevel Level
}

func (l *Logger) canLogAt(level Level) bool {
	return level >= Level(atomic.LoadInt32((*int32)(&l.minLevel)))
}

func (l *Logger) SetLevel(level Level) {
	if level < DebugLevel || level > WarnLevel {
		panic("log: invalid log level")
	}
	atomic.StoreInt32((*int32)(&l.minLevel), int32(level))
}

// baseCallerSkip skips the frames between runtime.Caller and the caller: base.Log, logMsg, and Debugf/Infof/Warnf.
const baseCallerSkip = 3

func (l *Logger) logMsg(level Level, message string) {
	if !l.canLogAt(level) {
		return
	}
	l.base.Log(baseCallerSkip, level, message)
}

func (l *Logger) Debugf(format string, args ...any) {
	l.logMsg(DebugLevel, fmt.Sprintf(format, args...))
}

func (l *Logger) Infof(format string, args ...any) {
	l.logMsg(InfoLevel, fmt.Sprintf(format, args...))
}

func (l *Logger) Warnf(format string, args ...any) {
	l.logMsg(WarnLevel, fmt.Sprintf(format, args...))
}

type Base interface {
	Log(callerSkip int, lvl Level, msg string)
}

type baseLogger struct {
	out   io.Writer
	mutex sync.Mutex
}

func newBase(out io.Writer) *baseLogger {
	return &baseLogger{out: out}
}

// continuation lines a message's later lines up under its first, past the "[DBG] " label.
const continuation = "      "

func (l *baseLogger) Log(callerSkip int, level Level, message string) {
	message = strings.TrimRight(message, "\n")
	if strings.Contains(message, "\n") {
		message = strings.ReplaceAll(message, "\n", "\n"+continuation)
	}

	label := l.formatLabel(level)
	var line string
	if level == DebugLevel {
		_, file, lineNumber, ok := runtime.Caller(callerSkip)
		if ok {
			file = filepath.Base(file)
		} else {
			file = "???"
			lineNumber = 0
		}
		line = fmt.Sprintf("%s %s:%d: %s\n", label, file, lineNumber, message)
	} else {
		line = fmt.Sprintf("%s %s\n", label, message)
	}

	l.mutex.Lock()
	_, _ = io.WriteString(l.out, line)
	l.mutex.Unlock()
}

func (l *baseLogger) formatLabel(level Level) string {
	return "[" + level.String() + "]"
}
