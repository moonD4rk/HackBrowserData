package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/moond4rk/hackbrowserdata/internal/app"
	"github.com/moond4rk/hackbrowserdata/internal/log"
)

func newDumpKeysCommand() *cobra.Command {
	var (
		flags      app.Flags
		outputPath string
	)

	command := &cobra.Command{
		Use:   "dumpkeys",
		Short: "Export Chromium master keys as JSON for cross-host decryption",
		Example: `  hack-browser-data dumpkeys -o keys.json
  hack-browser-data dumpkeys -b chrome`,
		RunE: func(command *cobra.Command, args []string) error {
			options, err := app.ValidateDumpKeys(flags)
			if err != nil {
				return err
			}
			keyFile, err := app.DumpKeys(command.Context(), app.Discover(options), options)
			if err != nil {
				return err
			}
			log.Infof("Exported keys for %d vault(s)", len(keyFile.Vaults))

			if outputPath == "" {
				return keyFile.Write(os.Stdout)
			}
			file, err := os.OpenFile(outputPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
			if err != nil {
				return fmt.Errorf("create %s: %w", outputPath, err)
			}
			return errors.Join(keyFile.Write(file), file.Close())
		},
	}

	command.Flags().StringVarP(&flags.Browser, "browser", "b", "all", "target browser: all|"+browserNames())
	command.Flags().StringVarP(&outputPath, "output", "o", "", "output file (default: stdout)")
	addKeychainPasswordFlags(command, &flags)

	return command
}
