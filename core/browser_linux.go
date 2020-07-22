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
	fireFoxProfilePath = "/home/*/.mozilla/firefox/*.default-release/"
	fireFoxCommand     = ""
)

var (
	browserList = map[string]struct {
		ProfilePath string
		Name        string
		KeyPath     string
		New         func(profile, key, name string) (Browser, error)
	}{
		"firefox": {
			ProfilePath: fireFoxProfilePath,
			Name:        fireFoxCommand,
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
	chromePass := temp[:len(temp)-1]
	var chromeSalt = []byte("saltysalt")
	// @https://source.chromium.org/chromium/chromium/src/+/master:components/os_crypt/os_crypt_mac.mm;l=157
	key := pbkdf2.Key(chromePass, chromeSalt, 1003, 16, sha1.New)
	c.SecretKey = key
	return err
}
