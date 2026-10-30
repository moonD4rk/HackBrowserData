package export

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/moond4rk/hackbrowserdata/internal/record"
)

const jsonIndent = "  "

// jsonEncoder writes the bytes a single indented Encode of the whole slice would produce, one element at a time.
type jsonEncoder struct {
	w          io.Writer
	escapeHTML bool
	value      func(record.Record) any
	n          int
}

func (e *jsonEncoder) encode(r record.Record) error {
	var v any = r
	if e.value != nil {
		v = e.value(r)
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(e.escapeHTML)
	enc.SetIndent(jsonIndent, jsonIndent)
	if err := enc.Encode(v); err != nil {
		return err
	}

	sep := ",\n" + jsonIndent
	if e.n == 0 {
		sep = "[\n" + jsonIndent
	}
	e.n++
	if _, err := io.WriteString(e.w, sep); err != nil {
		return err
	}
	_, err := e.w.Write(bytes.TrimSuffix(buf.Bytes(), []byte("\n")))
	return err
}

func (e *jsonEncoder) end() error {
	closing := "\n]\n"
	if e.n == 0 {
		closing = "[]\n"
	}
	_, err := io.WriteString(e.w, closing)
	return err
}
