//go:build linux

package chromium

import (
	"crypto/sha1"
	"fmt"
	"os"

	"github.com/godbus/dbus/v5"
	keyring "github.com/ppacher/go-dbus-keyring"

	"github.com/moond4rk/hackbrowserdata/crypto"
	"github.com/moond4rk/hackbrowserdata/log"
	"github.com/moond4rk/hackbrowserdata/types"
)

func (c *Chromium) GetMasterKey() ([]byte, error) {
	// what is d-bus @https://dbus.freedesktop.org/
	// don't need chromium key file for Linux
	defer os.Remove(types.ChromiumKey.TempFilename())

	conn, err := dbus.SessionBus()
	if err != nil {
		return nil, err
	}
	svc, err := keyring.GetSecretService(conn)
	if err != nil {
		return nil, err
	}
	session, err := svc.OpenSession()
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := session.Close(); err != nil {
			log.Errorf("close dbus session error: %v", err)
		}
	}()
	collections, err := svc.GetAllCollections()
	if err != nil {
		return nil, err
	}
	var secret []byte
	for _, col := range collections {
		items, err := col.GetAllItems()
		if err != nil {
			return nil, err
		}
		for _, i := range items {
			label, err := i.GetLabel()
			if err != nil {
				log.Warnf("get label from dbus: %v", err)
				continue
			}
			if label == c.storage {
				se, err := i.GetSecret(session.Path())
				if err != nil {
					return nil, fmt.Errorf("get storage from dbus: %w", err)
				}
				secret = se.Value
			}
		}
	}

	if len(secret) == 0 {
		// set default secret @https://source.chromium.org/chromium/chromium/src/+/main:components/os_crypt/os_crypt_linux.cc;l=100
		secret = []byte("peanuts")
	}
	salt := []byte("saltysalt")
	// @https://source.chromium.org/chromium/chromium/src/+/master:components/os_crypt/os_crypt_linux.cc
	key := crypto.PBKDF2Key(secret, salt, 1, 16, sha1.New)
	c.masterKey = key
	log.Debugf("get master key success, browser %s", c.name)
	return key, nil
}
