package app

import (
	"errors"
	"fmt"
	"io/fs"
	"unicode/utf8"

	"github.com/moond4rk/hackbrowserdata/internal/log"
	"github.com/moond4rk/hackbrowserdata/internal/record"
)

type collector struct {
	browserName, profileName string
	records                  []record.Record
	failures                 int
	firstFailure             error
	maxStorageValue          int
}

func (c *collector) Add(row record.Record) {
	row.SetBrowserAndProfile(c.browserName, c.profileName)
	capStorageValue(row, c.maxStorageValue)
	c.records = append(c.records, row)
}

// capStorageValue replaces an over-long localStorage/sessionStorage value with a size marker; limit <= 0 keeps it whole.
func capStorageValue(row record.Record, limit int) {
	if limit <= 0 {
		return
	}
	switch stored := row.(type) {
	case *record.LocalStorage:
		if !stored.IsMeta {
			stored.Value = cappedValue(stored.Value, limit)
		}
	case *record.SessionStorage:
		if !stored.IsMeta {
			stored.Value = cappedValue(stored.Value, limit)
		}
	}
}

func cappedValue(value string, limit int) string {
	if count := utf8.RuneCountInString(value); count > limit {
		return fmt.Sprintf("value too long: %d chars (max %d, raise with --max-storage-value)", count, limit)
	}
	return value
}

func (c *collector) Fail(err error) {
	c.failures++
	if c.firstFailure == nil {
		c.firstFailure = err
	}
	log.Debugf("%s/%s: %v", c.browserName, c.profileName, err)
}

// result treats a file gone since planning as absent; failures alone are an error, and failures beside records partial.
func (c *collector) result(extraction Extraction, err error) Result {
	result := Result{Extraction: extraction, Records: len(c.records), Failures: c.failures}
	switch {
	case err != nil && errors.Is(err, fs.ErrNotExist) && result.Records == 0:
		result.Outcome = OutcomeAbsent
	case err != nil:
		result.Outcome, result.Err = OutcomeError, err
	case result.Failures > 0 && result.Records == 0:
		result.Outcome, result.Err = OutcomeError, c.firstFailure
	case result.Failures > 0:
		result.Outcome = OutcomePartial
	default:
		result.Outcome = OutcomeOK
	}
	return result
}
