package log

import (
	"fmt"
	"os"
)

var defaultLogger = newLogger(os.Stderr)

func SetVerbose() {
	defaultLogger.setLevel(debugLevel)
}

func Debugf(format string, args ...any) {
	defaultLogger.write(debugLevel, fmt.Sprintf(format, args...))
}

func Infof(format string, args ...any) {
	defaultLogger.write(infoLevel, fmt.Sprintf(format, args...))
}

func Warnf(format string, args ...any) {
	defaultLogger.write(warnLevel, fmt.Sprintf(format, args...))
}
