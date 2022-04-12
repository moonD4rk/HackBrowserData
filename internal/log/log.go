package log

import (
	"fmt"
	"io"
	"log"
	"os"
)

type Level int

const (
	LevelDebug Level = iota
	LevelWarn
	LevelError
)

func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "debug"
	case LevelError:
		return "error"
	}
	return ""
}

var (
	formatLogger *Logger
	levelMap     = map[string]Level{
		"debug": LevelDebug,
		"error": LevelError,
	}
)

func InitLog(l string) {
	formatLogger = newLog(os.Stdout).setLevel(levelMap[l]).setFlags(log.Lshortfile)
}

type Logger struct {
	level Level
	l     *log.Logger
}

func newLog(w io.Writer) *Logger {
	return &Logger{
		l: log.New(w, "", 0),
	}
}

func (l *Logger) setFlags(flag int) *Logger {
	l.l.SetFlags(flag)
	return l
}

func (l *Logger) setLevel(level Level) *Logger {
	l.level = level
	return l
}

func (l *Logger) doLog(level Level, v ...interface{}) bool {
	if level < l.level {
		return false
	}
	l.l.Output(3, level.String()+" "+fmt.Sprintln(v...))
	return true
}

func (l *Logger) doLogf(level Level, format string, v ...interface{}) bool {
	if level < l.level {
		return false
	}
	l.l.Output(3, level.String()+" "+fmt.Sprintln(fmt.Sprintf(format, v...)))
	return true
}

func Debug(v ...interface{}) {
	formatLogger.doLog(LevelDebug, v...)
}

func Warn(v ...interface{}) {
	formatLogger.doLog(LevelWarn, v...)
}

func Error(v ...interface{}) {
	formatLogger.doLog(LevelError, v...)
}

func Errorf(format string, v ...interface{}) {
	formatLogger.doLogf(LevelError, format, v...)
}

func Warnf(format string, v ...interface{}) {
	formatLogger.doLogf(LevelWarn, format, v...)
}

func Debugf(format string, v ...interface{}) {
	formatLogger.doLogf(LevelDebug, format, v...)
}

// NewSugaredLogger(os.Stdout, DebugLevel).Configure(func(sl *SugaredLogger) {
// 	sl.SetName("stdLogger")
// 	sl.ReportCaller = true
// 	// auto enable console color
// 	sl.Formatter.(*TextFormatter).EnableColor = color.SupportColor()
//  sl.Formatter.SetCallerSkip(1)
// })
