package log

import (
	"fmt"
	"io"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
)

type logger struct {
	out      io.Writer
	mutex    sync.Mutex
	minLevel Level
}

func newLogger(out io.Writer) *logger {
	return &logger{out: out, minLevel: InfoLevel}
}

func (l *logger) canLogAt(level Level) bool {
	return level >= Level(atomic.LoadInt32((*int32)(&l.minLevel)))
}

func (l *logger) SetLevel(level Level) {
	if level < DebugLevel || level > WarnLevel {
		panic("log: invalid log level")
	}
	atomic.StoreInt32((*int32)(&l.minLevel), int32(level))
}

// callerSkip skips the frames between runtime.Caller and the caller: logMsg, and Debugf/Infof/Warnf.
const callerSkip = 2

// continuation lines a message's later lines up under its first, past the "[DBG] " label.
const continuation = "      "

func (l *logger) logMsg(level Level, message string) {
	if !l.canLogAt(level) {
		return
	}
	message = strings.TrimRight(message, "\n")
	if strings.Contains(message, "\n") {
		message = strings.ReplaceAll(message, "\n", "\n"+continuation)
	}

	label := "[" + level.String() + "]"
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

func (l *logger) Debugf(format string, args ...any) {
	l.logMsg(DebugLevel, fmt.Sprintf(format, args...))
}

func (l *logger) Infof(format string, args ...any) {
	l.logMsg(InfoLevel, fmt.Sprintf(format, args...))
}

func (l *logger) Warnf(format string, args ...any) {
	l.logMsg(WarnLevel, fmt.Sprintf(format, args...))
}
