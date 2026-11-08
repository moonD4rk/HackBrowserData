package main

import (
	"fmt"
	"io"
	"path/filepath"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/moond4rk/hackbrowserdata/internal/app"
	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/internal/log"
)

func newListCommand() *cobra.Command {
	var detail bool

	command := &cobra.Command{
		Use:   "list",
		Short: "List detected browsers and profiles",
		Example: `  hack-browser-data list
  hack-browser-data list --detail`,
		RunE: func(command *cobra.Command, args []string) error {
			options, err := app.ListOptions()
			if err != nil {
				return err
			}
			discovery := app.Discover(options.Target)
			for _, failure := range discovery.DirFailures {
				log.Warnf("%s: %v", failure.UserDataDirID, failure.Err)
			}
			if len(discovery.Dirs) == 0 {
				command.Println("No browsers found.")
				return nil
			}
			if !detail {
				return printProfiles(command.OutOrStdout(), discovery)
			}
			plan := app.NewPlan(discovery, options.Categories)
			summary, err := app.CountRecords(command.Context(), plan, options.Keychain)
			if err != nil {
				return err
			}
			return printRecordCounts(command.OutOrStdout(), discovery, summary)
		},
	}

	command.Flags().BoolVar(&detail, "detail", false, "show per-category entry counts")
	return command
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
				fmt.Fprintf(table, "\t%d", counts[app.Extraction{UserDataDirID: dir.ID, ProfileName: profile.Name, Category: dataCategory}])
			}
			fmt.Fprintln(table)
		}
	}
	return table.Flush()
}
