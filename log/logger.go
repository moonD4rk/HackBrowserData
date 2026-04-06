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

	"golang.org/x/term"
)

// NewLogger creates and returns a new instance of Logger.
// Default level is InfoLevel (Debug messages are suppressed unless SetVerbose is called).
func NewLogger(base Base) *Logger {
	if base == nil {
		base = newBase(os.Stderr)
	}
	return &Logger{base: base, minLevel: InfoLevel}
}

// Logger logs messages to io.Writer at various log levels.
type Logger struct {
	base Base

	// Minimum log level for this logger.
	// Messages with level lower than this won't be outputted.
	minLevel Level
}

// canLogAt reports whether logger can log at level v.
func (l *Logger) canLogAt(v Level) bool {
	return v >= Level(atomic.LoadInt32((*int32)(&l.minLevel)))
}

// SetLevel sets the logger level.
// It panics if v is less than DebugLevel or greater than FatalLevel.
func (l *Logger) SetLevel(v Level) {
	if v < DebugLevel || v > FatalLevel {
		panic("log: invalid log level")
	}
	atomic.StoreInt32((*int32)(&l.minLevel), int32(v))
}

// baseCallerSkip is the number of frames to skip in runtime.Caller to reach
// the actual call site. Both package-level functions (log.Xxx -> logMsg -> base.Log)
// and Logger methods (Logger.Xxx -> logMsg -> base.Log) add exactly one frame
// above logMsg, so the skip is the same: base.Log(0) -> logMsg(1) -> caller_wrapper(2) -> caller(3).
const baseCallerSkip = 3

// logMsg is the internal method all public methods delegate to.
func (l *Logger) logMsg(lvl Level, msg string) {
	if !l.canLogAt(lvl) {
		return
	}
	l.base.Log(baseCallerSkip, lvl, msg)
}

func (l *Logger) Debug(args ...any) {
	l.logMsg(DebugLevel, fmt.Sprint(args...))
}

func (l *Logger) Debugf(format string, args ...any) {
	l.logMsg(DebugLevel, fmt.Sprintf(format, args...))
}

func (l *Logger) Info(args ...any) {
	l.logMsg(InfoLevel, fmt.Sprint(args...))
}

func (l *Logger) Infof(format string, args ...any) {
	l.logMsg(InfoLevel, fmt.Sprintf(format, args...))
}

func (l *Logger) Warn(args ...any) {
	l.logMsg(WarnLevel, fmt.Sprint(args...))
}

func (l *Logger) Warnf(format string, args ...any) {
	l.logMsg(WarnLevel, fmt.Sprintf(format, args...))
}

func (l *Logger) Error(args ...any) {
	l.logMsg(ErrorLevel, fmt.Sprint(args...))
}

func (l *Logger) Errorf(format string, args ...any) {
	l.logMsg(ErrorLevel, fmt.Sprintf(format, args...))
}

func (l *Logger) Fatal(args ...any) {
	l.logMsg(FatalLevel, fmt.Sprint(args...))
	osExit(1)
}

func (l *Logger) Fatalf(format string, args ...any) {
	l.logMsg(FatalLevel, fmt.Sprintf(format, args...))
	osExit(1)
}

// Base is the interface that underlies the Logger. It receives the caller
// skip count, log level, and formatted message.
type Base interface {
	Log(callerSkip int, lvl Level, msg string)
}

// ANSI color escape sequences.
const (
	colorReset   = "\033[0m"
	colorRed     = "\033[31m"
	colorYellow  = "\033[33m"
	colorBlue    = "\033[34m"
	colorMagenta = "\033[35m"
	colorBoldRed = "\033[1;31m"
)

// levelColor maps each log level to its ANSI color.
var levelColor = map[Level]string{
	DebugLevel: colorMagenta,
	InfoLevel:  colorBlue,
	WarnLevel:  colorYellow,
	ErrorLevel: colorRed,
	FatalLevel: colorBoldRed,
}

// baseLogger writes formatted log messages to an io.Writer.
// Output format (example with colors disabled):
//
//	[DBG] file.go:42: message
//	[INF] message
//	[WRN] message
//	[ERR] message
//	[FTL] message
type baseLogger struct {
	out   io.Writer
	mu    sync.Mutex
	color bool
}

func newBase(out io.Writer) *baseLogger {
	return &baseLogger{out: out, color: isColorTerminal(out)}
}

// isColorTerminal reports whether out is a terminal that supports ANSI colors.
func isColorTerminal(out io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	f, ok := out.(*os.File)
	if !ok {
		return false
	}
	if !term.IsTerminal(int(f.Fd())) {
		return false
	}
	// On Windows, only enable colors if running inside a modern terminal
	// (Windows Terminal sets WT_SESSION, mintty/ConEmu/Cygwin set TERM).
	// Plain cmd.exe on Windows 7/8 does not support ANSI escape codes.
	if runtime.GOOS == "windows" {
		return os.Getenv("WT_SESSION") != "" || os.Getenv("TERM") != ""
	}
	return true
}

var osExit = os.Exit

// continuation is the indent for multi-line messages.
// Width matches "[DBG] " (6 chars).
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

// formatLabel returns the bracketed level label, optionally with ANSI color.
func (l *baseLogger) formatLabel(lvl Level) string {
	tag := lvl.String() // e.g. "DBG", "INF"
	if l.color {
		return levelColor[lvl] + "[" + tag + "]" + colorReset
	}
	return "[" + tag + "]"
}
