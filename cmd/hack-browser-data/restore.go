package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/moond4rk/hackbrowserdata/browser"
	"github.com/moond4rk/hackbrowserdata/log"
	"github.com/moond4rk/hackbrowserdata/masterkey"
)

func restoreCmd() *cobra.Command {
	var (
		keysPath     string
		browserName  string
		category     string
		outputFormat string
		outputDir    string
		profilePath  string
		compress     bool
	)

	cmd := &cobra.Command{
		Use:   "restore",
		Short: "Decrypt a copied profile using exported master keys",
		Example: `  hack-browser-data restore -i keys.json -b chrome -p /path/to/copied/User\ Data
  hack-browser-data restore -i keys.json -b edge -p /path -c cookie -f csv
  ssh origin "hack-browser-data dumpkeys" | hack-browser-data restore -i - -b chrome -p /path`,
		RunE: func(cmd *cobra.Command, args []string) error {
			browsers, err := loadAndApplyKeys(browserName, profilePath, keysPath)
			if err != nil {
				return err
			}
			if len(browsers) == 0 {
				log.Warnf("no browsers found")
				return nil
			}
			categories, err := parseCategories(category)
			if err != nil {
				return err
			}
			return extractAndWrite(browsers, categories, outputDir, outputFormat, compress)
		},
	}

	cmd.Flags().StringVarP(&keysPath, "input", "i", "", "input keys file (use - for stdin)")
	cmd.Flags().StringVarP(&browserName, "browser", "b", "", "target browser (single, required): "+browser.Names())
	cmd.Flags().StringVarP(&category, "category", "c", "all", "data categories (comma-separated): all|"+categoryNames())
	cmd.Flags().StringVarP(&outputFormat, "format", "f", "json", "output format: csv|json|cookie-editor")
	cmd.Flags().StringVarP(&outputDir, "dir", "d", "results", "output directory")
	cmd.Flags().StringVarP(&profilePath, "profile-path", "p", "", "copied profile dir path (required)")
	cmd.Flags().BoolVar(&compress, "zip", false, "compress output to zip")

	_ = cmd.MarkFlagRequired("input")
	_ = cmd.MarkFlagRequired("browser")
	_ = cmd.MarkFlagRequired("profile-path")

	return cmd
}

func loadAndApplyKeys(browserName, profilePath, keysPath string) ([]browser.Browser, error) {
	if profilePath == "" {
		return nil, fmt.Errorf("requires -p <copied-profile-dir>")
	}
	name := strings.ToLower(browserName)
	if name == "" || name == "all" {
		return nil, fmt.Errorf(`requires -b <browser> (single, not "all")`)
	}
	if keysPath == "" {
		return nil, fmt.Errorf("requires -i <keys-file> (or - for stdin)")
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

	browsers, err := browser.DiscoverBrowsers(browser.DiscoverOptions{
		Name:        browserName,
		ProfilePath: profilePath,
	})
	if err != nil {
		return nil, err
	}

	browser.ApplyDump(browsers, dump)

	for _, b := range browsers {
		if _, ok := b.(browser.KeychainPasswordReceiver); ok {
			log.Infof("Safari has no portable master key; run `dump -b safari` separately for full extraction")
			break
		}
	}

	return browsers, nil
}
