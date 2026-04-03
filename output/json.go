package output

import (
	"encoding/json"
	"io"
)

type jsonFormatter struct{}

func (f *jsonFormatter) ext() string { return "json" }

func (f *jsonFormatter) format(w io.Writer, data any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(data)
}
