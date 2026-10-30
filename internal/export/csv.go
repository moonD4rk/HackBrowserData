package export

import (
	"encoding/csv"
	"fmt"
	"io"

	"github.com/moond4rk/hackbrowserdata/internal/record"
)

// utf8BOM is written at the start of CSV files for Excel compatibility.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

type csvEncoder struct {
	w      io.Writer
	rows   *csv.Writer
	header bool
}

func newCSVEncoder(w io.Writer) *csvEncoder {
	return &csvEncoder{w: w, rows: csv.NewWriter(w)}
}

func (e *csvEncoder) encode(r record.Record) error {
	if !e.header {
		if _, err := e.w.Write(utf8BOM); err != nil {
			return fmt.Errorf("write BOM: %w", err)
		}
		if err := e.rows.Write(structCSVHeader(r)); err != nil {
			return err
		}
		e.header = true
	}
	return e.rows.Write(structCSVRow(r))
}

func (e *csvEncoder) end() error {
	e.rows.Flush()
	return e.rows.Error()
}
