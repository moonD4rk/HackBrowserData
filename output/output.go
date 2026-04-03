// Package output writes extracted browser data to files.
//
// Architecture:
//   - Formatter interface: serializes data to io.Writer (csv.go, json.go, cookie_editor.go)
//   - Write function: file management + calls Formatter
//
// Supported formats: csv, json, cookie-editor
package output

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/moond4rk/hackbrowserdata/log"
	"github.com/moond4rk/hackbrowserdata/types"
)

// utf8BOM is written at the start of CSV files for Excel compatibility.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// Write outputs all non-empty categories from data to files in dir.
// Each category gets its own file (e.g. password.csv, cookie.csv).
// Browser and profile are written as columns by the Formatter, not as filenames.
func Write(data *types.BrowserData, browser, profile, dir string, f Formatter) {
	data.Each(func(cd types.CategoryData) {
		filename := fmt.Sprintf("%s.%s", cd.Category, f.Ext())
		path := filepath.Join(dir, filename)

		if err := writeToFile(path, f, cd, browser, profile); err != nil {
			log.Debugf("write %s: %v", filename, err)
			return
		}
		log.Warnf("export: %s (%d entries)", path, cd.Len)
	})
}

func writeToFile(path string, f Formatter, cd types.CategoryData, browser, profile string) error {
	file, err := openFileAppend(path)
	if err != nil {
		return err
	}
	defer file.Close()

	return f.Format(file, cd, browser, profile)
}

func openFileAppend(path string) (*os.File, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("create dir %s: %w", dir, err)
	}

	f, err := os.OpenFile(filepath.Clean(path), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}

	// Write UTF-8 BOM at the start of new CSV files
	if strings.HasSuffix(path, ".csv") {
		info, _ := f.Stat()
		if info != nil && info.Size() == 0 {
			if _, err := f.Write(utf8BOM); err != nil {
				f.Close()
				return nil, fmt.Errorf("write BOM: %w", err)
			}
		}
	}

	return f, nil
}
