package main

import (
	"fmt"
	"io"
	"path/filepath"
	"text/tabwriter"

	"github.com/moond4rk/hackbrowserdata/internal/app"
	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/internal/export"
	"github.com/moond4rk/hackbrowserdata/internal/log"
)

// printSummary is not part of the output, so failing to print it never fails the run.
func printSummary(stderr io.Writer, summary app.Summary) {
	table := tabwriter.NewWriter(stderr, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "Browser\tProfile\tCategory\tResult\tRecords\tDetail")
	for _, failure := range summary.DirFailures {
		fmt.Fprintf(table, "%s\t-\t-\t%s\t0\t%s\n", failure.DirID, app.OutcomeError, failure.Err)
	}
	for _, result := range summary.Results {
		fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%d\t%s\n",
			result.DirID, result.Profile.Name, result.Category, result.Outcome, result.Records, resultDetail(result))
	}
	if err := table.Flush(); err != nil {
		log.Warnf("print summary: %v", err)
	}
}

func resultDetail(result app.Result) string {
	switch {
	case result.Err != nil:
		return result.Err.Error()
	case result.Failures > 0:
		return fmt.Sprintf("%d not recovered", result.Failures)
	default:
		return ""
	}
}

// printNothingFound lists discovery's failures first, the likely reason a run found no browser data.
func printNothingFound(stderr io.Writer, discovery app.Discovery, message string) {
	if len(discovery.DirFailures) > 0 {
		printSummary(stderr, app.Summary{DirFailures: discovery.DirFailures})
	}
	log.Warnf("%s", message)
}

// printExported lists the files a run wrote, even when it stopped early, so the user finds what it left.
func printExported(stderr io.Writer, exported export.Listing) {
	if len(exported.Files) == 0 {
		return
	}
	fmt.Fprintln(stderr)
	log.Infof("Exported to %s/", exported.Dir)
	for _, file := range exported.Files {
		log.Infof("  %-24s %d entries", file.Name, file.Count)
	}
	if exported.ZipName != "" {
		log.Infof("Compressed: %s/%s", exported.Dir, exported.ZipName)
	}
}

func printProfiles(out io.Writer, discovery app.Discovery) error {
	table := tabwriter.NewWriter(out, 0, 0, 3, ' ', 0)
	fmt.Fprintln(table, "Browser\tProfile\tPath")
	for _, dir := range discovery.Dirs {
		for _, profile := range dir.Profiles {
			profilePath := filepath.Join(dir.Path, filepath.FromSlash(profile.RelativePath))
			fmt.Fprintf(table, "%s\t%s\t%s\n", dir.Browser.DisplayName, profile.Name, profilePath)
		}
	}
	return table.Flush()
}

func printRecordCounts(out io.Writer, discovery app.Discovery, summary app.Summary) error {
	counts := make(map[app.Extraction]int, len(summary.Results))
	for _, result := range summary.Results {
		counts[result.Extraction] = result.Records
	}

	table := tabwriter.NewWriter(out, 0, 0, 3, ' ', 0)
	fmt.Fprint(table, "Browser\tProfile")
	for _, dataCategory := range category.All() {
		fmt.Fprintf(table, "\t%s", dataCategory.String())
	}
	fmt.Fprintln(table)
	for _, dir := range discovery.Dirs {
		for _, profile := range dir.Profiles {
			fmt.Fprintf(table, "%s\t%s", dir.Browser.DisplayName, profile.Name)
			for _, dataCategory := range category.All() {
				fmt.Fprintf(table, "\t%d", counts[app.Extraction{DirID: dir.ID, Profile: profile, Category: dataCategory}])
			}
			fmt.Fprintln(table)
		}
	}
	return table.Flush()
}
