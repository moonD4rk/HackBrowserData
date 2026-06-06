package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/moond4rk/hackbrowserdata/browser"
	"github.com/moond4rk/hackbrowserdata/log"
)

func dumpKeysCmd() *cobra.Command {
	var (
		browserName string
		outputPath  string
		keychainPw  string
	)

	cmd := &cobra.Command{
		Use:   "dumpkeys",
		Short: "Export Chromium master keys as JSON for cross-host decryption",
		Example: `  hack-browser-data dumpkeys -o keys.json
  hack-browser-data dumpkeys -b chrome`,
		RunE: func(cmd *cobra.Command, args []string) error {
			browsers, err := browser.DiscoverBrowsersWithKeys(browser.DiscoverOptions{
				Name:             browserName,
				KeychainPassword: keychainPw,
			})
			if err != nil {
				return err
			}

			dump := browser.BuildDump(browsers)
			log.Infof("Exported keys for %d vault(s)", len(dump.Vaults))

			if outputPath == "" {
				return dump.WriteJSON(os.Stdout)
			}
			f, err := os.OpenFile(outputPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
			if err != nil {
				return fmt.Errorf("create %s: %w", outputPath, err)
			}
			defer f.Close()
			return dump.WriteJSON(f)
		},
	}

	cmd.Flags().StringVarP(&browserName, "browser", "b", "all", "target browser: all|"+browser.Names())
	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "output file (default: stdout)")
	cmd.Flags().StringVar(&keychainPw, "keychain-pw", "", "macOS keychain password")

	return cmd
}
