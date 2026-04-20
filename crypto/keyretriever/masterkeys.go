package keyretriever

import (
	"errors"
	"fmt"
)

// MasterKeys bundles the per-cipher-version Chromium master keys used to decrypt data from a
// single profile. decryptValue dispatches on the ciphertext's version prefix and picks the
// matching key; a missing (nil) key for a tier means "that cipher version cannot be decrypted",
// but the other tiers remain usable — a Chrome 127+ profile upgraded from pre-127 carries mixed
// v10+v20 ciphertexts, and Linux profiles may carry mixed v10+v11 for analogous reasons.
//
//   - V10: Chrome 80+ key with "v10" cipher prefix.
//   - Windows: os_crypt.encrypted_key decrypted by user-level DPAPI (AES-GCM ciphertexts).
//   - macOS:   derived from Keychain via PBKDF2(1003, SHA-1) (AES-CBC ciphertexts).
//   - Linux:   derived from "peanuts" hardcoded password (Chromium's kV10Key, AES-CBC).
//   - V11: Chrome Linux key with "v11" cipher prefix, derived from D-Bus Secret Service
//     (KWallet / GNOME Keyring) via PBKDF2. Nil on Windows and macOS (v11 prefix not used there).
//   - V20: Chrome 127+ Windows key with "v20" cipher prefix, retrieved via reflective injection
//     into the browser's elevation service. Nil on non-Windows platforms.
type MasterKeys struct {
	V10 []byte
	V11 []byte
	V20 []byte
}

// Retrievers is the per-tier retriever configuration passed to NewMasterKeys. Each slot runs
// independently — failure or absence of one tier does not affect others. Platform injectors set
// only the slots that apply to their platform and leave the rest nil (e.g. Linux populates
// V10+V11, leaves V20 nil).
type Retrievers struct {
	V10 KeyRetriever
	V11 KeyRetriever
	V20 KeyRetriever
}

// NewMasterKeys fetches every configured tier in r independently and returns the assembled
// MasterKeys together with any per-tier errors joined into one. Nil retrievers and retrievers
// returning (nil, nil) (the "not applicable" signal — e.g. ABERetriever on a non-ABE fork)
// contribute nil keys silently; only non-nil errors propagate.
//
// The returned error, when non-nil, is an errors.Join of per-tier failures formatted as
// "<tier>: <err>" (e.g. "v10: dpapi decrypt: ..."). Callers are expected to log it at whatever
// severity fits their context — this function itself never logs, leaving logging policy to its
// callers. Other pieces of the keyretriever package (e.g. ChainRetriever) may still log on their
// own failures; the "no-logging" guarantee is scoped to NewMasterKeys.
func NewMasterKeys(r Retrievers, storage, localStatePath string) (MasterKeys, error) {
	var keys MasterKeys
	var errs []error

	for _, t := range []struct {
		name string
		r    KeyRetriever
		dst  *[]byte
	}{
		{"v10", r.V10, &keys.V10},
		{"v11", r.V11, &keys.V11},
		{"v20", r.V20, &keys.V20},
	} {
		if t.r == nil {
			continue
		}
		k, err := t.r.RetrieveKey(storage, localStatePath)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", t.name, err))
			continue
		}
		*t.dst = k
	}
	return keys, errors.Join(errs...)
}
