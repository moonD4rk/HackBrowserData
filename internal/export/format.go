package export

import (
	"fmt"
	"io"

	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/internal/record"
)

// The zero Format is JSON, the -f default.
type Format int

const (
	JSON Format = iota
	CSV
	CookieEditor
)

func ParseFormat(s string) (Format, error) {
	switch s {
	case "json":
		return JSON, nil
	case "csv":
		return CSV, nil
	case "cookie-editor":
		return CookieEditor, nil
	default:
		return 0, fmt.Errorf("unsupported format: %s", s)
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
	encode(r record.Record) error
	end() error
}

func newEncoder(w io.Writer, f Format, c category.Category) encoder {
	switch {
	case f == CSV:
		return newCSVEncoder(w)
	case f == CookieEditor && c == category.Cookie:
		return &jsonEncoder{w: w, value: cookieEditorValue}
	default:
		return &jsonEncoder{w: w, escapeHTML: true}
	}
}
