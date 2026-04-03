package output

import (
	"fmt"
	"io"

	"github.com/moond4rk/hackbrowserdata/types"
)

// Formatter serializes category data to a writer.
// Each format (CSV, JSON, CookieEditor) implements this interface.
// Formatters only handle serialization — file management is done by Write().
type Formatter interface {
	// Format writes the category data to w, prefixed with browser/profile context.
	Format(w io.Writer, cd types.CategoryData, browser, profile string) error
	// Ext returns the file extension for this format (e.g. "csv", "json").
	Ext() string
}

// NewFormatter creates a Formatter for the given format name.
func NewFormatter(name string) (Formatter, error) {
	switch name {
	case "csv":
		return &CSVFormatter{}, nil
	case "json":
		return &JSONFormatter{}, nil
	case "cookie-editor":
		return &CookieEditorFormatter{}, nil
	default:
		return nil, fmt.Errorf("unsupported format: %s", name)
	}
}
