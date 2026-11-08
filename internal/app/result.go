package app

import "github.com/moond4rk/hackbrowserdata/internal/browser"

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

// Result is one extraction's outcome, or a failure of a whole user data dir when Profile and Category are empty.
type Result struct {
	Extraction
	Outcome  Outcome
	Records  int   // records emitted
	Failures int   // record-level problems reported
	Err      error // set when Outcome is OutcomeError
}

// Summary is every result of a run, in output order: what never ran first, then each user data dir's extractions.
type Summary struct {
	Results []Result
}

func dirFailure(id browser.UserDataDirID, err error) Result {
	return Result{Extraction: Extraction{UserDataDirID: id}, Outcome: OutcomeError, Err: err}
}
