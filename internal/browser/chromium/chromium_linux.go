package chromium

import (
	"crypto/sha1"

	"github.com/godbus/dbus/v5"
	keyring "github.com/ppacher/go-dbus-keyring"
	"golang.org/x/crypto/pbkdf2"

	"hack-browser-data/internal/log"
)

func (c *chromium) GetMasterKey() ([]byte, error) {
	// what is d-bus @https://dbus.freedesktop.org/
	var chromeSecret []byte
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
		session.Close()
	}()
	collections, err := svc.GetAllCollections()
	if err != nil {
		return nil, err
	}
	for _, col := range collections {
		items, err := col.GetAllItems()
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			label, err := item.GetLabel()
			if err != nil {
				log.Error(err)
				continue
			}
			if label == c.storage {
				se, err := item.GetSecret(session.Path())
				if err != nil {
					log.Error(err)
					return nil, err
				}
				chromeSecret = se.Value
			}
		}
	}
	// TODO: handle error if no secret found
	if chromeSecret == nil {
		return nil, err
	}
	var chromeSalt = []byte("saltysalt")
	// @https://source.chromium.org/chromium/chromium/src/+/master:components/os_crypt/os_crypt_linux.cc
	key := pbkdf2.Key(chromeSecret, chromeSalt, 1, 16, sha1.New)
	c.masterKey = key
	return key, nil
}
