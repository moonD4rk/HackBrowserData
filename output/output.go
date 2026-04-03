// Package output writes extracted browser data to files.
//
// Usage:
//
//	w, _ := output.NewWriter(dir, "csv")
//	w.Add(browserName, profileName, data)
//	w.Write()
//
// Supported formats: csv, json, cookie-editor.
package output

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/moond4rk/hackbrowserdata/log"
	"github.com/moond4rk/hackbrowserdata/types"
)

// utf8BOM is written at the start of CSV files for Excel compatibility.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// Writer collects browser data and writes it to files.
// It is the only exported type in this package.
type Writer struct {
	dir       string
	formatter formatter
	results   []result
}

type result struct {
	browser string
	profile string
	data    *types.BrowserData
}

// NewWriter creates a Writer that writes to dir in the given format.
func NewWriter(dir, format string) (*Writer, error) {
	f, err := newFormatter(format)
	if err != nil {
		return nil, err
	}
	return &Writer{dir: dir, formatter: f}, nil
}

// Add accumulates one browser profile's data for later writing.
func (o *Writer) Add(browser, profile string, data *types.BrowserData) {
	if data == nil {
		return
	}
	o.results = append(o.results, result{browser, profile, data})
}

// Write aggregates all accumulated data by category and writes each
// non-empty category to its own file (e.g. password.csv, cookie.json).
func (o *Writer) Write() error {
	if err := os.MkdirAll(o.dir, 0o750); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}
	for _, cs := range o.aggregate() {
		if err := o.writeFile(cs.name, cs.rows); err != nil {
			return err
		}
	}
	return nil
}

// categoryRows holds one category's aggregated rows for writing.
type categoryRows struct {
	name string
	rows []row
}

// extractor pulls rows from a single result for one category.
type extractor func(r result) []row

// makeExtractor creates a type-safe extractor using generics.
func makeExtractor[T any](entries func(*types.BrowserData) []T) extractor {
	return func(r result) []row {
		items := entries(r.data)
		rows := make([]row, 0, len(items))
		for _, e := range items {
			rows = append(rows, row{Browser: r.browser, Profile: r.profile, entry: e})
		}
		return rows
	}
}

// categories maps each data category to its extractor.
// Adding a new category requires only one line here.
var categories = []struct {
	name    string
	extract extractor
}{
	{"password", makeExtractor(func(d *types.BrowserData) []types.LoginEntry { return d.Passwords })},
	{"cookie", makeExtractor(func(d *types.BrowserData) []types.CookieEntry { return d.Cookies })},
	{"history", makeExtractor(func(d *types.BrowserData) []types.HistoryEntry { return d.Histories })},
	{"download", makeExtractor(func(d *types.BrowserData) []types.DownloadEntry { return d.Downloads })},
	{"bookmark", makeExtractor(func(d *types.BrowserData) []types.BookmarkEntry { return d.Bookmarks })},
	{"creditcard", makeExtractor(func(d *types.BrowserData) []types.CreditCardEntry { return d.CreditCards })},
	{"extension", makeExtractor(func(d *types.BrowserData) []types.ExtensionEntry { return d.Extensions })},
	{"localstorage", makeExtractor(func(d *types.BrowserData) []types.StorageEntry { return d.LocalStorage })},
	{"sessionstorage", makeExtractor(func(d *types.BrowserData) []types.StorageEntry { return d.SessionStorage })},
}

// aggregate merges all results into row slices grouped by category,
// returning only non-empty categories.
func (o *Writer) aggregate() []categoryRows {
	var s []categoryRows
	for _, cat := range categories {
		var rows []row
		for _, r := range o.results {
			rows = append(rows, cat.extract(r)...)
		}
		if len(rows) > 0 {
			s = append(s, categoryRows{cat.name, rows})
		}
	}
	return s
}

func (o *Writer) writeFile(category string, rows []row) (err error) {
	// Format to buffer first — if formatter produces no output (e.g.
	// cookie-editor skipping non-cookie data), don't create the file.
	var buf bytes.Buffer
	if err := o.formatter.format(&buf, rows); err != nil {
		return fmt.Errorf("format %s: %w", category, err)
	}
	if buf.Len() == 0 {
		return nil
	}

	filename := fmt.Sprintf("%s.%s", category, o.formatter.ext())
	path := filepath.Join(o.dir, filename)

	f, err := os.OpenFile(filepath.Clean(path), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("create %s: %w", filename, err)
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("close %s: %w", filename, cerr)
		}
	}()

	if strings.HasSuffix(path, ".csv") {
		if _, err := f.Write(utf8BOM); err != nil {
			return fmt.Errorf("write BOM: %w", err)
		}
	}

	if _, err := f.Write(buf.Bytes()); err != nil {
		return fmt.Errorf("write %s: %w", filename, err)
	}
	log.Warnf("export: %s", path)
	return nil
}
