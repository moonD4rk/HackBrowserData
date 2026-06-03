package chromium

import (
	"fmt"

	"github.com/moond4rk/hackbrowserdata/crypto"
	"github.com/moond4rk/hackbrowserdata/masterkey"
)

// decryptValue decrypts a Chromium-encrypted value by dispatching on the ciphertext's version
// prefix to the matching tier in masterKeys:
//
//   - v10 → masterKeys.V10 (Windows DPAPI / macOS Keychain / Linux peanuts kV10Key)
//   - v11 → masterKeys.V11 (Linux keyring kV11Key; nil on Windows/macOS — Chromium doesn't emit v11 there)
//   - v20 → masterKeys.V20 (Windows ABE; nil on non-Windows — Chromium doesn't emit v20 there)
//
// A single profile can carry mixed prefixes (Chrome 127+ upgrades on Windows; Linux session-mode
// changes), so every applicable key must be populated upstream for lossless extraction. Missing
// tier keys surface as decrypt errors at the ciphertext level; the extract layer treats those as
// empty plaintexts rather than fatal errors.
func decryptValue(masterKeys masterkey.MasterKeys, ciphertext []byte) ([]byte, error) {
	if len(ciphertext) == 0 {
		return nil, nil
	}

	version := crypto.DetectVersion(ciphertext)
	switch version {
	case crypto.CipherV10:
		// v10's cipher depends on the platform that sealed it: a 32-byte AES-256 key means GCM
		// (Windows), a 16-byte AES-128 key means CBC (macOS/Linux). Dispatching on key length keeps
		// cross-host decryption OS-independent: a 32-byte key dumped on Windows decrypts here on macOS.
		if len(masterKeys.V10) == 32 {
			return crypto.DecryptChromiumGCM(masterKeys.V10, ciphertext)
		}
		return crypto.DecryptChromiumCBC(masterKeys.V10, ciphertext)
	case crypto.CipherV11:
		// v11 is Linux-only AES-CBC; same algorithm as Linux v10 but the key comes from the keyring
		// (kV11Key) rather than peanuts (kV10Key), so both tiers need distinct keys.
		return crypto.DecryptChromiumCBC(masterKeys.V11, ciphertext)
	case crypto.CipherV20:
		// v20 is cross-platform AES-GCM (Chrome 127+ ABE); same wire layout as Windows v10.
		return crypto.DecryptChromiumGCM(masterKeys.V20, ciphertext)
	case crypto.CipherV12:
		// Chromium's SecretPortalKeyProvider (Flatpak / xdg-desktop-portal) — HKDF-SHA256 +
		// AES-256-GCM with a secret retrieved via org.freedesktop.portal.Desktop. Recognized here
		// to surface an actionable "known gap" error rather than the generic "unsupported" one.
		return nil, fmt.Errorf("unsupported cipher version v12 (Chromium SecretPortal / Flatpak; not yet implemented)")
	case crypto.CipherDPAPI:
		return crypto.DecryptDPAPI(ciphertext)
	default:
		return nil, fmt.Errorf("unsupported cipher version: %s", version)
	}
}
