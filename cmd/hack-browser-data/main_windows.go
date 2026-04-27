//go:build windows

package main

import (
	"github.com/inconshreveable/mousetrap"
	"github.com/spf13/cobra"
	"golang.org/x/sys/windows"
)

const swHide = 0

var (
	kernel32             = windows.NewLazySystemDLL("kernel32.dll")
	user32               = windows.NewLazySystemDLL("user32.dll")
	procGetConsoleWindow = kernel32.NewProc("GetConsoleWindow")
	procShowWindow       = user32.NewProc("ShowWindow")
)

// configureDoubleClickMode hides the console and bypasses cobra's
// double-click guard when launched from Explorer (issue #344).
func configureDoubleClickMode() {
	if !mousetrap.StartedByExplorer() {
		return
	}

	cobra.MousetrapHelpText = ""

	hwnd, _, _ := procGetConsoleWindow.Call()
	if hwnd != 0 {
		_, _, _ = procShowWindow.Call(hwnd, swHide)
	}
}
