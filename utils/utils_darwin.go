package utils

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"errors"
	"hack-browser-data/log"
	"os/exec"
	"strings"

	"golang.org/x/crypto/pbkdf2"
)

const (
	chromeProfilePath  = "/Users/*/Library/Application Support/Google/Chrome/*/"
	chromeCommand      = "Chrome"
	edgeProfilePath    = "/Users/*/Library/Application Support/Microsoft Edge/*/"
	edgeCommand        = "Microsoft Edge"
	fireFoxProfilePath = "/Users/*/Library/Application Support/Firefox/Profiles/*.default-release/"
	fireFoxCommand     = ""
)

var (
	iv          = []byte{32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32}
	command     = []string{"security", "find-generic-password", "-wa"}
	chromeSalt  = []byte("saltysalt")
	chromeKey   []byte
	browserList = map[string]struct {
		ProfilePath string
		Command     string
	}{
		"chrome": {
			chromeProfilePath,
			chromeCommand,
		},
		"edge": {
			edgeProfilePath,
			edgeCommand,
		},
		"firefox": {
			fireFoxProfilePath,
			fireFoxCommand,
		},
	}
)

func DecryptStringWithDPAPI(data []byte) (string, error) {
	return string(data), nil
}

func PickBrowser(name string) (browserDir, command string, err error) {
	name = strings.ToLower(name)
	if choice, ok := browserList[name]; ok {
		return choice.ProfilePath, choice.Command, err
	}
	return "", "", errBrowserNotSupported
}

func InitKey(key string) error {
	var (
		cmd            *exec.Cmd
		stdout, stderr bytes.Buffer
	)
	cmd = exec.Command(command[0], command[1], command[2], key)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		log.Println(err)
		return err
	}
	if stderr.Len() > 0 {
		err = errors.New(stderr.String())
		log.Println(err)
	}
	temp := stdout.Bytes()
	chromePass := temp[:len(temp)-1]
	decryptChromeKey(chromePass)
	return err
}

func decryptChromeKey(chromePass []byte) {
	chromeKey = pbkdf2.Key(chromePass, chromeSalt, 1003, 16, sha1.New)
}

func DecryptChromePass(encryptPass []byte) (string, error) {
	if len(encryptPass) > 3 {
		return aes128CBCDecrypt(encryptPass[3:])
	} else {
		return "", &DecryptError{
			err: passwordIsEmpty,
		}
	}
}

func aes128CBCDecrypt(encryptPass []byte) (string, error) {
	if len(chromeKey) == 0 {
		return "", nil
	}
	block, err := aes.NewCipher(chromeKey)
	if err != nil {
		return "", err
	}
	dst := make([]byte, len(encryptPass))
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(dst, encryptPass)
	dst = PKCS5UnPadding(dst)
	return string(dst), nil
}

func PKCS5UnPadding(src []byte) []byte {
	length := len(src)
	unpadding := int(src[length-1])
	return src[:(length - unpadding)]
}

func PKCS7UnPadding(origData []byte)[]byte{
	length := len(origData)
	unpadding := int(origData[length-1])
	return origData[:length-unpadding]
}
