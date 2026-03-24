//go:build !windows

package filemanager

import "fmt"

// copyLocked is a no-op on non-Windows platforms.
// File locking is primarily a Windows issue where Chrome holds exclusive
// locks on Cookie files via SQLite WAL mode.
func copyLocked(_, _ string) error {
	return fmt.Errorf("locked file copy not supported on this platform")
}
