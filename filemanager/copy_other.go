//go:build !windows

package filemanager

import "fmt"

// copyLocked is not supported on non-Windows platforms and always returns an error.
// File locking is primarily a Windows issue where Chrome holds exclusive
// locks on Cookie files via SQLite WAL mode.
func copyLocked(_, _ string) error {
	return fmt.Errorf("locked file copy not supported on this platform")
}
