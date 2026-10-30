package app

import (
	"fmt"
	"io"
	"text/tabwriter"
)

// Report writes every result as one line, in output order. The summary is additive: it is not part of the output
// contract, and it never changes the exit status.
func Report(w io.Writer, s Summary) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "Browser\tProfile\tCategory\tResult\tRecords\tDetail")
	for _, r := range s.Results {
		profile, cat := "-", "-"
		if r.Profile != "" {
			profile = r.Profile
		}
		if r.Category != 0 {
			cat = r.Category.String()
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%d\t%s\n", r.DirID, profile, cat, r.Outcome, r.Count, detail(r))
	}
	return tw.Flush()
}

func detail(r Result) string {
	switch {
	case r.Err != nil:
		return r.Err.Error()
	case r.Failed > 0:
		return fmt.Sprintf("%d not recovered", r.Failed)
	default:
		return ""
	}
}
