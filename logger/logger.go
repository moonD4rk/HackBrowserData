package logger

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

// Default is the default *Logger for the default handler.
var Default = &Logger{
	AddSource:     true,
	IsVerbose:     false,
	IsJSONHandler: false,
	Level:         slog.LevelWarn,
	ReplaceAttr:   defaultReplaceAttrFunc,
	Output:        os.Stderr,
}

func init() {
	Configure(Default)
}

// Configure configures the logger by the given options.
func Configure(opts *Logger) {
	customHandler := NewHandler(opts)
	slog.SetDefault(slog.New(customHandler))
}

type Logger struct {
	// AddSource indicates whether to add source code location to the log.
	AddSource bool

	// IsVerbose indicates whether to enable verbose mode. If true, debug level will be enabled.
	// If false, only warn and error level will be enabled.
	IsVerbose bool

	// Level indicates the log level of the handler. If IsVerbose is true, Level will be slog.LevelDebug.
	Level slog.Level

	// IsJSONHandler indicates whether to use JSON format for log output.
	IsJSONHandler bool

	// ReplaceAttr is a function that can be used to replace the value of an attribute.
	ReplaceAttr func(groups []string, a slog.Attr) slog.Attr

	// Output is the writer to write the log to. If nil, os.Stderr will be used.
	Output io.Writer
}

func (o *Logger) clone() *Logger {
	return &Logger{
		AddSource:     o.AddSource,
		IsVerbose:     o.IsVerbose,
		IsJSONHandler: o.IsJSONHandler,
		ReplaceAttr:   o.ReplaceAttr,
		Output:        o.Output,
	}
}

// SetMaxLevel sets the max logging level for logger, default is slog.LevelWarn.
// if IsVerbose is true, level will be slog.LevelDebug.
func (o *Logger) SetMaxLevel(level slog.Level) {
	o.Level = level
}

func (o *Logger) SetJSONHandler() {
	o.IsJSONHandler = true
}

func (o *Logger) SetTextHandler() {
	o.IsJSONHandler = false
}

func (o *Logger) SetOutput(output io.Writer) {
	o.Output = output
}

func (o *Logger) SetVerbose() {
	o.IsVerbose = true
	o.Level = slog.LevelDebug
}

func (o *Logger) SetReplaceAttrFunc(replaceAttrFunc func(groups []string, a slog.Attr) slog.Attr) {
	o.ReplaceAttr = replaceAttrFunc
}

// defaultReplaceAttrFunc is a function that can be used to replace the value of an attribute.
// remove time key and source prefix
var defaultReplaceAttrFunc = func(groups []string, a slog.Attr) slog.Attr {
	// Remove time attributes from the log.
	if a.Key == slog.TimeKey {
		return slog.Attr{}
	}
	// Remove source filepath prefix attributes from the log.
	if a.Key == slog.SourceKey {
		source, ok := a.Value.Any().(*slog.Source)
		if !ok {
			return slog.Attr{}
		}
		if source != nil {
			source.File = filepath.Base(source.File)
		}
	}
	return a
}
