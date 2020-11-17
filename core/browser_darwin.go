package core

import (
	"bytes"
	"crypto/sha1"
	"errors"
	"os/exec"

	"golang.org/x/crypto/pbkdf2"
)

const (
	chromeProfilePath     = "/Users/*/Library/Application Support/Google/Chrome/*/"
	chromeBetaProfilePath = "/Users/*/Library/Application Support/Google/Chrome Beta/*/"
	edgeProfilePath       = "/Users/*/Library/Application Support/Microsoft Edge/*/"
	fireFoxProfilePath    = "/Users/*/Library/Application Support/Firefox/Profiles/*.default-release/"
	braveProfilePath      = "/Users/*/Library/Application Support/BraveSoftware/Brave-Browser/*/"
)

const (
	chromeStorageName     = "Chrome"
	chromeBetaStorageName = "Chrome"
	edgeStorageName       = "Microsoft Edge"
	braveStorageName      = "Brave"
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
	var (
		cmd            *exec.Cmd
		stdout, stderr bytes.Buffer
	)
	// ➜ security find-generic-password -wa 'Chrome'
	cmd = exec.Command("security", "find-generic-password", "-wa", c.storage)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return err
	}
	if stderr.Len() > 0 {
		err = errors.New(stderr.String())
		return err
	}
	temp := stdout.Bytes()
	chromeSecret := temp[:len(temp)-1]
	if chromeSecret == nil {
		return errChromeSecretIsEmpty
	}
	var chromeSalt = []byte("saltysalt")
	// @https://source.chromium.org/chromium/chromium/src/+/master:components/os_crypt/os_crypt_mac.mm;l=157
	key := pbkdf2.Key(chromeSecret, chromeSalt, 1003, 16, sha1.New)
	c.secretKey = key
	return nil
}
