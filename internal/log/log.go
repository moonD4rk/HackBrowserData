package log

import "fmt"

var defaultLogger = NewLogger(nil)

func SetVerbose() {
	defaultLogger.SetLevel(DebugLevel)
}

func Debugf(format string, args ...any) {
	defaultLogger.logMsg(DebugLevel, fmt.Sprintf(format, args...))
}

func Infof(format string, args ...any) {
	defaultLogger.logMsg(InfoLevel, fmt.Sprintf(format, args...))
}

func Warnf(format string, args ...any) {
	defaultLogger.logMsg(WarnLevel, fmt.Sprintf(format, args...))
}
