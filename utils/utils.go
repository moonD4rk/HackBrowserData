package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/des"
	"encoding/asn1"
	"errors"
	"fmt"
	"hack-browser-data/log"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

var (
	errPasswordIsEmpty     = errors.New("decrypt failed, password is empty")
	errBrowserNotSupported = errors.New("browser not supported")
	errKeyIsEmpty          = errors.New("input [security find-generic-password -wa 'Chrome'] in terminal")
	VersionUnder80         bool
)

type DecryptError struct {
	err error
	msg string
}

func (e *DecryptError) Error() string {
	return fmt.Sprintf("%s: %s", e.msg, e.err)
}

func (e *DecryptError) Unwrap() error {
	return e.err
}

type Browser struct {
	Name    string
	DataDir string
}

const (
	LoginData        = "Login Data"
	History          = "History"
	Cookies          = "Cookies"
	Bookmarks        = "Bookmarks"
	FirefoxCookie    = "cookies.sqlite"
	FirefoxKey4DB    = "key4.db"
	FirefoxLoginData = "logins.json"
	FirefoxData      = "places.sqlite"
	FirefoxKey3DB    = "key3.db"
)

func InitKey(string) error {
	return nil
}

func ListBrowser() []string {
	var l []string
	for k := range browserList {
		l = append(l, k)
	}
	return l
}

func GetDBPath(dir string, dbName ...string) (dbFile []string) {
	for _, v := range dbName {
		s, err := filepath.Glob(dir + v)
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

func CopyDB(src, dst string) error {
	locals, _ := filepath.Glob("*")
	for _, v := range locals {
		if v == dst {
			err := os.Remove(dst)
			if err != nil {
				return err
			}
		}
	}
	sourceFile, err := ioutil.ReadFile(src)
	if err != nil {
		log.Debug(err.Error())
	}
	err = ioutil.WriteFile(dst, sourceFile, 0777)
	if err != nil {
		log.Debug(err.Error())
	}
	return err
}

func IntToBool(a int) bool {
	switch a {
	case 0, -1:
		return false
	}
	return true
}

func BookMarkType(a int64) string {
	switch a {
	case 1:
		return "url"
	default:
		return "folder"
	}
}

func TimeStampFormat(stamp int64) time.Time {
	s1 := time.Unix(stamp, 0)
	return s1
}

func TimeEpochFormat(epoch int64) time.Time {
	maxTime := int64(99633311740000000)
	if epoch > maxTime {
		epoch = maxTime
	}
	t := time.Date(1601, 1, 1, 0, 0, 0, 0, time.UTC)
	d := time.Duration(epoch)
	for i := 0; i < 1000; i++ {
		t = t.Add(d)
	}
	return t
}

// check time our range[1.9999]
func checkTimeRange(check time.Time) time.Time {
	end, _ := time.Parse(time.RFC3339, "9000-01-02T15:04:05Z07:00")
	if check.Before(end) {
		return check
	} else {
		return end
	}
}

func ReadFile(filename string) (string, error) {
	s, err := ioutil.ReadFile(filename)
	return string(s), err
}

func WriteFile(filename string, data []byte) error {
	err := ioutil.WriteFile(filename, data, 0644)
	if err != nil {
		return nil
	}
	return err
}

func FormatFileName(dir, browser, filename, format string) string {
	r := strings.TrimSpace(strings.ToLower(filename))
	r = strings.Replace(r, " ", "_", -1)
	p := path.Join(dir, fmt.Sprintf("%s_%s.%s", r, browser, format))
	return p
}

func MakeDir(dirName string) {
	if _, err := os.Stat(dirName); os.IsNotExist(err) {
		err = os.Mkdir(dirName, 0700)
	}
}

func PaddingZero(s []byte, l int) []byte {
	h := l - len(s)
	if h <= 0 {
		return s
	} else {
		for i := len(s); i < l; i++ {
			s = append(s, 0)
		}
		return s
	}
}

func PKCS5UnPadding(src []byte) []byte {
	length := len(src)
	unpadding := int(src[length-1])
	return src[:(length - unpadding)]
}

func Des3Decrypt(key, iv []byte, src []byte) ([]byte, error) {
	block, err := des.NewTripleDESCipher(key)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	blockMode := cipher.NewCBCDecrypter(block, iv)
	sq := make([]byte, len(src))
	blockMode.CryptBlocks(sq, src)
	return sq, nil
}

/*
SEQUENCE (3 elem)
	OCTET STRING (16 byte)
	SEQUENCE (2 elem)
		OBJECT IDENTIFIER 1.2.840.113549.3.7 des-EDE3-CBC (RSADSI encryptionAlgorithm)
		OCTET STRING (8 byte)
	OCTET STRING (16 byte)
*/
type LoginPBE struct {
	CipherText []byte
	SequenceLogin
	Encrypted []byte
}

type SequenceLogin struct {
	asn1.ObjectIdentifier
	Iv []byte
}

func DecodeLogin(decodeItem []byte) (pbe LoginPBE, err error) {
	_, err = asn1.Unmarshal(decodeItem, &pbe)
	if err != nil {
		log.Error(err)
		return
	}
	return pbe, nil
}

func aes128CBCDecrypt(key, iv, encryptPass []byte) ([]byte, error) {

	block, err := aes.NewCipher(key)
	if err != nil {
		return []byte{}, err
	}
	dst := make([]byte, len(encryptPass))
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(dst, encryptPass)
	dst = PKCS5UnPadding(dst)
	return dst, nil
}
