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
	o.results = append(o.results, result{browser, profile, data})
}

// Write aggregates all accumulated data by category and writes each
// non-empty category to its own file (e.g. password.csv, cookie.json).
func (o *Writer) Write() error {
	agg := o.aggregate()

	if err := os.MkdirAll(o.dir, 0o750); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	if len(agg.passwords) > 0 {
		o.writeFile("password", agg.passwords)
	}
	if len(agg.cookies) > 0 {
		o.writeFile("cookie", agg.cookies)
	}
	if len(agg.histories) > 0 {
		o.writeFile("history", agg.histories)
	}
	if len(agg.downloads) > 0 {
		o.writeFile("download", agg.downloads)
	}
	if len(agg.bookmarks) > 0 {
		o.writeFile("bookmark", agg.bookmarks)
	}
	if len(agg.creditCards) > 0 {
		o.writeFile("creditcard", agg.creditCards)
	}
	if len(agg.extensions) > 0 {
		o.writeFile("extension", agg.extensions)
	}
	if len(agg.localStorage) > 0 {
		o.writeFile("localstorage", agg.localStorage)
	}
	if len(agg.sessionStorage) > 0 {
		o.writeFile("sessionstorage", agg.sessionStorage)
	}
	return nil
}

// aggregate merges all results into flat row slices grouped by category.
func (o *Writer) aggregate() aggregated {
	var agg aggregated
	for _, r := range o.results {
		for _, p := range r.data.Passwords {
			agg.passwords = append(agg.passwords, passwordRow{r.browser, r.profile, p})
		}
		for _, c := range r.data.Cookies {
			agg.cookies = append(agg.cookies, cookieRow{r.browser, r.profile, c})
		}
		for _, h := range r.data.Histories {
			agg.histories = append(agg.histories, historyRow{r.browser, r.profile, h})
		}
		for _, d := range r.data.Downloads {
			agg.downloads = append(agg.downloads, downloadRow{r.browser, r.profile, d})
		}
		for _, b := range r.data.Bookmarks {
			agg.bookmarks = append(agg.bookmarks, bookmarkRow{r.browser, r.profile, b})
		}
		for _, c := range r.data.CreditCards {
			agg.creditCards = append(agg.creditCards, creditCardRow{r.browser, r.profile, c})
		}
		for _, e := range r.data.Extensions {
			agg.extensions = append(agg.extensions, extensionRow{r.browser, r.profile, e})
		}
		for _, s := range r.data.LocalStorage {
			agg.localStorage = append(agg.localStorage, storageRow{r.browser, r.profile, s})
		}
		for _, s := range r.data.SessionStorage {
			agg.sessionStorage = append(agg.sessionStorage, storageRow{r.browser, r.profile, s})
		}
	}
	return agg
}

type aggregated struct {
	passwords      []passwordRow
	cookies        []cookieRow
	histories      []historyRow
	downloads      []downloadRow
	bookmarks      []bookmarkRow
	creditCards    []creditCardRow
	extensions     []extensionRow
	localStorage   []storageRow
	sessionStorage []storageRow
}

func (o *Writer) writeFile(category string, data any) {
	// Format to buffer first — if formatter produces no output (e.g.
	// cookie-editor skipping non-cookie data), don't create the file.
	var buf bytes.Buffer
	if err := o.formatter.format(&buf, data); err != nil {
		log.Debugf("format %s: %v", category, err)
		return
	}
	if buf.Len() == 0 {
		return
	}

	filename := fmt.Sprintf("%s.%s", category, o.formatter.ext())
	path := filepath.Join(o.dir, filename)

	f, err := os.OpenFile(filepath.Clean(path), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		log.Debugf("create %s: %v", filename, err)
		return
	}
	defer f.Close()

	// Write UTF-8 BOM for CSV files
	if strings.HasSuffix(path, ".csv") {
		if _, err := f.Write(utf8BOM); err != nil {
			log.Debugf("write BOM: %v", err)
			return
		}
	}

	if _, err := f.Write(buf.Bytes()); err != nil {
		log.Debugf("write %s: %v", filename, err)
		return
	}
	log.Warnf("export: %s", path)
}
