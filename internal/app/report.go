package app

import (
	"fmt"
	"io"
	"text/tabwriter"
)

// Report prints a summary that is not part of the output contract and never changes the exit status.
func Report(writer io.Writer, summary Summary) error {
	table := tabwriter.NewWriter(writer, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "Browser\tProfile\tCategory\tResult\tRecords\tDetail")
	for _, result := range summary.Results {
		profile, cat := "-", "-"
		if result.Profile != "" {
			profile = result.Profile
		}
		if result.Category != 0 {
			cat = result.Category.String()
		}
		fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%d\t%s\n", result.UserDataDirID, profile, cat, result.Outcome, result.Count, detail(result))
	}
	return table.Flush()
}

func detail(result Result) string {
	switch {
	case result.Err != nil:
		return result.Err.Error()
	case result.Failed > 0:
		return fmt.Sprintf("%d not recovered", result.Failed)
	default:
		return ""
	}
}
