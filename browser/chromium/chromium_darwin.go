//go:build darwin

package chromium

import (
	"bytes"
	"crypto/sha1"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/moond4rk/hackbrowserdata/crypto"
	"github.com/moond4rk/hackbrowserdata/log"
	"github.com/moond4rk/hackbrowserdata/types"
)

var (
	errWrongSecurityCommand   = errors.New("wrong security command")
	errCouldNotFindInKeychain = errors.New("could not be find in keychain")
)

func (c *Chromium) GetMasterKey() ([]byte, error) {
	// don't need chromium key file for macOS
	defer os.Remove(types.ChromiumKey.TempFilename())
	// Get the master key from the keychain
	// $ security find-generic-password -wa 'Chrome'
	var (
		stdout, stderr bytes.Buffer
	)
	cmd := exec.Command("security", "find-generic-password", "-wa", strings.TrimSpace(c.storage)) //nolint:gosec
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("run security command failed: %w, message %s", err, stderr.String())
	}

	if stderr.Len() > 0 {
		if strings.Contains(stderr.String(), "could not be found") {
			return nil, errCouldNotFindInKeychain
		}
		return nil, errors.New(stderr.String())
	}

	secret := bytes.TrimSpace(stdout.Bytes())
	if len(secret) == 0 {
		return nil, errWrongSecurityCommand
	}
	salt := []byte("saltysalt")
	// @https://source.chromium.org/chromium/chromium/src/+/master:components/os_crypt/os_crypt_mac.mm;l=157
	key := crypto.PBKDF2Key(secret, salt, 1003, 16, sha1.New)
	if key == nil {
		return nil, errWrongSecurityCommand
	}
	c.masterKey = key
	log.Debugf("get master key success, browser %s", c.name)
	return key, nil
}
