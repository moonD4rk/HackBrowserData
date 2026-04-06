package log

import "fmt"

// defaultLogger is the default logger used by the package-level functions.
var defaultLogger = NewLogger(nil)

func SetVerbose() {
	defaultLogger.SetLevel(DebugLevel)
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
