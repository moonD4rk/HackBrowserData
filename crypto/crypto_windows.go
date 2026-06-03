//go:build windows

package crypto

import (
	"github.com/moond4rk/hackbrowserdata/utils/winapi"
)

// DecryptDPAPI decrypts a DPAPI-protected blob using the current user's
// master key. The actual Win32 call (and its DATA_BLOB / LocalFree dance)
// lives in utils/winapi so every package that needs a syscall handle
// shares a single declaration instead of re-opening Crypt32.dll per call.
func DecryptDPAPI(ciphertext []byte) ([]byte, error) {
	return winapi.DecryptDPAPI(ciphertext)
}
