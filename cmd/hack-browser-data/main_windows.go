//go:build windows

package main

import (
	"github.com/inconshreveable/mousetrap"
	"github.com/spf13/cobra"

	"github.com/moond4rk/hackbrowserdata/utils/winapi"
)

// configureDoubleClickMode hides the console and bypasses cobra's
// double-click guard when launched from Explorer (issue #344).
func configureDoubleClickMode() {
	if !mousetrap.StartedByExplorer() {
		return
	}

	cobra.MousetrapHelpText = ""
	winapi.HideConsoleWindow()
}
