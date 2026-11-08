package main

import (
	"context"
	"io"

	"github.com/spf13/cobra"

	"github.com/moond4rk/hackbrowserdata/internal/app"
	"github.com/moond4rk/hackbrowserdata/internal/log"
)

func newDumpCommand() *cobra.Command {
	var flags app.Flags

	command := &cobra.Command{
		Use:   "dump",
		Short: "Extract and decrypt browser data (default command)",
		Example: `  hack-browser-data dump
  hack-browser-data dump -b chrome -c password,cookie
  hack-browser-data dump -b chrome -f json -d output
  hack-browser-data dump -f cookie-editor
  hack-browser-data dump --zip`,
		RunE: func(command *cobra.Command, args []string) error {
			return runDump(command.Context(), command.ErrOrStderr(), flags)
		},
	}

	command.Flags().StringVarP(&flags.Browser, "browser", "b", "all", "target browser: all|"+browserNames())
	command.Flags().StringVarP(&flags.Categories, "category", "c", "all", "data categories (comma-separated): all|"+categoryNames())
	command.Flags().StringVarP(&flags.Format, "format", "f", "json", "output format: csv|json|cookie-editor")
	command.Flags().StringVarP(&flags.OutputDir, "dir", "d", "results", "output directory")
	command.Flags().StringVarP(&flags.ProfilePath, "profile-path", "p", "", "custom profile dir path, get with chrome://version")
	addKeychainPasswordFlags(command, &flags)
	command.Flags().BoolVar(&flags.Zip, "zip", false, "compress output to zip")

	return command
}

func addKeychainPasswordFlags(command *cobra.Command, flags *app.Flags) {
	command.Flags().StringVar(&flags.KeychainPassword, "keychain-pw", "", "macOS keychain password (visible in ps and shell history)")
	command.Flags().BoolVar(&flags.KeychainPasswordStdin, "keychain-pw-stdin", false,
		"read the macOS keychain password from the first line of stdin")
	command.MarkFlagsMutuallyExclusive("keychain-pw", "keychain-pw-stdin")
}

func runDump(ctx context.Context, stderr io.Writer, flags app.Flags) error {
	options, err := app.ValidateDump(flags)
	if err != nil {
		return err
	}
	discovery := app.Discover(options)
	if len(discovery.Dirs) == 0 {
		if len(discovery.Errors) > 0 {
			report(stderr, app.Summary{Results: discovery.Errors})
		}
		log.Warnf("no browsers found")
		return nil
	}
	summary, err := app.Execute(ctx, app.NewPlan(discovery, options), options)
	if ctx.Err() == nil {
		report(stderr, summary)
	}
	return err
}
