package chromium

import (
	"bytes"
	"crypto/aes"
	"crypto/sha1"
	"errors"

	"github.com/moond4rk/hackbrowserdata/internal/crypto"
)

// cipherVersion represents the encryption version used by Chromium browsers.
type cipherVersion string

const (
	// cipherV10 is Chrome 80+ encryption (AES-GCM on Windows, AES-CBC on macOS/Linux).
	cipherV10 cipherVersion = "v10"

	// cipherV11 is the Linux-only AES-CBC variant where the key comes from
	// libsecret / kwallet. Same algorithm as cipherV10; only the key source differs.
	cipherV11 cipherVersion = "v11"

	// cipherV20 is Chrome 127+ App-Bound Encryption.
	cipherV20 cipherVersion = "v20"

	// cipherV12 is Chromium's SecretPortalKeyProvider (Flatpak / xdg-desktop-portal) tier —
	// HKDF-SHA256 + AES-256-GCM with a secret retrieved via org.freedesktop.portal.Desktop.
	// Recognized by detectVersion so decryptValue can emit a known-gap error rather than a
	// generic "unsupported cipher version" message; not yet implemented.
	cipherV12 cipherVersion = "v12"

	// cipherDPAPI is pre-Chrome 80 raw DPAPI encryption (no version prefix).
	cipherDPAPI cipherVersion = "dpapi"

	versionPrefixLen = 3
)

// detectVersion identifies the encryption version from a ciphertext prefix.
func detectVersion(ciphertext []byte) cipherVersion {
	if len(ciphertext) < versionPrefixLen {
		return cipherDPAPI
	}
	prefix := string(ciphertext[:versionPrefixLen])
	switch prefix {
	case "v10":
		return cipherV10
	case "v11":
		return cipherV11
	case "v12":
		return cipherV12
	case "v20":
		return cipherV20
	default:
		return cipherDPAPI
	}
}

var errShortCiphertext = errors.New("ciphertext too short")

// gcmNonceSize is the AES-GCM standard nonce size used by Chromium's v10/v20
// cipher formats. Cross-platform because the v20 ciphertext layout is the
// same regardless of host OS (only Windows currently produces v20).
const gcmNonceSize = 12

// chromiumCBCIV is the fixed IV Chromium uses for AES-CBC v10/v11 (macOS/Linux).
var chromiumCBCIV = bytes.Repeat([]byte{0x20}, aes.BlockSize)

// kEmptyKey is Chromium's decrypt-only fallback for data corrupted by a KWallet
// race in Chrome ~89 (crbug.com/40055416). Matches kEmptyKey in os_crypt_linux.cc.
var kEmptyKey = crypto.PBKDF2Key([]byte(""), []byte("saltysalt"), 1, 16, sha1.New)

// decryptGCM decrypts a prefixed AES-GCM blob: version(3B)+nonce(12B)+ct+tag.
// Used by Windows v10 (AES-256) and v20; the layout is identical and platform-neutral.
func decryptGCM(key, ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < versionPrefixLen+gcmNonceSize {
		return nil, errShortCiphertext
	}
	nonce := ciphertext[versionPrefixLen : versionPrefixLen+gcmNonceSize]
	payload := ciphertext[versionPrefixLen+gcmNonceSize:]
	return crypto.AESGCMDecrypt(key, nonce, payload)
}

// decryptCBC decrypts a prefixed AES-CBC blob (version(3B)+ct) with Chromium's
// fixed IV, retrying with kEmptyKey to recover crbug.com/40055416 KWallet-corrupted data.
// Used by macOS/Linux v10 and Linux v11 (both AES-128).
func decryptCBC(key, ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < versionPrefixLen+aes.BlockSize {
		return nil, errShortCiphertext
	}
	payload := ciphertext[versionPrefixLen:]
	plaintext, err := crypto.AESCBCDecrypt(key, chromiumCBCIV, payload)
	if err == nil {
		return plaintext, nil
	}
	if alt, altErr := crypto.AESCBCDecrypt(kEmptyKey, chromiumCBCIV, payload); altErr == nil {
		return alt, nil
	}
	return nil, err
}
