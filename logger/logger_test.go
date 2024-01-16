package logger

import (
	"bytes"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigure(t *testing.T) {
	asserts := assert.New(t)
	buf := new(bytes.Buffer)

	opts := &Logger{
		AddSource:     true,
		IsVerbose:     false,
		IsJSONHandler: true,
		Output:        buf,
	}
	Configure(opts)

	slog.Warn("test message")

	output := buf.String()
	asserts.Contains(output, "test message", "Log output should contain the test message")
}

func TestSetVerbose(t *testing.T) {
	asserts := assert.New(t)
	buf := new(bytes.Buffer)

	logger := Default.clone()
	logger.SetVerbose()
	logger.SetOutput(buf)
	Configure(logger)

	slog.Debug("verbose test")

	output := buf.String()
	asserts.Contains(output, "verbose test", "Verbose mode should enable debug level logs")
}

func TestLogger_SetJSONHandler(t *testing.T) {
	asserts := assert.New(t)

	logger := Default.clone()
	logger.SetJSONHandler()
	asserts.True(logger.IsJSONHandler, "IsJSONHandler should be true")
}

func TestOptionsClone(t *testing.T) {
	asserts := assert.New(t)

	original := &Logger{
		AddSource:     true,
		IsVerbose:     true,
		IsJSONHandler: true,
		Output:        os.Stdout,
	}
	cloned := original.clone()

	asserts.Equal(original.AddSource, cloned.AddSource, "AddSource should be equal")
	asserts.Equal(original.IsVerbose, cloned.IsVerbose, "IsVerbose should be equal")
	asserts.Equal(original.IsJSONHandler, cloned.IsJSONHandler, "IsJSONHandler should be equal")
	asserts.Equal(original.Output, cloned.Output, "Output should be equal")
}
