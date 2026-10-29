//go:build windows

package main

import (
	"os"

	"github.com/inconshreveable/mousetrap"
	"github.com/spf13/cobra"
	"golang.org/x/sys/windows"

	"github.com/moond4rk/hackbrowserdata/internal/winapi"
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

// exitBy ends the process with the status Windows gives a console process that Ctrl-C or closing the console ends.
func exitBy(os.Signal) {
	status := uint32(windows.STATUS_CONTROL_C_EXIT)
	os.Exit(int(int32(status)))
}
