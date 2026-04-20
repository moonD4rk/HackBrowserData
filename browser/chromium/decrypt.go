package chromium

import (
	"fmt"

	"github.com/moond4rk/hackbrowserdata/crypto"
	"github.com/moond4rk/hackbrowserdata/crypto/keyretriever"
)

// decryptValue decrypts a Chromium-encrypted value by dispatching on the ciphertext's version
// prefix to the matching tier in keys:
//
//   - v10 → keys.V10 (Windows DPAPI / macOS Keychain / Linux peanuts kV10Key)
//   - v11 → keys.V11 (Linux keyring kV11Key; nil on Windows/macOS — Chromium doesn't emit v11 there)
//   - v20 → keys.V20 (Windows ABE; nil on non-Windows — Chromium doesn't emit v20 there)
//
// A single profile can carry mixed prefixes (Chrome 127+ upgrades on Windows; Linux session-mode
// changes), so every applicable key must be populated upstream for lossless extraction. Missing
// tier keys surface as decrypt errors at the ciphertext level; the extract layer treats those as
// empty plaintexts rather than fatal errors.
func decryptValue(keys keyretriever.MasterKeys, ciphertext []byte) ([]byte, error) {
	if len(ciphertext) == 0 {
		return nil, nil
	}

	version := crypto.DetectVersion(ciphertext)
	switch version {
	case crypto.CipherV10:
		return crypto.DecryptChromium(keys.V10, ciphertext)
	case crypto.CipherV11:
		// v11 is Linux-only and shares v10's AES-CBC path, but uses the keyring-derived kV11Key
		// rather than the peanuts-derived kV10Key — so a Linux profile with both prefixes needs
		// distinct per-tier keys to decrypt everything.
		return crypto.DecryptChromium(keys.V11, ciphertext)
	case crypto.CipherV20:
		// v20 is cross-platform AES-GCM; routed through a dedicated function so Linux/macOS CI can
		// exercise the same decryption path as Windows.
		return crypto.DecryptChromiumV20(keys.V20, ciphertext)
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
