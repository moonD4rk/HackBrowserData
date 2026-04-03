package output

import (
	"encoding/csv"
	"fmt"
	"io"
)

type csvFormatter struct{}

func (f *csvFormatter) ext() string { return "csv" }

func (f *csvFormatter) format(w io.Writer, data any) error {
	cw := csv.NewWriter(w)
	defer cw.Flush()

	switch rows := data.(type) {
	case []passwordRow:
		return writeCSV(cw, rows)
	case []cookieRow:
		return writeCSV(cw, rows)
	case []historyRow:
		return writeCSV(cw, rows)
	case []downloadRow:
		return writeCSV(cw, rows)
	case []bookmarkRow:
		return writeCSV(cw, rows)
	case []creditCardRow:
		return writeCSV(cw, rows)
	case []extensionRow:
		return writeCSV(cw, rows)
	case []storageRow:
		return writeCSV(cw, rows)
	default:
		return fmt.Errorf("csv: unsupported type %T", data)
	}
}

func writeCSV[T csvRecord](w *csv.Writer, rows []T) error {
	if len(rows) == 0 {
		return nil
	}
	if err := w.Write(rows[0].csvHeader()); err != nil {
		return err
	}
	for _, row := range rows {
		if err := w.Write(row.csvRow()); err != nil {
			return err
		}
	}
	return nil
}
