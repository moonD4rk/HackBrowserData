//go:build linux

package keyretriever

import (
	"crypto/sha1"
	"fmt"

	"github.com/godbus/dbus/v5"
	keyring "github.com/ppacher/go-dbus-keyring"
)

// https://source.chromium.org/chromium/chromium/src/+/main:components/os_crypt/os_crypt_linux.cc
var linuxParams = pbkdf2Params{
	salt:       []byte("saltysalt"),
	iterations: 1,
	keySize:    16,
	hashFunc:   sha1.New,
}

// DBusRetriever queries GNOME Keyring / KDE Wallet via D-Bus Secret Service.
type DBusRetriever struct{}

func (r *DBusRetriever) RetrieveKey(storage, _ string) ([]byte, error) {
	conn, err := dbus.SessionBus()
	if err != nil {
		return nil, fmt.Errorf("dbus session: %w", err)
	}

	svc, err := keyring.GetSecretService(conn)
	if err != nil {
		return nil, fmt.Errorf("secret service: %w", err)
	}

	session, err := svc.OpenSession()
	if err != nil {
		return nil, fmt.Errorf("open session: %w", err)
	}
	defer session.Close()

	collections, err := svc.GetAllCollections()
	if err != nil {
		return nil, fmt.Errorf("get collections: %w", err)
	}

	for _, col := range collections {
		items, err := col.GetAllItems()
		if err != nil {
			continue
		}
		for _, item := range items {
			label, err := item.GetLabel()
			if err != nil {
				continue
			}
			if label == storage {
				secret, err := item.GetSecret(session.Path())
				if err != nil {
					return nil, fmt.Errorf("get secret for %s: %w", storage, err)
				}
				if len(secret.Value) > 0 {
					return linuxParams.deriveKey(secret.Value), nil
				}
			}
		}
	}

	return nil, fmt.Errorf("%q: %w", storage, errStorageNotFound)
}

// PosixRetriever produces Chromium's kV10Key by applying PBKDF2 to the hardcoded password
// "peanuts". Matches Chromium's upstream PosixKeyProvider (components/os_crypt/async/browser/
// posix_key_provider.cc): a deterministic 16-byte AES-128 key used to encrypt ciphertexts with
// the "v10" prefix when no keyring is available (headless servers, Docker, CI).
type PosixRetriever struct{}

func (r *PosixRetriever) RetrieveKey(_, _ string) ([]byte, error) {
	return linuxParams.deriveKey([]byte("peanuts")), nil
}

// DefaultRetrievers returns the Linux Retrievers, one per cipher tier. Chromium on Linux emits
// distinct prefixes for distinct key sources:
//
//   - v10 prefix → PBKDF2("peanuts") — Chromium's kV10Key, emitted when no keyring is available
//     (headless servers, Docker, CI).
//   - v11 prefix → PBKDF2(keyring secret) — Chromium's kV11Key, emitted when D-Bus Secret
//     Service (GNOME Keyring / KWallet) is reachable.
//
// A profile can carry both prefixes if the host moved between keyring-equipped and headless
// sessions, so both tiers run independently with per-tier logging rather than a first-success
// chain.
func DefaultRetrievers() Retrievers {
	return Retrievers{
		V10: &PosixRetriever{},
		V11: &DBusRetriever{},
	}
}
