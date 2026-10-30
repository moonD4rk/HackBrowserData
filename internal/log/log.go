package log

import "fmt"

// defaultLogger is the default logger used by the package-level functions.
var defaultLogger = NewLogger(nil)

// SetVerbose lowers the package-level logger to DebugLevel.
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
