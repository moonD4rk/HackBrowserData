package app

import (
	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/internal/export"
)

// Extraction is plain data, so a Result prints as it is and list can key record counts by it.
type Extraction struct {
	UserDataDirID browser.UserDataDirID
	ProfileName   string // Profile.Name
	Category      category.Category
}

// Outcome is how one extraction ended. It starts at 1, so a result whose outcome was never set does not read as ok.
type Outcome int

const (
	OutcomeOK Outcome = iota + 1
	OutcomePartial
	OutcomeAbsent
	OutcomeUnsupported
	OutcomeError
)

func (o Outcome) String() string {
	switch o {
	case OutcomeOK:
		return "ok"
	case OutcomePartial:
		return "partial"
	case OutcomeAbsent:
		return "absent"
	case OutcomeUnsupported:
		return "unsupported"
	case OutcomeError:
		return "error"
	default:
		return "unset"
	}
}

// Result is one extraction's outcome.
type Result struct {
	Extraction
	Outcome  Outcome
	Records  int   // records emitted
	Failures int   // record-level problems reported
	Err      error // set when Outcome is OutcomeError
}

// DirFailure is a whole user data dir that could not be read, or whose master keys could not be retrieved.
type DirFailure struct {
	UserDataDirID browser.UserDataDirID
	Err           error
}

// Summary lists each dir's results in turn, what never ran before what ran, and the dirs that failed as a whole.
type Summary struct {
	Results     []Result
	DirFailures []DirFailure
	Exported    export.Output // the files Execute wrote; empty when nothing was exported
}
