//go:build linux

package masterkey

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

func (r *DBusRetriever) RetrieveKey(hints Hints) ([]byte, error) {
	storage := hints.KeychainLabel
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

// PosixRetriever derives Chromium's kV10Key via PBKDF2 over the hardcoded "peanuts" password — the
// deterministic v10 key used when no keyring exists (headless/Docker/CI). Mirrors PosixKeyProvider.
type PosixRetriever struct{}

func (r *PosixRetriever) RetrieveKey(_ Hints) ([]byte, error) {
	return linuxParams.deriveKey([]byte("peanuts")), nil
}

// DefaultRetrievers wires the Linux tiers, one per prefix Chromium emits: v10 = PBKDF2("peanuts")
// (kV10Key, no keyring); v11 = PBKDF2(keyring secret) (kV11Key, via D-Bus). A profile can carry both
// if the host moved between headless and keyring sessions, so both run independently.
func DefaultRetrievers() Retrievers {
	return Retrievers{
		V10: &PosixRetriever{},
		V11: &DBusRetriever{},
	}
}
