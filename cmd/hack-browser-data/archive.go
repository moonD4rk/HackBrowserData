package main

import (
	"github.com/spf13/cobra"

	"github.com/moond4rk/hackbrowserdata/internal/app"
	"github.com/moond4rk/hackbrowserdata/internal/log"
)

func newArchiveCommand() *cobra.Command {
	var flags app.ArchiveFlags

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
			discovery, fileCount, err := app.Archive(command.Context(), options)
			if err != nil {
				return err
			}
			if len(discovery.Dirs) == 0 {
				printDirFailures(command.ErrOrStderr(), discovery.DirFailures)
				log.Warnf("no browsers found")
				return nil
			}
			log.Infof("Archived %d entries to %s", fileCount, options.ArchivePath)
			return nil
		},
	}

	command.Flags().StringVarP(&flags.Browser, "browser", "b", "all", "target browser: all|"+browserNames())
	command.Flags().StringVarP(&flags.Categories, "category", "c", "all", "data categories (comma-separated): all|"+categoryNames())
	command.Flags().StringVarP(&flags.OutputPath, "output", "o", "browser-data.zip", "output archive of decryption-relevant browser files")

	return command
}
