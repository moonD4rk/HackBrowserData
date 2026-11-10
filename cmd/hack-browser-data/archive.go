package main

import (
	"github.com/spf13/cobra"

	"github.com/moond4rk/hackbrowserdata/internal/app"
	"github.com/moond4rk/hackbrowserdata/internal/log"
)

func newArchiveCommand() *cobra.Command {
	var (
		flags      app.ArchiveFlags
		outputPath string
	)

	command := &cobra.Command{
		Use:   "archive",
		Short: "Pack decryption-relevant profile files into a zip for cross-host restore",
		Example: `  hack-browser-data archive
  hack-browser-data archive -b chrome -c cookie -o chrome-cookies.zip`,
		RunE: func(command *cobra.Command, args []string) error {
			options, err := app.ValidateArchive(flags)
			if err != nil {
				return err
			}
			discovery := app.Discover(command.Context(), options.Target)
			if warnIfNothingFound(command.ErrOrStderr(), discovery, "no browsers found") {
				return nil
			}
			fileCount, err := app.Archive(command.Context(), discovery, options.Categories, outputPath)
			if err != nil {
				return err
			}
			log.Infof("Archived %d entries to %s", fileCount, outputPath)
			return nil
		},
	}

	command.Flags().StringVarP(&flags.Browser, "browser", "b", "all", "target browser: all|"+browserNames())
	command.Flags().StringVarP(&flags.Categories, "category", "c", "all", "data categories (comma-separated): all|"+categoryNames())
	command.Flags().StringVarP(&outputPath, "output", "o", "browser-data.zip", "output archive of decryption-relevant browser files")

	return command
}
