package output

import (
	"encoding/csv"
	"io"
)

type csvFormatter struct{}

func (f *csvFormatter) ext() string { return "csv" }

func (f *csvFormatter) format(w io.Writer, rows []row) error {
	if len(rows) == 0 {
		return nil
	}

	cw := csv.NewWriter(w)
	if err := cw.Write(rows[0].csvHeader()); err != nil {
		return err
	}
	for _, r := range rows {
		if err := cw.Write(r.csvRow()); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}
