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

func restoreCmd() *cobra.Command {
	var f app.RestoreFlags

	cmd := &cobra.Command{
		Use:   "restore",
		Short: "Decrypt copied profile data using exported master keys",
		Example: `  hack-browser-data restore --keys keys.json --data-zip data.zip
  hack-browser-data restore --keys keys.json --data-dir ./data -b chrome -c cookie
  hack-browser-data restore --keys keys.json --data-dir ./chrome-userdata -b chrome
  ssh origin "hack-browser-data dumpkeys" | hack-browser-data restore --keys - --data-zip data.zip`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRestore(cmd.Context(), cmd.ErrOrStderr(), f)
		},
	}

	cmd.Flags().StringVar(&f.Keys, "keys", "", "keys file from dumpkeys (use - for stdin)")
	cmd.Flags().StringVar(&f.DataDir, "data-dir", "", "copied profile data dir (archive layout, or one browser's User Data with -b)")
	cmd.Flags().StringVar(&f.DataZip, "data-zip", "", "zip produced by the archive command (alternative to --data-dir)")
	cmd.Flags().StringVarP(&f.Browser, "browser", "b", "", "restore only this browser (optional; must match a vault in --keys)")
	cmd.Flags().StringVarP(&f.Categories, "category", "c", "all", "data categories (comma-separated): all|"+categoryNames())
	cmd.Flags().StringVarP(&f.Format, "format", "f", "json", "output format: csv|json|cookie-editor")
	cmd.Flags().StringVarP(&f.Dir, "dir", "d", "results", "output directory")
	cmd.Flags().BoolVar(&f.Zip, "zip", false, "compress output to zip")

	_ = cmd.MarkFlagRequired("keys")
	cmd.MarkFlagsMutuallyExclusive("data-dir", "data-zip")

	return cmd
}

func runRestore(ctx context.Context, stderr io.Writer, f app.RestoreFlags) error {
	req, dump, err := app.ValidateRestore(f)
	if err != nil {
		return err
	}
	data, err := openRestoreData(f.DataDir, f.DataZip)
	if err != nil {
		return err
	}
	defer closeRestoreData(data)

	d, keys, err := app.RestoreDiscovery(data, dump, req)
	if err != nil {
		return err
	}
	if len(d.Dirs) == 0 {
		log.Warnf("no browsers to restore from the supplied keys and data")
		return nil
	}
	req.Keys = keys
	summary, err := app.Execute(ctx, app.NewPlan(d, req), req)
	if ctx.Err() == nil {
		report(stderr, summary)
	}
	return err
}

// openRestoreData returns the snapshot directory restore reads from: --data-dir in place, or --data-zip extracted
// into a temporary one.
func openRestoreData(dataDir, dataZip string) (*snapshot.Dir, error) {
	if dataDir != "" {
		return snapshot.Open(dataDir), nil
	}
	d, err := snapshot.Unzip(dataZip)
	if err != nil {
		return nil, fmt.Errorf("extract %s: %w", dataZip, err)
	}
	return d, nil
}

func closeRestoreData(d *snapshot.Dir) {
	if err := d.Close(); err != nil {
		log.Warnf("restore: %v", err)
	}
}
