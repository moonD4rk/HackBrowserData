package export

import (
	"fmt"
	"io"

	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/internal/record"
)

// Format defaults to JSON at its zero value, as -f does.
type Format int

const (
	JSON Format = iota
	CSV
	CookieEditor
)

func ParseFormat(value string) (Format, error) {
	switch value {
	case "json":
		return JSON, nil
	case "csv":
		return CSV, nil
	case "cookie-editor":
		return CookieEditor, nil
	default:
		return 0, fmt.Errorf("unsupported format: %s", value)
	}
}

func (f Format) ext() string {
	if f == CSV {
		return "csv"
	}
	return "json"
}

// encoder writes one category's file; it frames the document itself so that every Write can append to it.
type encoder interface {
	encode(row record.Record) error
	end() error
}

func newEncoder(writer io.Writer, format Format, dataCategory category.Category) encoder {
	switch {
	case format == CSV:
		return newCSVEncoder(writer)
	case format == CookieEditor && dataCategory == category.Cookie:
		return &jsonEncoder{writer: writer, value: cookieEditorValue}
	default:
		return &jsonEncoder{writer: writer, escapeHTML: true}
	}
}
