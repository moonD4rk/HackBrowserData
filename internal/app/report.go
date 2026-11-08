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
	for _, failure := range summary.DirFailures {
		fmt.Fprintf(table, "%s\t-\t-\t%s\t0\t%s\n", failure.UserDataDirID, OutcomeError, failure.Err)
	}
	for _, result := range summary.Results {
		fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%d\t%s\n",
			result.UserDataDirID, result.ProfileName, result.Category, result.Outcome, result.Records, detail(result))
	}
	return table.Flush()
}

func detail(result Result) string {
	switch {
	case result.Err != nil:
		return result.Err.Error()
	case result.Failures > 0:
		return fmt.Sprintf("%d not recovered", result.Failures)
	default:
		return ""
	}
}
