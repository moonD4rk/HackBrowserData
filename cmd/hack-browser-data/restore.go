package main

import (
	"context"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/moond4rk/hackbrowserdata/internal/app"
)

func newRestoreCommand() *cobra.Command {
	var flags app.RestoreFlags

	command := &cobra.Command{
		Use:   "restore",
		Short: "Decrypt copied profile data using exported master keys",
		Example: `  hack-browser-data restore --keys keys.json --data-zip data.zip
  hack-browser-data restore --keys keys.json --data-dir ./data -b chrome -c cookie
  hack-browser-data restore --keys keys.json --data-dir ./chrome-userdata -b chrome
  ssh origin "hack-browser-data dumpkeys" | hack-browser-data restore --keys - --data-zip data.zip`,
		RunE: func(command *cobra.Command, args []string) error {
			return runRestore(command.Context(), command.ErrOrStderr(), flags)
		},
	}

	command.Flags().StringVar(&flags.KeyFile, "keys", "", "keys file from dumpkeys (use - for stdin)")
	command.Flags().StringVar(&flags.DataDir, "data-dir", "", "copied profile data dir (archive layout, or one browser's User Data with -b)")
	command.Flags().StringVar(&flags.DataZip, "data-zip", "", "zip produced by the archive command (alternative to --data-dir)")
	command.Flags().StringVarP(&flags.Browser, "browser", "b", "", "restore only this browser (optional; must match a vault in --keys)")
	command.Flags().StringVarP(&flags.Categories, "category", "c", "all", "data categories (comma-separated): all|"+categoryNames())
	addOutputFlags(command, &flags.Output)

	_ = command.MarkFlagRequired("keys")
	command.MarkFlagsMutuallyExclusive("data-dir", "data-zip")

	return command
}

func runRestore(ctx context.Context, stderr io.Writer, flags app.RestoreFlags) error {
	options, err := app.ValidateRestore(flags, os.Stdin)
	if err != nil {
		return err
	}
	discovery, summary, err := app.Restore(ctx, options)
	if err != nil {
		return err
	}
	if len(discovery.Dirs) == 0 {
		printNothingFound(stderr, discovery, "no browsers to restore from the supplied keys and data")
		return nil
	}
	printExported(stderr, summary.Exported)
	if ctx.Err() == nil {
		printSummary(stderr, summary)
	}
	return summary.Err
}
