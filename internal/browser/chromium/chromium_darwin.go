//go:build darwin

package chromium

import (
	"bytes"
	"crypto/sha1"
	"errors"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/crypto/pbkdf2"

	"hack-browser-data/internal/item"
	"hack-browser-data/internal/log"
)

var (
	errWrongSecurityCommand   = errors.New("wrong security command")
	errCouldNotFindInKeychain = errors.New("could not be find in keychain")
)

func (c *chromium) GetMasterKey() ([]byte, error) {
	var (
		cmd            *exec.Cmd
		stdout, stderr bytes.Buffer
	)
	// don't need chromium key file for macOS
	defer os.Remove(item.TempChromiumKey)
	// Get the master key from the keychain
	// $ security find-generic-password -wa 'Chrome'
	cmd = exec.Command("security", "find-generic-password", "-wa", strings.TrimSpace(c.storage)) //nolint:gosec
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return nil, err
	}
	if stderr.Len() > 0 {
		if strings.Contains(stderr.String(), "could not be found") {
			return nil, errCouldNotFindInKeychain
		}
		return nil, errors.New(stderr.String())
	}
	chromeSecret := bytes.TrimSpace(stdout.Bytes())
	if chromeSecret == nil {
		return nil, errWrongSecurityCommand
	}
	chromeSalt := []byte("saltysalt")
	// @https://source.chromium.org/chromium/chromium/src/+/master:components/os_crypt/os_crypt_mac.mm;l=157
	key := pbkdf2.Key(chromeSecret, chromeSalt, 1003, 16, sha1.New)
	if key == nil {
		return nil, errWrongSecurityCommand
	}
	c.masterKey = key
	log.Infof("%s initialized master key success", c.name)
	return key, nil
}
