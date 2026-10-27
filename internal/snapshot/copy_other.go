//go:build !windows

package snapshot

import "errors"

// copyLocked exists for the build only: file locks outside Windows are advisory and never block a reader.
func copyLocked(_, _ string) error {
	return errors.New("locked file copy not supported on this platform")
}
