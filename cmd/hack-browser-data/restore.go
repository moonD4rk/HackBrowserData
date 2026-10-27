package main

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/moond4rk/hackbrowserdata/browser"
	"github.com/moond4rk/hackbrowserdata/internal/log"
	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
	"github.com/moond4rk/hackbrowserdata/internal/snapshot"
)

func restoreCmd() *cobra.Command {
	var (
		keysPath     string
		dataDir      string
		dataZip      string
		browserName  string
		category     string
		outputFormat string
		outputDir    string
		compress     bool
	)

	cmd := &cobra.Command{
		Use:   "restore",
		Short: "Decrypt copied profile data using exported master keys",
		Example: `  hack-browser-data restore --keys keys.json --data-zip data.zip
  hack-browser-data restore --keys keys.json --data-dir ./data -b chrome -c cookie
  hack-browser-data restore --keys keys.json --data-dir ./chrome-userdata -b chrome
  ssh origin "hack-browser-data dumpkeys" | hack-browser-data restore --keys - --data-zip data.zip`,
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := openRestoreData(dataDir, dataZip)
			if err != nil {
				return err
			}
			defer closeRestoreData(data)

			browsers, err := loadRestoreBrowsers(keysPath, data.Path(), browserName)
			if err != nil {
				return err
			}
			if len(browsers) == 0 {
				log.Warnf("no browsers to restore from the supplied keys and data")
				return nil
			}
			categories, err := parseCategories(category)
			if err != nil {
				return err
			}
			return extractAndWrite(browsers, categories, outputDir, outputFormat, compress)
		},
	}

	cmd.Flags().StringVar(&keysPath, "keys", "", "keys file from dumpkeys (use - for stdin)")
	cmd.Flags().StringVar(&dataDir, "data-dir", "", "copied profile data dir (archive layout, or one browser's User Data with -b)")
	cmd.Flags().StringVar(&dataZip, "data-zip", "", "zip produced by the archive command (alternative to --data-dir)")
	cmd.Flags().StringVarP(&browserName, "browser", "b", "", "restore only this browser (optional; must match a vault in --keys)")
	cmd.Flags().StringVarP(&category, "category", "c", "all", "data categories (comma-separated): all|"+categoryNames())
	cmd.Flags().StringVarP(&outputFormat, "format", "f", "json", "output format: csv|json|cookie-editor")
	cmd.Flags().StringVarP(&outputDir, "dir", "d", "results", "output directory")
	cmd.Flags().BoolVar(&compress, "zip", false, "compress output to zip")

	_ = cmd.MarkFlagRequired("keys")
	cmd.MarkFlagsMutuallyExclusive("data-dir", "data-zip")

	return cmd
}

func loadRestoreBrowsers(keysPath, dataDir, browserName string) ([]browser.Browser, error) {
	if keysPath == "" {
		return nil, fmt.Errorf("requires --keys <file> (or - for stdin)")
	}

	var r io.Reader = os.Stdin
	if keysPath != "-" {
		f, err := os.Open(keysPath)
		if err != nil {
			return nil, fmt.Errorf("open keys file %q: %w", keysPath, err)
		}
		defer f.Close()
		r = f
	}
	dump, err := masterkey.ReadJSON(r)
	if err != nil {
		return nil, fmt.Errorf("read keys file %q: %w", keysPath, err)
	}

	return browser.BuildFromDump(dump, dataDir, browserName)
}

// openRestoreData returns the snapshot directory restore reads from: --data-dir in place, or --data-zip extracted
// into a temporary one. Exactly one of the two must be set.
func openRestoreData(dataDir, dataZip string) (*snapshot.Dir, error) {
	if (dataDir == "") == (dataZip == "") {
		return nil, fmt.Errorf("exactly one of --data-dir or --data-zip is required")
	}
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
