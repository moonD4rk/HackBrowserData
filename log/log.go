package log

import (
	"github.com/moond4rk/hackbrowserdata/log/level"
)

var (
	// defaultLogger is the default logger used by the package-level functions.
	defaultLogger = NewLogger(nil)
)

func SetVerbose() {
	defaultLogger.SetLevel(level.DebugLevel)
}

func Debug(args ...any) {
	defaultLogger.Debug(args...)
}

func Debugf(format string, args ...any) {
	defaultLogger.Debugf(format, args...)
}

func Warn(args ...any) {
	defaultLogger.Warn(args...)
}

func Warnf(format string, args ...any) {
	defaultLogger.Warnf(format, args...)
}

func Error(args ...any) {
	defaultLogger.Error(args...)
}

func Errorf(format string, args ...any) {
	defaultLogger.Errorf(format, args...)
}

func Fatal(args ...any) {
	defaultLogger.Fatal(args...)
}

func Fatalf(format string, args ...any) {
	defaultLogger.Fatalf(format, args...)
}
