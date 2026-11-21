package main

import (
	"github.com/spf13/cobra"

	"github.com/moond4rk/hackbrowserdata/internal/app"
)

// addOutputFlags defines the output flags once, so dump and restore keep one wording and one set of defaults.
func addOutputFlags(command *cobra.Command, flags *app.OutputFlags) {
	command.Flags().StringVarP(&flags.Format, "format", "f", "json", "output format: csv|json|cookie-editor")
	command.Flags().StringVarP(&flags.Dir, "dir", "d", "results", "output directory")
	command.Flags().BoolVar(&flags.Zip, "zip", false, "compress output to zip")
	command.Flags().IntVar(&flags.MaxStorageValue, "max-storage-value", app.DefaultMaxStorageValue, "max storage value chars, 0=unlimited")
}
