//go:build windows

package snapshot

import (
	"os"

	"github.com/moond4rk/hackbrowserdata/internal/winapi"
)

func copyLocked(from, into string) error {
	data, err := winapi.ReadLockedFile(from)
	if err != nil {
		return err
	}
	return os.WriteFile(into, data, 0o600)
}
