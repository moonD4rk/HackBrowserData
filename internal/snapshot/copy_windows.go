//go:build windows

package snapshot

import (
	"os"

	"github.com/moond4rk/hackbrowserdata/internal/winapi"
)

func copyLocked(src, dst string) error {
	data, err := winapi.ReadLockedFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o600)
}
