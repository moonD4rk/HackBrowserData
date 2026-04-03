package output

import (
	"encoding/csv"
	"fmt"
	"io"

	"github.com/moond4rk/hackbrowserdata/types"
)

// CSVFormatter writes category data as CSV with browser/profile prefix columns.
type CSVFormatter struct {
	wroteHeader map[string]bool // tracks which categories already have headers
}

func (f *CSVFormatter) Ext() string { return "csv" }

func (f *CSVFormatter) Format(w io.Writer, cd types.CategoryData, browser, profile string) error {
	if f.wroteHeader == nil {
		f.wroteHeader = make(map[string]bool)
	}

	cw := csv.NewWriter(w)
	defer cw.Flush()

	prefix := []string{browser, profile}
	needHeader := !f.wroteHeader[cd.Category.String()]

	if err := writeCSVRows(cw, cd.Data, prefix, needHeader); err != nil {
		return err
	}
	f.wroteHeader[cd.Category.String()] = true
	return cw.Error()
}

// writeCSVRows dispatches to the generic writeTypedCSV based on the concrete slice type.
func writeCSVRows(w *csv.Writer, data interface{}, prefix []string, writeHeader bool) error {
	switch rows := data.(type) {
	case []types.LoginEntry:
		return writeTypedCSV(w, rows, prefix, writeHeader)
	case []types.CookieEntry:
		return writeTypedCSV(w, rows, prefix, writeHeader)
	case []types.HistoryEntry:
		return writeTypedCSV(w, rows, prefix, writeHeader)
	case []types.DownloadEntry:
		return writeTypedCSV(w, rows, prefix, writeHeader)
	case []types.BookmarkEntry:
		return writeTypedCSV(w, rows, prefix, writeHeader)
	case []types.CreditCardEntry:
		return writeTypedCSV(w, rows, prefix, writeHeader)
	case []types.ExtensionEntry:
		return writeTypedCSV(w, rows, prefix, writeHeader)
	case []types.StorageEntry:
		return writeTypedCSV(w, rows, prefix, writeHeader)
	default:
		return fmt.Errorf("unsupported data type: %T", data)
	}
}

func writeTypedCSV[T types.CSVRecord](w *csv.Writer, rows []T, prefix []string, writeHeader bool) error {
	if len(rows) == 0 {
		return nil
	}
	prefixHeader := []string{"browser", "profile"}
	if writeHeader {
		header := make([]string, 0, len(prefixHeader)+len(rows[0].CSVHeader()))
		header = append(header, prefixHeader...)
		header = append(header, rows[0].CSVHeader()...)
		if err := w.Write(header); err != nil {
			return err
		}
	}
	for _, row := range rows {
		record := make([]string, 0, len(prefix)+len(row.CSVRow()))
		record = append(record, prefix...)
		record = append(record, row.CSVRow()...)
		if err := w.Write(record); err != nil {
			return err
		}
	}
	return nil
}
