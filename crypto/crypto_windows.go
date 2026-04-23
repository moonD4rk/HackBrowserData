//go:build windows

package crypto

import (
	"github.com/moond4rk/hackbrowserdata/utils/winapi"
)

// gcmNonceSize is defined in crypto.go (cross-platform).
const minGCMDataSize = versionPrefixLen + gcmNonceSize // "v10" + nonce = 15 bytes minimum

func DecryptChromium(key, ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < minGCMDataSize {
		return nil, errShortCiphertext
	}
	nonce := ciphertext[versionPrefixLen : versionPrefixLen+gcmNonceSize]
	payload := ciphertext[versionPrefixLen+gcmNonceSize:]
	return AESGCMDecrypt(key, nonce, payload)
}

// DecryptDPAPI decrypts a DPAPI-protected blob using the current user's
// master key. The actual Win32 call (and its DATA_BLOB / LocalFree dance)
// lives in utils/winapi so every package that needs a syscall handle
// shares a single declaration instead of re-opening Crypt32.dll per call.
func DecryptDPAPI(ciphertext []byte) ([]byte, error) {
	return winapi.DecryptDPAPI(ciphertext)
}
