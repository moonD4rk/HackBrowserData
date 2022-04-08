package chromium

import (
	"bytes"
	"crypto/sha1"
	"errors"
	"os/exec"

	"golang.org/x/crypto/pbkdf2"
)

var (
	ErrWrongSecurityCommand = errors.New("macOS wrong security command")
)

func (c *chromium) GetMasterKey() ([]byte, error) {
	var (
		cmd            *exec.Cmd
		stdout, stderr bytes.Buffer
	)
	// $ security find-generic-password -wa 'Chrome'
	cmd = exec.Command("security", "find-generic-password", "-wa", c.storage)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return nil, err
	}
	if stderr.Len() > 0 {
		return nil, errors.New(stderr.String())
	}
	temp := stdout.Bytes()
	chromeSecret := temp[:len(temp)-1]
	if chromeSecret == nil {
		return nil, ErrWrongSecurityCommand
	}
	var chromeSalt = []byte("saltysalt")
	// @https://source.chromium.org/chromium/chromium/src/+/master:components/os_crypt/os_crypt_mac.mm;l=157
	key := pbkdf2.Key(chromeSecret, chromeSalt, 1003, 16, sha1.New)
	if key != nil {
		c.browserInfo.masterKey = key
		return key, nil
	}
	return nil, errors.New("macOS wrong security command")
}
