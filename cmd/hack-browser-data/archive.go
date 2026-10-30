package main

import (
	"github.com/spf13/cobra"

	"github.com/moond4rk/hackbrowserdata/internal/app"
	"github.com/moond4rk/hackbrowserdata/internal/log"
)

func archiveCmd() *cobra.Command {
	var (
		f          app.Flags
		outputPath string
	)

	cmd := &cobra.Command{
		Use:   "archive",
		Short: "Pack decryption-relevant profile files into a zip for cross-host restore",
		Example: `  hack-browser-data archive
  hack-browser-data archive -b chrome -c cookie -o chrome-cookies.zip`,
		RunE: func(cmd *cobra.Command, args []string) error {
			req, err := app.ValidateArchive(f)
			if err != nil {
				return err
			}
			d := app.Discover(req)
			if len(d.Dirs) == 0 {
				log.Warnf("no browsers found")
				return nil
			}
			n, err := app.Archive(cmd.Context(), d, req, outputPath)
			if err != nil {
				return err
			}
			log.Infof("Archived %d entries to %s", n, outputPath)
			return nil
		},
	}

	cmd.Flags().StringVarP(&f.Browser, "browser", "b", "all", "target browser: all|"+browserNames())
	cmd.Flags().StringVarP(&f.Categories, "category", "c", "all", "data categories (comma-separated): all|"+categoryNames())
	cmd.Flags().StringVarP(&outputPath, "output", "o", "browser-data.zip", "output archive of decryption-relevant browser files")

	return cmd
}
