package main

import (
	"github.com/spf13/cobra"

	"github.com/moond4rk/hackbrowserdata/browser"
	"github.com/moond4rk/hackbrowserdata/log"
)

func archiveCmd() *cobra.Command {
	var (
		browserName string
		category    string
		outputPath  string
	)

	cmd := &cobra.Command{
		Use:   "archive",
		Short: "Pack decryption-relevant profile files into a zip for cross-host restore",
		Example: `  hack-browser-data archive
  hack-browser-data archive -b chrome -c cookie -o chrome-cookies.zip`,
		RunE: func(cmd *cobra.Command, args []string) error {
			browsers, err := browser.DiscoverBrowsers(browser.DiscoverOptions{Name: browserName})
			if err != nil {
				return err
			}
			if len(browsers) == 0 {
				log.Warnf("no browsers found")
				return nil
			}
			categories, err := parseCategories(category)
			if err != nil {
				return err
			}
			n, err := browser.WriteArchive(browsers, categories, outputPath)
			if err != nil {
				return err
			}
			log.Infof("Archived %d entries to %s", n, outputPath)
			return nil
		},
	}

	cmd.Flags().StringVarP(&browserName, "browser", "b", "all", "target browser: all|"+browser.Names())
	cmd.Flags().StringVarP(&category, "category", "c", "all", "data categories (comma-separated): all|"+categoryNames())
	cmd.Flags().StringVarP(&outputPath, "output", "o", "browser-data.zip", "output archive of decryption-relevant browser files")

	return cmd
}
