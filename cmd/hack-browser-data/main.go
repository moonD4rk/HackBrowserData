package main

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/moond4rk/hackbrowserdata/log"
)

var verbose bool

func rootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "hack-browser-data",
		Short: "Export passwords|bookmarks|cookies|history|credit cards|downloads|localStorage|extensions from browser",
		Long: `Export all browsing data (passwords/cookies/history/bookmarks) from browser.
Github Link: https://github.com/moonD4rk/HackBrowserData`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if verbose {
				log.SetVerbose()
			}
		},
	}

	root.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable debug logging")

	dump := dumpCmd()
	root.AddCommand(dump, listCmd(), versionCmd())

	// Default to dump when no subcommand is given.
	root.RunE = dump.RunE

	return root
}

func main() {
	if err := rootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
