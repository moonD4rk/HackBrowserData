package main

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/moond4rk/hackbrowserdata/log"
)

var verbose bool

func rootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "hack-browser-data",
		Short: "A CLI tool for decrypting and exporting browser data",
		Long: `hack-browser-data decrypts and exports browser data from Chromium-based
browsers and Firefox on Windows, macOS, and Linux.

GitHub: https://github.com/moonD4rk/HackBrowserData`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if verbose {
				log.SetVerbose()
			}
		},
	}

	root.CompletionOptions.HiddenDefaultCmd = true

	root.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable debug logging")

	dump := dumpCmd()
	root.AddCommand(dump, listCmd(), versionCmd())

	// Default to dump when no subcommand is given.
	// Copy dump flags to root so that `hack-browser-data -b chrome`
	// works the same as `hack-browser-data dump -b chrome`.
	root.RunE = func(cmd *cobra.Command, args []string) error {
		return dump.RunE(dump, args)
	}
	dump.Flags().VisitAll(func(f *pflag.Flag) {
		if root.Flags().Lookup(f.Name) == nil {
			root.Flags().AddFlag(f)
		}
	})

	return root
}

func main() {
	configureDoubleClickMode()
	if err := rootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
