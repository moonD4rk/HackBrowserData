package main

import (
	"errors"

	"github.com/moond4rk/hackbrowserdata/browser"
	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/internal/export"
	"github.com/moond4rk/hackbrowserdata/internal/log"
	"github.com/moond4rk/hackbrowserdata/internal/record"
	"github.com/moond4rk/hackbrowserdata/types"
)

func extractAndWrite(browsers []browser.Browser, categories []category.Category, outputDir, outputFormat string, compress bool) error {
	format, err := export.ParseFormat(outputFormat)
	if err != nil {
		return err
	}
	w, err := export.NewWriter(outputDir, format, compress)
	if err != nil {
		return err
	}
	for _, b := range browsers {
		log.Infof("Extracting %s...", b.BrowserName())
		results, extractErr := b.Extract(categories)
		if extractErr != nil {
			log.Errorf("extract %s: %v", b.BrowserName(), extractErr)
		}
		for _, r := range results {
			if err := writeResult(w, b.BrowserName(), r); err != nil {
				return errors.Join(err, w.Close())
			}
		}
	}
	return w.Close()
}

// writeResult bridges today's BrowserData to the writer until the pipeline replaces it.
func writeResult(w *export.Writer, browserName string, r types.ExtractResult) error {
	if r.Data == nil {
		return nil
	}
	for _, c := range category.All() {
		records := profileRecords(r.Data, c)
		for _, rec := range records {
			record.Stamp(rec, browserName, r.Name)
		}
		if err := w.Write(c, records); err != nil {
			return err
		}
	}
	return nil
}

func profileRecords(d *types.BrowserData, c category.Category) []record.Record {
	switch c {
	case category.Password:
		return recordsOf(d.Passwords)
	case category.Cookie:
		return recordsOf(d.Cookies)
	case category.Bookmark:
		return recordsOf(d.Bookmarks)
	case category.History:
		return recordsOf(d.Histories)
	case category.Download:
		return recordsOf(d.Downloads)
	case category.CreditCard:
		return recordsOf(d.CreditCards)
	case category.Extension:
		return recordsOf(d.Extensions)
	case category.LocalStorage:
		return recordsOf(d.LocalStorage)
	case category.SessionStorage:
		return recordsOf(d.SessionStorage)
	default:
		return nil
	}
}

func recordsOf[T any, P interface {
	*T
	record.Record
}](items []T) []record.Record {
	records := make([]record.Record, len(items))
	for i := range items {
		records[i] = P(&items[i])
	}
	return records
}
