package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

var defaultHandleOptions = &LogHandlerOptions{
	AddSource:     true,
	IsVerbose:     true,
	IsJSONHandler: true,
	ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
		if a.Key == slog.TimeKey {
			return slog.Attr{}
		}
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
	},
}

var defaultHandler = NewLogHandler(os.Stderr, defaultHandleOptions)

var STDLogger = slog.New(defaultHandler)

var _ slog.Handler = (*LogHandler)(nil)

type LogHandler struct {
	handler slog.Handler
}

type LogHandlerOptions struct {
	AddSource     bool
	IsVerbose     bool
	IsJSONHandler bool
	ReplaceAttr   func(groups []string, a slog.Attr) slog.Attr
}

func NewLogHandler(output io.Writer, opts *LogHandlerOptions) LogHandler {
	if opts == nil {
		opts = defaultHandleOptions
	}
	level := slog.LevelWarn
	if opts.IsVerbose {
		level = slog.LevelDebug
	}

	handlerOptions := &slog.HandlerOptions{
		AddSource:   opts.AddSource,
		Level:       level,
		ReplaceAttr: opts.ReplaceAttr,
	}
	if opts.IsJSONHandler {
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
