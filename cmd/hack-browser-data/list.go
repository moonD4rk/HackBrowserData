package main

import (
	"github.com/spf13/cobra"

	"github.com/moond4rk/hackbrowserdata/internal/app"
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
			options, err := app.ValidateList()
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
			summary, err := app.CountRecords(command.Context(), discovery, options.Keychain)
			if err != nil {
				return err
			}
			return printRecordCounts(command.OutOrStdout(), discovery, summary)
		},
	}

	command.Flags().BoolVar(&detail, "detail", false, "show per-category entry counts")
	return command
}
