package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/moond4rk/hackbrowserdata/browser"
	"github.com/moond4rk/hackbrowserdata/log"
	"github.com/moond4rk/hackbrowserdata/output"
	"github.com/moond4rk/hackbrowserdata/types"
	"github.com/moond4rk/hackbrowserdata/utils/fileutil"
)

func dumpCmd() *cobra.Command {
	var (
		browserName  string
		category     string
		outputFormat string
		outputDir    string
		profilePath  string
		keychainPw   string
		compress     bool
	)

	cmd := &cobra.Command{
		Use:   "dump",
		Short: "Extract and decrypt browser data (default command)",
		Example: `  hack-browser-data dump
  hack-browser-data dump -b chrome -c password,cookie
  hack-browser-data dump -b chrome -f json -d output
  hack-browser-data dump -f cookie-editor
  hack-browser-data dump --zip`,
		RunE: func(cmd *cobra.Command, args []string) error {
			browsers, err := browser.PickBrowsers(browser.PickOptions{
				Name:             browserName,
				ProfilePath:      profilePath,
				KeychainPassword: keychainPw,
			})
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

			w, err := output.NewWriter(outputDir, outputFormat)
			if err != nil {
				return err
			}

			for _, b := range browsers {
				data, extractErr := b.Extract(categories)
				if extractErr != nil {
					log.Errorf("extract %s/%s: %v", b.BrowserName(), b.ProfileName(), extractErr)
				}
				w.Add(b.BrowserName(), b.ProfileName(), data)
			}

			if err := w.Write(); err != nil {
				return err
			}

			if compress {
				if err := fileutil.CompressDir(outputDir); err != nil {
					return fmt.Errorf("compress: %w", err)
				}
				log.Warnf("compressed: %s/%s.zip", outputDir, filepath.Base(outputDir))
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&browserName, "browser", "b", "all", "target browser: all|"+browser.Names())
	cmd.Flags().StringVarP(&category, "category", "c", "all", "data categories (comma-separated): all|"+categoryNames())
	cmd.Flags().StringVarP(&outputFormat, "format", "f", "csv", "output format: csv|json|cookie-editor")
	cmd.Flags().StringVarP(&outputDir, "dir", "d", "results", "output directory")
	cmd.Flags().StringVarP(&profilePath, "profile-path", "p", "", "custom profile dir path, get with chrome://version")
	cmd.Flags().StringVar(&keychainPw, "keychain-pw", "", "macOS keychain password")
	cmd.Flags().BoolVar(&compress, "zip", false, "compress output to zip")

	return cmd
}

// parseCategories converts a comma-separated string into a Category slice.
// "all" returns all categories.
func parseCategories(s string) ([]types.Category, error) {
	s = strings.TrimSpace(s)
	if strings.EqualFold(s, "all") {
		return types.AllCategories, nil
	}

	categoryMap := make(map[string]types.Category, len(types.AllCategories))
	for _, c := range types.AllCategories {
		categoryMap[c.String()] = c
	}

	var categories []types.Category
	for _, name := range strings.Split(s, ",") {
		name = strings.TrimSpace(strings.ToLower(name))
		if name == "" {
			continue
		}
		c, ok := categoryMap[name]
		if !ok {
			return nil, fmt.Errorf("unknown category: %q, available: all|%s", name, categoryNames())
		}
		categories = append(categories, c)
	}
	if len(categories) == 0 {
		return nil, fmt.Errorf("no categories specified")
	}
	return categories, nil
}

func categoryNames() string {
	names := make([]string, len(types.AllCategories))
	for i, c := range types.AllCategories {
		names[i] = c.String()
	}
	return strings.Join(names, ",")
}
