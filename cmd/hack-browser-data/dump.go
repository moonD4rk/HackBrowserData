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
		flags    app.DumpFlags
		keychain keychainFlags
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
			return runDump(command.Context(), command.ErrOrStderr(), flags, keychain)
		},
	}

	command.Flags().StringVarP(&flags.Browser, "browser", "b", "all", "target browser: all|"+browserNames())
	command.Flags().StringVarP(&flags.Categories, "category", "c", "all", "data categories (comma-separated): all|"+categoryNames())
	command.Flags().StringVarP(&flags.Format, "format", "f", "json", "output format: csv|json|cookie-editor")
	command.Flags().StringVarP(&flags.OutputDir, "dir", "d", "results", "output directory")
	command.Flags().StringVarP(&flags.ProfilePath, "profile-path", "p", "", "custom profile dir path, get with chrome://version")
	addKeychainPasswordFlags(command, &keychain)
	command.Flags().BoolVar(&flags.Zip, "zip", false, "compress output to zip")

	return command
}

func runDump(ctx context.Context, stderr io.Writer, flags app.DumpFlags, keychain keychainFlags) error {
	options, err := app.ValidateDump(flags, keychainPasswordFunc(keychain, os.Stdin))
	if err != nil {
		return err
	}
	discovery := app.Discover(options.Target)
	if foundNothing(stderr, discovery, "no browsers found") {
		return nil
	}
	summary, err := app.Execute(ctx, discovery, options.Categories, options.Output, options.Keychain)
	announceExport(stderr, summary.Exported)
	if ctx.Err() == nil {
		report(stderr, summary)
	}
	return err
}
