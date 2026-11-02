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

func listCmd() *cobra.Command {
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
			discovery := app.Discover(options)
			for _, result := range discovery.Failed {
				log.Warnf("%s: %v", result.UserDataDirID, result.Err)
			}
			if len(discovery.Dirs) == 0 {
				command.Println("No browsers found.")
				return nil
			}
			if !detail {
				return printBasic(command.OutOrStdout(), discovery)
			}
			summary, err := app.Count(command.Context(), app.NewPlan(discovery, options), options)
			if err != nil {
				return err
			}
			return printDetail(command.OutOrStdout(), discovery, summary)
		},
	}

	command.Flags().BoolVar(&detail, "detail", false, "show per-category entry counts")
	return command
}

func printBasic(out io.Writer, discovery app.Discovery) error {
	table := tabwriter.NewWriter(out, 0, 0, 3, ' ', 0)
	fmt.Fprintln(table, "Browser\tProfile\tPath")
	for _, dir := range discovery.Dirs {
		for _, profile := range dir.Profiles {
			fmt.Fprintf(table, "%s\t%s\t%s\n", dir.Browser.DisplayName, profile.Name, filepath.Join(dir.Path, filepath.FromSlash(profile.Dir)))
		}
	}
	return table.Flush()
}

func printDetail(out io.Writer, discovery app.Discovery, summary app.Summary) error {
	counts := make(map[app.Extraction]int, len(summary.Results))
	for _, result := range summary.Results {
		counts[result.Extraction] = result.Count
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
				fmt.Fprintf(table, "\t%d", counts[app.Extraction{UserDataDirID: dir.ID, Profile: profile.Name, Category: dataCategory}])
			}
			fmt.Fprintln(table)
		}
	}
	return table.Flush()
}
