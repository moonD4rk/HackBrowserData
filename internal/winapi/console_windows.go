//go:build windows

package winapi

var (
	procGetConsoleWindow = Kernel32.NewProc("GetConsoleWindow")
	procShowWindow       = User32.NewProc("ShowWindow")
)

const swHide = 0

// HideConsoleWindow hides the console window attached to the current
// process. Returns true if the window was previously visible.
func HideConsoleWindow() bool {
	hwnd, _, _ := procGetConsoleWindow.Call()
	if hwnd == 0 {
		return false
	}
	prev, _, _ := procShowWindow.Call(hwnd, swHide)
	return prev != 0
}
