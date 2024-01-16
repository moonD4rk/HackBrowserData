package logger

import (
	"context"
	"log/slog"
	"os"
)

var _ slog.Handler = (*LogHandler)(nil)

// LogHandler is a slog.Handler implementation that can be used to log to a file.
type LogHandler struct {
	handler slog.Handler
}

func NewHandler(logger *Logger) LogHandler {
	if logger == nil {
		logger = Default
	}

	level := logger.Level
	if logger.IsVerbose {
		level = slog.LevelDebug
	}

	output := logger.Output
	if output == nil {
		output = os.Stderr
	}

	handlerOptions := &slog.HandlerOptions{
		AddSource:   logger.AddSource,
		Level:       level,
		ReplaceAttr: logger.ReplaceAttr,
	}

	if logger.IsJSONHandler {
		return LogHandler{
			handler: slog.NewJSONHandler(output, handlerOptions),
		}
	}
	return LogHandler{
		handler: slog.NewTextHandler(output, handlerOptions),
	}
}

var _ slog.Handler = (*LogHandler)(nil)

func (t LogHandler) Handle(ctx context.Context, r slog.Record) error {
	return t.handler.Handle(ctx, r)
}

func (t LogHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return t.handler.Enabled(ctx, l)
}

func (t LogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return LogHandler{handler: t.handler.WithAttrs(attrs)}
}

func (t LogHandler) WithGroup(name string) slog.Handler {
	return LogHandler{handler: t.handler.WithGroup(name)}
}
