package app

import (
	"errors"
	"io/fs"

	"github.com/moond4rk/hackbrowserdata/internal/log"
	"github.com/moond4rk/hackbrowserdata/internal/record"
)

type collector struct {
	browserName, profileName string
	records                  []record.Record
	failures                 int
	firstFailure             error
}

func (c *collector) Add(row record.Record) {
	row.SetBrowserAndProfile(c.browserName, c.profileName)
	c.records = append(c.records, row)
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
