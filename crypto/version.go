package crypto

// CipherVersion represents the encryption version used by Chromium browsers.
type CipherVersion string

const (
	// CipherV10 is Chrome 80+ encryption (AES-GCM on Windows, AES-CBC on macOS/Linux).
	CipherV10 CipherVersion = "v10"

	// CipherV20 is Chrome 127+ App-Bound Encryption.
	CipherV20 CipherVersion = "v20"

	// CipherDPAPI is pre-Chrome 80 raw DPAPI encryption (no version prefix).
	CipherDPAPI CipherVersion = "dpapi"
)

// DetectVersion identifies the encryption version from a ciphertext prefix.
func DetectVersion(ciphertext []byte) CipherVersion {
	if len(ciphertext) < 3 {
		return CipherDPAPI
	}
	prefix := string(ciphertext[:3])
	switch prefix {
	case "v10":
		return CipherV10
	case "v20":
		return CipherV20
	default:
		return CipherDPAPI
	}
}

// StripPrefix removes the version prefix (e.g. "v10") from ciphertext.
// Returns the ciphertext unchanged if no known prefix is found.
func StripPrefix(ciphertext []byte) []byte {
	ver := DetectVersion(ciphertext)
	if ver == CipherV10 || ver == CipherV20 {
		return ciphertext[3:]
	}
	return ciphertext
}
