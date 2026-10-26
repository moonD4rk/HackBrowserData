package main

import (
	"github.com/spf13/cobra"

	"github.com/moond4rk/hackbrowserdata/browser"
	"github.com/moond4rk/hackbrowserdata/internal/log"
)

func dumpCmd() *cobra.Command {
	var (
		browserName  string
		category     string
		outputFormat string
		outputDir    string
		profilePath  string
		keychainPw   string
		compress     bool
	)

	cmd := &cobra.Command{
		Use:   "dump",
		Short: "Extract and decrypt browser data (default command)",
		Example: `  hack-browser-data dump
  hack-browser-data dump -b chrome -c password,cookie
  hack-browser-data dump -b chrome -f json -d output
  hack-browser-data dump -f cookie-editor
  hack-browser-data dump --zip`,
		RunE: func(cmd *cobra.Command, args []string) error {
			browsers, err := browser.DiscoverBrowsersWithKeys(browser.DiscoverOptions{
				Name:             browserName,
				ProfilePath:      profilePath,
				KeychainPassword: keychainPw,
			})
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
			return extractAndWrite(browsers, categories, outputDir, outputFormat, compress)
		},
	}

	cmd.Flags().StringVarP(&browserName, "browser", "b", "all", "target browser: all|"+browser.Names())
	cmd.Flags().StringVarP(&category, "category", "c", "all", "data categories (comma-separated): all|"+categoryNames())
	cmd.Flags().StringVarP(&outputFormat, "format", "f", "json", "output format: csv|json|cookie-editor")
	cmd.Flags().StringVarP(&outputDir, "dir", "d", "results", "output directory")
	cmd.Flags().StringVarP(&profilePath, "profile-path", "p", "", "custom profile dir path, get with chrome://version")
	cmd.Flags().StringVar(&keychainPw, "keychain-pw", "", "macOS keychain password")
	cmd.Flags().BoolVar(&compress, "zip", false, "compress output to zip")

	return cmd
}
