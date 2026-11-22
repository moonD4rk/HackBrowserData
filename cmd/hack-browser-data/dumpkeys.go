package main

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/moond4rk/hackbrowserdata/internal/app"
	"github.com/moond4rk/hackbrowserdata/internal/log"
)

func newDumpKeysCommand() *cobra.Command {
	var (
		flags    app.DumpKeysFlags
		keychain passwordFlags
	)

	command := &cobra.Command{
		Use:   "dumpkeys",
		Short: "Export Chromium master keys as JSON for cross-host decryption",
		Example: `  hack-browser-data dumpkeys -o keys.json
  hack-browser-data dumpkeys -b chrome`,
		RunE: func(command *cobra.Command, args []string) error {
			options, err := app.ValidateDumpKeys(flags, keychainPasswordFunc(keychain, os.Stdin))
			if err != nil {
				return err
			}
			vaultCount, err := app.DumpKeys(command.Context(), options, command.OutOrStdout())
			if err != nil {
				return err
			}
			log.Infof("Exported keys for %d vault(s)", vaultCount)
			return nil
		},
	}

	command.Flags().StringVarP(&flags.Browser, "browser", "b", "all", "target browser: all|"+browserNames())
	command.Flags().StringVarP(&flags.OutputPath, "output", "o", "", "output file (default: stdout)")
	addPasswordFlags(command, &keychain, "keychain-pw", "macOS keychain password")

	return command
}
