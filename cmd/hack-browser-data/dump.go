package main

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/moond4rk/hackbrowserdata/internal/app"
	"github.com/moond4rk/hackbrowserdata/internal/log"
)

func dumpCmd() *cobra.Command {
	var f app.Flags

	cmd := &cobra.Command{
		Use:   "dump",
		Short: "Extract and decrypt browser data (default command)",
		Example: `  hack-browser-data dump
  hack-browser-data dump -b chrome -c password,cookie
  hack-browser-data dump -b chrome -f json -d output
  hack-browser-data dump -f cookie-editor
  hack-browser-data dump --zip`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDump(cmd.Context(), f)
		},
	}

	cmd.Flags().StringVarP(&f.Browser, "browser", "b", "all", "target browser: all|"+browserNames())
	cmd.Flags().StringVarP(&f.Categories, "category", "c", "all", "data categories (comma-separated): all|"+categoryNames())
	cmd.Flags().StringVarP(&f.Format, "format", "f", "json", "output format: csv|json|cookie-editor")
	cmd.Flags().StringVarP(&f.Dir, "dir", "d", "results", "output directory")
	cmd.Flags().StringVarP(&f.ProfilePath, "profile-path", "p", "", "custom profile dir path, get with chrome://version")
	keychainPasswordFlags(cmd, &f)
	cmd.Flags().BoolVar(&f.Zip, "zip", false, "compress output to zip")

	return cmd
}

func keychainPasswordFlags(cmd *cobra.Command, f *app.Flags) {
	cmd.Flags().StringVar(&f.KeychainPassword, "keychain-pw", "", "macOS keychain password (visible in ps and shell history)")
	cmd.Flags().BoolVar(&f.KeychainPasswordStdin, "keychain-pw-stdin", false, "read the macOS keychain password from the first line of stdin")
	cmd.MarkFlagsMutuallyExclusive("keychain-pw", "keychain-pw-stdin")
}

func runDump(ctx context.Context, f app.Flags) error {
	req, err := app.Validate(f)
	if err != nil {
		return err
	}
	d := app.Discover(req)
	if len(d.Dirs) == 0 {
		if len(d.Failed) > 0 {
			report(app.Summary{Results: d.Failed})
		}
		log.Warnf("no browsers found")
		return nil
	}
	summary, err := app.Execute(ctx, app.NewPlan(d, req), req)
	if ctx.Err() == nil {
		report(summary)
	}
	return err
}
