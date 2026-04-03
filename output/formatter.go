package output

import (
	"fmt"
	"io"
)

// formatter serializes rows to a writer. Unexported — only used by Writer.
type formatter interface {
	format(w io.Writer, rows []row) error
	ext() string
}

func newFormatter(name string) (formatter, error) {
	switch name {
	case "csv":
		return &csvFormatter{}, nil
	case "json":
		return &jsonFormatter{}, nil
	case "cookie-editor":
		return &cookieEditorFormatter{}, nil
	default:
		return nil, fmt.Errorf("unsupported format: %s", name)
	}
}
