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

func (l *Logger) canLogAt(v Level) bool {
	return v >= Level(atomic.LoadInt32((*int32)(&l.minLevel)))
}

func (l *Logger) SetLevel(v Level) {
	if v < DebugLevel || v > WarnLevel {
		panic("log: invalid log level")
	}
	atomic.StoreInt32((*int32)(&l.minLevel), int32(v))
}

// Frames between runtime.Caller and the caller: base.Log, logMsg, and the exported Debugf/Infof/Warnf.
const baseCallerSkip = 3

func (l *Logger) logMsg(lvl Level, msg string) {
	if !l.canLogAt(lvl) {
		return
	}
	l.base.Log(baseCallerSkip, lvl, msg)
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
	out io.Writer
	mu  sync.Mutex
}

func newBase(out io.Writer) *baseLogger {
	return &baseLogger{out: out}
}

// continuation lines a message's later lines up under its first, past the "[DBG] " label.
const continuation = "      "

func (l *baseLogger) Log(callerSkip int, lvl Level, msg string) {
	msg = strings.TrimRight(msg, "\n")
	if strings.Contains(msg, "\n") {
		msg = strings.ReplaceAll(msg, "\n", "\n"+continuation)
	}

	label := l.formatLabel(lvl)
	var line string
	if lvl == DebugLevel {
		_, file, num, ok := runtime.Caller(callerSkip)
		if ok {
			file = filepath.Base(file)
		} else {
			file = "???"
			num = 0
		}
		line = fmt.Sprintf("%s %s:%d: %s\n", label, file, num, msg)
	} else {
		line = fmt.Sprintf("%s %s\n", label, msg)
	}

	l.mu.Lock()
	_, _ = io.WriteString(l.out, line)
	l.mu.Unlock()
}

func (l *baseLogger) formatLabel(lvl Level) string {
	return "[" + lvl.String() + "]"
}
