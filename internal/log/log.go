package log

import "fmt"

// defaultLogger is the default logger used by the package-level functions.
var defaultLogger = NewLogger(nil)

// SetLevel sets the minimum level of the package-level logger; messages below v
// are suppressed. Consumers embedding HackBrowserData as a library use it to
// quiet output, e.g. SetLevel(FatalLevel) to silence everything short of a fatal
// error. It panics if v is less than DebugLevel or greater than FatalLevel.
func SetLevel(v Level) {
	defaultLogger.SetLevel(v)
}

// SetVerbose lowers the package-level logger to DebugLevel.
func SetVerbose() {
	SetLevel(DebugLevel)
}

func Debug(args ...any) {
	defaultLogger.logMsg(DebugLevel, fmt.Sprint(args...))
}

func Debugf(format string, args ...any) {
	defaultLogger.logMsg(DebugLevel, fmt.Sprintf(format, args...))
}

func Info(args ...any) {
	defaultLogger.logMsg(InfoLevel, fmt.Sprint(args...))
}

func Infof(format string, args ...any) {
	defaultLogger.logMsg(InfoLevel, fmt.Sprintf(format, args...))
}

func Warn(args ...any) {
	defaultLogger.logMsg(WarnLevel, fmt.Sprint(args...))
}

func Warnf(format string, args ...any) {
	defaultLogger.logMsg(WarnLevel, fmt.Sprintf(format, args...))
}

func Error(args ...any) {
	defaultLogger.logMsg(ErrorLevel, fmt.Sprint(args...))
}

func Errorf(format string, args ...any) {
	defaultLogger.logMsg(ErrorLevel, fmt.Sprintf(format, args...))
}

func Fatal(args ...any) {
	defaultLogger.logMsg(FatalLevel, fmt.Sprint(args...))
	osExit(1)
}

func Fatalf(format string, args ...any) {
	defaultLogger.logMsg(FatalLevel, fmt.Sprintf(format, args...))
	osExit(1)
}
