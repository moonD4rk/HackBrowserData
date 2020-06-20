package utils

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
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

func GetDBPath(dbName string) (string, error) {
	s, err := filepath.Glob(macChromeDir + dbName)
	if err != nil && len(s) == 0 {
		return "", err
	}
	return s[0], nil
}

func InitChromeKey() {
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
		panic(err)
	}
	if stderr.Len() > 0 {
		panic(stderr.String())
	}
	// replace /n
	temp := stdout.Bytes()
	chromePass = temp[:len(temp)-1]
	DecryptPass(chromePass)
}

func DecryptPass(chromePass []byte) {
	chromeKey = pbkdf2.Key(chromePass, chromeSalt, 1003, 16, sha1.New)
}

func Aes128CBCDecrypt(encryptPass []byte) (string, error) {
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
