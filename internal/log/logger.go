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
	minLevel logLevel
}

func newLogger(out io.Writer) *logger {
	return &logger{out: out, minLevel: infoLevel}
}

func (l *logger) enabled(level logLevel) bool {
	return level >= logLevel(atomic.LoadInt32((*int32)(&l.minLevel)))
}

func (l *logger) setLevel(level logLevel) {
	if level < debugLevel || level > warnLevel {
		panic("log: invalid log level")
	}
	atomic.StoreInt32((*int32)(&l.minLevel), int32(level))
}

// callerSkip skips the frames between runtime.Caller and the caller: write, and Debugf/Infof/Warnf.
const callerSkip = 2

// continuation lines a message's later lines up under its first, past the "[DBG] " label.
const continuation = "      "

func (l *logger) write(level logLevel, message string) {
	if !l.enabled(level) {
		return
	}
	message = strings.TrimRight(message, "\n")
	if strings.Contains(message, "\n") {
		message = strings.ReplaceAll(message, "\n", "\n"+continuation)
	}

	label := "[" + level.String() + "]"
	var line string
	if level == debugLevel {
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
	l.write(debugLevel, fmt.Sprintf(format, args...))
}

func (l *logger) Infof(format string, args ...any) {
	l.write(infoLevel, fmt.Sprintf(format, args...))
}

func (l *logger) Warnf(format string, args ...any) {
	l.write(warnLevel, fmt.Sprintf(format, args...))
}
