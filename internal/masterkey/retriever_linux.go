//go:build linux

package masterkey

import (
	"context"
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

// A profile holds both v10 and v11 values when its host moved between headless and keyring sessions.
var platformRetrievers = []Retriever{posixRetriever{}, dbusRetriever{}}

type dbusRetriever struct{}

func (dbusRetriever) Version() Version { return V11 }

func (dbusRetriever) Retrieve(_ context.Context, loc Locator) ([]byte, error) {
	storage := loc.KeychainLabel
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

// posixRetriever covers hosts without a keyring (headless, Docker, CI), mirroring Chromium's PosixKeyProvider.
type posixRetriever struct{}

func (posixRetriever) Version() Version { return V10 }

func (posixRetriever) Retrieve(context.Context, Locator) ([]byte, error) {
	return linuxParams.deriveKey([]byte("peanuts")), nil
}
