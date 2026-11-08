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
	writer     io.Writer
	escapeHTML bool
	convert    func(record.Record) any
	written    int
}

func (e *jsonEncoder) encode(row record.Record) error {
	var value any = row
	if e.convert != nil {
		value = e.convert(row)
	}
	var buffer bytes.Buffer
	valueEncoder := json.NewEncoder(&buffer)
	valueEncoder.SetEscapeHTML(e.escapeHTML)
	valueEncoder.SetIndent(jsonIndent, jsonIndent)
	if err := valueEncoder.Encode(value); err != nil {
		return err
	}

	separator := ",\n" + jsonIndent
	if e.written == 0 {
		separator = "[\n" + jsonIndent
	}
	e.written++
	if _, err := io.WriteString(e.writer, separator); err != nil {
		return err
	}
	_, err := e.writer.Write(bytes.TrimSuffix(buffer.Bytes(), []byte("\n")))
	return err
}

func (e *jsonEncoder) end() error {
	closing := "\n]\n"
	if e.written == 0 {
		closing = "[]\n"
	}
	_, err := io.WriteString(e.writer, closing)
	return err
}
