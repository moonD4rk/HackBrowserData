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

	// versionPrefixLen is the byte length of the version prefix ("v10", "v20").
	versionPrefixLen = 3
)

// DetectVersion identifies the encryption version from a ciphertext prefix.
func DetectVersion(ciphertext []byte) CipherVersion {
	if len(ciphertext) < versionPrefixLen {
		return CipherDPAPI
	}
	prefix := string(ciphertext[:versionPrefixLen])
	switch prefix {
	case "v10":
		return CipherV10
	case "v20":
		return CipherV20
	default:
		return CipherDPAPI
	}
}

// stripPrefix removes the version prefix (e.g. "v10") from ciphertext.
// Returns the ciphertext unchanged if no known prefix is found.
func stripPrefix(ciphertext []byte) []byte {
	ver := DetectVersion(ciphertext)
	if ver == CipherV10 || ver == CipherV20 {
		return ciphertext[versionPrefixLen:]
	}
	return ciphertext
}
