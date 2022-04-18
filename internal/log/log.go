package log

import (
	"os"

	"github.com/gookit/color"
	"github.com/gookit/slog"
)

var std = &slog.SugaredLogger{}

func Init(l string) {
	if l == "debug" {
		std = newStdLogger(slog.DebugLevel)
	} else {
		std = newStdLogger(slog.NoticeLevel)
	}
}

const template = "[{{level}}] [{{caller}}] {{message}} {{data}} {{extra}}\n"

// NewStdLogger instance
func newStdLogger(level slog.Level) *slog.SugaredLogger {
	return slog.NewSugaredLogger(os.Stdout, level).Configure(func(sl *slog.SugaredLogger) {
		sl.SetName("stdLogger")
		sl.ReportCaller = true
		sl.CallerSkip = 3
		// auto enable console color
		sl.Formatter.(*slog.TextFormatter).EnableColor = color.SupportColor()
		sl.Formatter.(*slog.TextFormatter).SetTemplate(template)
	})
}

// Trace logs a message at level Trace
func Trace(args ...interface{}) {
	std.Log(slog.TraceLevel, args...)
}

// Tracef logs a message at level Trace
func Tracef(format string, args ...interface{}) {
	std.Logf(slog.TraceLevel, format, args...)
}

// Info logs a message at level Info
func Info(args ...interface{}) {
	std.Log(slog.InfoLevel, args...)
}

// Infof logs a message at level Info
func Infof(format string, args ...interface{}) {
	std.Logf(slog.InfoLevel, format, args...)
}

// Notice logs a message at level Notice
func Notice(args ...interface{}) {
	std.Log(slog.NoticeLevel, args...)
}

// Noticef logs a message at level Notice
func Noticef(format string, args ...interface{}) {
	std.Logf(slog.NoticeLevel, format, args...)
}

// Warn logs a message at level Warn
func Warn(args ...interface{}) {
	std.Log(slog.WarnLevel, args...)
}

// Warnf logs a message at level Warn
func Warnf(format string, args ...interface{}) {
	std.Logf(slog.WarnLevel, format, args...)
}

// Error logs a message at level Error
func Error(args ...interface{}) {
	std.Log(slog.ErrorLevel, args...)
}

// ErrorT logs a error type at level Error
func ErrorT(err error) {
	if err != nil {
		std.Log(slog.ErrorLevel, err)
	}
}

// Errorf logs a message at level Error
func Errorf(format string, args ...interface{}) {
	std.Logf(slog.ErrorLevel, format, args...)
}

// Debug logs a message at level Debug
func Debug(args ...interface{}) {
	std.Log(slog.DebugLevel, args...)
}

// Debugf logs a message at level Debug
func Debugf(format string, args ...interface{}) {
	std.Logf(slog.DebugLevel, format, args...)
}

// Fatal logs a message at level Fatal
func Fatal(args ...interface{}) {
	std.Log(slog.FatalLevel, args...)
}

// Fatalf logs a message at level Fatal
func Fatalf(format string, args ...interface{}) {
	std.Logf(slog.FatalLevel, format, args...)
}

// Panic logs a message at level Panic
func Panic(args ...interface{}) {
	std.Log(slog.PanicLevel, args...)
}

// Panicf logs a message at level Panic
func Panicf(format string, args ...interface{}) {
	std.Logf(slog.PanicLevel, format, args...)
}
