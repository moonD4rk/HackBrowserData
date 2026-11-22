package main

import (
	"context"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/moond4rk/hackbrowserdata/internal/app"
)

func newDumpCommand() *cobra.Command {
	var (
		flags         app.DumpFlags
		keychainFlags passwordFlags
		firefoxFlags  passwordFlags
	)

	command := &cobra.Command{
		Use:   "dump",
		Short: "Extract and decrypt browser data (default command)",
		Example: `  hack-browser-data dump
  hack-browser-data dump -b chrome -c password,cookie
  hack-browser-data dump -b chrome -f json -d output
  hack-browser-data dump -f cookie-editor
  hack-browser-data dump --zip`,
		RunE: func(command *cobra.Command, args []string) error {
			return runDump(command.Context(), command.ErrOrStderr(), flags, keychainFlags, firefoxFlags)
		},
	}

	command.Flags().StringVarP(&flags.Browser, "browser", "b", "all", "target browser: all|"+browserNames())
	command.Flags().StringVarP(&flags.Categories, "category", "c", "all", "data categories (comma-separated): all|"+categoryNames())
	command.Flags().StringVarP(&flags.ProfilePath, "profile-path", "p", "", "custom profile dir path, get with chrome://version")
	addOutputFlags(command, &flags.Output)
	addPasswordFlags(command, &keychainFlags, "keychain-pw", "macOS keychain password")
	addPasswordFlags(command, &firefoxFlags, "firefox-pw", "Firefox primary password")
	command.MarkFlagsMutuallyExclusive("keychain-pw-stdin", "firefox-pw-stdin")

	return command
}

func runDump(ctx context.Context, stderr io.Writer, flags app.DumpFlags, keychainFlags, firefoxFlags passwordFlags) error {
	options, err := app.ValidateDump(flags)
	if err != nil {
		return err
	}
	options.KeychainPassword = keychainPasswordFunc(keychainFlags, os.Stdin)
	if options.FirefoxPrimaryPassword, err = readFirefoxPassword(ctx, firefoxFlags, os.Stdin); err != nil {
		return err
	}
	discovery, summary, err := app.Dump(ctx, options)
	if err != nil {
		return err
	}
	if len(discovery.Dirs) == 0 {
		printNothingFound(stderr, discovery, "no browsers found")
		return nil
	}
	printExported(stderr, summary.Exported)
	if ctx.Err() == nil {
		printSummary(stderr, summary)
	}
	return summary.Err
}
