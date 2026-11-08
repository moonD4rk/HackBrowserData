package main

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/moond4rk/hackbrowserdata/internal/app"
	"github.com/moond4rk/hackbrowserdata/internal/log"
	"github.com/moond4rk/hackbrowserdata/internal/snapshot"
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
	command.Flags().StringVarP(&flags.Format, "format", "f", "json", "output format: csv|json|cookie-editor")
	command.Flags().StringVarP(&flags.OutputDir, "dir", "d", "results", "output directory")
	command.Flags().BoolVar(&flags.Zip, "zip", false, "compress output to zip")

	_ = command.MarkFlagRequired("keys")
	command.MarkFlagsMutuallyExclusive("data-dir", "data-zip")

	return command
}

func runRestore(ctx context.Context, stderr io.Writer, flags app.RestoreFlags) error {
	options, keyFile, err := app.ValidateRestore(flags)
	if err != nil {
		return err
	}
	data, err := openRestoreData(flags.DataDir, flags.DataZip)
	if err != nil {
		return err
	}
	defer closeRestoreData(data)

	discovery, keys, err := app.DiscoverRestore(data.Path(), keyFile, options.Target.BrowserName)
	if err != nil {
		return err
	}
	if foundNothing(stderr, discovery, "no browsers to restore from the supplied keys and data") {
		return nil
	}
	summary, err := app.ExecuteWithKeys(ctx, discovery, options.Categories, options.Output, keys)
	announceExport(stderr, summary.Exported)
	if ctx.Err() == nil {
		report(stderr, summary)
	}
	return err
}

func openRestoreData(dataDir, dataZip string) (*snapshot.Dir, error) {
	if dataDir != "" {
		return snapshot.Open(dataDir), nil
	}
	snapshots, err := snapshot.Unzip(dataZip)
	if err != nil {
		return nil, fmt.Errorf("extract %s: %w", dataZip, err)
	}
	return snapshots, nil
}

func closeRestoreData(snapshots *snapshot.Dir) {
	if err := snapshots.Close(); err != nil {
		log.Warnf("restore: %v", err)
	}
}
