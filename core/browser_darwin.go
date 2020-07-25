package core

import (
	"bytes"
	"crypto/sha1"
	"errors"
	"hack-browser-data/log"
	"os/exec"

	"golang.org/x/crypto/pbkdf2"
)

const (
	chromeProfilePath  = "/Users/*/Library/Application Support/Google/Chrome/*/"
	edgeProfilePath    = "/Users/*/Library/Application Support/Microsoft Edge/*/"
	fireFoxProfilePath = "/Users/*/Library/Application Support/Firefox/Profiles/*.default-release/"
)

var (
	browserList = map[string]struct {
		ProfilePath string
		Name        string
		KeyPath     string
		New         func(profile, key, name string) (Browser, error)
	}{
		"chrome": {
			ProfilePath: chromeProfilePath,
			Name:        chromeName,
			New:         decryptChromium,
		},
		"edge": {
			ProfilePath: edgeProfilePath,
			Name:        edgeName,
			New:         decryptChromium,
		},
		"firefox": {
			ProfilePath: fireFoxProfilePath,
			Name:        firefoxName,
			New:         decryptFirefox,
		},
	}
)

func (c *chromium) InitSecretKey() error {
	var (
		cmd            *exec.Cmd
		stdout, stderr bytes.Buffer
	)
	//➜ security find-generic-password -wa 'Chrome'
	cmd = exec.Command("security", "find-generic-password", "-wa", c.Name)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		log.Error(err)
		return err
	}
	if stderr.Len() > 0 {
		err = errors.New(stderr.String())
		log.Error(err)
	}
	temp := stdout.Bytes()
	chromeSecret := temp[:len(temp)-1]
	if chromeSecret == nil {
		return ErrChromeSecretIsEmpty
	}
	var chromeSalt = []byte("saltysalt")
	// @https://source.chromium.org/chromium/chromium/src/+/master:components/os_crypt/os_crypt_mac.mm;l=157
	key := pbkdf2.Key(chromeSecret, chromeSalt, 1003, 16, sha1.New)
	c.SecretKey = key
	return err
}
