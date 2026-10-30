package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/moond4rk/hackbrowserdata/internal/app"
	"github.com/moond4rk/hackbrowserdata/internal/log"
)

func dumpKeysCmd() *cobra.Command {
	var (
		f          app.Flags
		outputPath string
	)

	cmd := &cobra.Command{
		Use:   "dumpkeys",
		Short: "Export Chromium master keys as JSON for cross-host decryption",
		Example: `  hack-browser-data dumpkeys -o keys.json
  hack-browser-data dumpkeys -b chrome`,
		RunE: func(cmd *cobra.Command, args []string) error {
			req, err := app.ValidateKeys(f)
			if err != nil {
				return err
			}
			dump, err := app.DumpKeys(cmd.Context(), app.Discover(req), req)
			if err != nil {
				return err
			}
			log.Infof("Exported keys for %d vault(s)", len(dump.Vaults))

			if outputPath == "" {
				return dump.WriteJSON(os.Stdout)
			}
			file, err := os.OpenFile(outputPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
			if err != nil {
				return fmt.Errorf("create %s: %w", outputPath, err)
			}
			return errors.Join(dump.WriteJSON(file), file.Close())
		},
	}

	cmd.Flags().StringVarP(&f.Browser, "browser", "b", "all", "target browser: all|"+browserNames())
	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "output file (default: stdout)")
	keychainPasswordFlags(cmd, &f)

	return cmd
}
