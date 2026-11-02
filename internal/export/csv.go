package export

import (
	"encoding/csv"
	"fmt"
	"io"

	"github.com/moond4rk/hackbrowserdata/internal/record"
)

// utf8BOM starts every CSV file, because Excel reads one as UTF-8 only when it starts with a BOM.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

type csvEncoder struct {
	writer io.Writer
	rows   *csv.Writer
	header bool
}

func newCSVEncoder(writer io.Writer) *csvEncoder {
	return &csvEncoder{writer: writer, rows: csv.NewWriter(writer)}
}

func (e *csvEncoder) encode(row record.Record) error {
	if !e.header {
		if _, err := e.writer.Write(utf8BOM); err != nil {
			return fmt.Errorf("write BOM: %w", err)
		}
		if err := e.rows.Write(structCSVHeader(row)); err != nil {
			return err
		}
		e.header = true
	}
	return e.rows.Write(structCSVRow(row))
}

func (e *csvEncoder) end() error {
	e.rows.Flush()
	return e.rows.Error()
}
