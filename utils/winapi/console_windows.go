//go:build windows

package winapi

var (
	procGetConsoleWindow = Kernel32.NewProc("GetConsoleWindow")
	procShowWindow       = User32.NewProc("ShowWindow")
)

const swHide = 0

// HideConsoleWindow hides the console window attached to the current
// process. Used when the binary is launched via Explorer double-click
// so no cmd window appears.
func HideConsoleWindow() {
	hwnd, _, _ := procGetConsoleWindow.Call()
	if hwnd == 0 {
		return
	}
	_, _, _ = procShowWindow.Call(hwnd, swHide)
}
