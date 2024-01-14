package logger

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLogHandler_Level(t *testing.T) {
	buf := new(bytes.Buffer)
	handler := NewLogHandler(buf, &LogHandlerOptions{
		IsVerbose: true,
	})
	assert.True(t, handler.Enabled(context.Background(), slog.LevelDebug), "Verbose handler should enable debug level")
}

func TestLogHandler_JSONFormat(t *testing.T) {
	buf := new(bytes.Buffer)
	handler := NewLogHandler(buf, &LogHandlerOptions{
		IsJSONHandler: true,
	})

	handler.Handle(context.Background(), slog.Record{
		Level: slog.LevelInfo,
	})

	output := buf.String()
	assert.JSONEq(t, `{"message":"test message"}`, output, "Output should be in JSON format")
}

func TestLogHandler_TextFormat(t *testing.T) {
	buf := new(bytes.Buffer)
	handler := NewLogHandler(buf, &LogHandlerOptions{
		IsJSONHandler: false,
	})

	handler.Handle(context.Background(), slog.Record{
		Level: slog.LevelInfo,
	})

	output := buf.String()
	assert.Contains(t, output, "test message", "Output should contain the log message in text format")
}

func TestLogHandler_ReplaceAttr(t *testing.T) {
	buf := new(bytes.Buffer)
	handler := NewLogHandler(buf, &LogHandlerOptions{
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == "test" {
				return slog.Attr{Key: a.Key, Value: slog.StringValue("replaced")}
			}
			return a
		},
	})

	// 触发一个带有特定属性的日志记录
	handler.Handle(context.Background(), slog.Record{
		Level: slog.LevelInfo,
	})

	// 检查属性是否被替换
	assert.Contains(t, buf.String(), "replaced", "Attribute value should be replaced")
}

func TestLogHandler_WithAttrs(t *testing.T) {
	buf := new(bytes.Buffer)
	baseHandler := NewLogHandler(buf, nil)
	extendedHandler := baseHandler.WithAttrs([]slog.Attr{slog.Attr{Key: "extra", Value: slog.StringValue("value")}})

	// 使用扩展的处理器记录日志
	extendedHandler.Handle(context.Background(), slog.Record{
		Level: slog.LevelInfo,
	})

	// 检查输出中是否包含额外的属性
	assert.Contains(t, buf.String(), "extra", "Output should contain the additional attribute")
	assert.Contains(t, buf.String(), "value", "Output should contain the value of the additional attribute")
}

func TestLogHandler_WithGroup(t *testing.T) {
	buf := new(bytes.Buffer)
	baseHandler := NewLogHandler(buf, nil)
	groupHandler := baseHandler.WithGroup("testGroup")

	groupHandler.Handle(context.Background(), slog.Record{
		Level: slog.LevelInfo,
	})

	assert.Contains(t, buf.String(), "testGroup", "Output should contain the group name")
}
