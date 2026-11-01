//go:build windows

package main

import (
	"os"

	"github.com/inconshreveable/mousetrap"
	"github.com/spf13/cobra"
	"golang.org/x/sys/windows"

	"github.com/moond4rk/hackbrowserdata/internal/winapi"
)

// configureDoubleClickMode lets a double-click from Explorer run the tool with its console hidden (issue #344).
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
