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

// FallbackRetriever uses the hardcoded "peanuts" password when D-Bus is unavailable.
// https://source.chromium.org/chromium/chromium/src/+/main:components/os_crypt/os_crypt_linux.cc;l=100
type FallbackRetriever struct{}

func (r *FallbackRetriever) RetrieveKey(_, _ string) ([]byte, error) {
	return linuxParams.deriveKey([]byte("peanuts")), nil
}

// DefaultRetriever returns the Linux retriever chain:
// D-Bus Secret Service first, then "peanuts" fallback.
func DefaultRetriever() KeyRetriever {
	return NewChain(
		&DBusRetriever{},
		&FallbackRetriever{},
	)
}
