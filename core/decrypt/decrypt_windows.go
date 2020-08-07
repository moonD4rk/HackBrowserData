package decrypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/asn1"
	"encoding/hex"
	"hack-browser-data/log"
	"syscall"
	"unsafe"

	"golang.org/x/crypto/pbkdf2"
)

func ChromePass(key, encryptPass []byte) ([]byte, error) {
	if len(encryptPass) > 15 {
		// remove Prefix 'v10'
		return aesGCMDecrypt(encryptPass[15:], key, encryptPass[3:15])
	} else {
		return nil, errPasswordIsEmpty
	}
}

// chromium > 80 https://source.chromium.org/chromium/chromium/src/+/master:components/os_crypt/os_crypt_win.cc
func aesGCMDecrypt(crypted, key, nounce []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	blockMode, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	origData, err := blockMode.Open(nil, nounce, crypted, nil)
	if err != nil {
		return nil, err
	}
	return origData, nil
}

type dataBlob struct {
	cbData uint32
	pbData *byte
}

func NewBlob(d []byte) *dataBlob {
	if len(d) == 0 {
		return &dataBlob{}
	}
	return &dataBlob{
		pbData: &d[0],
		cbData: uint32(len(d)),
	}
}

func (b *dataBlob) ToByteArray() []byte {
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
	var outBlob dataBlob
	r, _, err := procDecryptData.Call(uintptr(unsafe.Pointer(NewBlob(data))), 0, 0, 0, 0, 0, uintptr(unsafe.Pointer(&outBlob)))
	if r == 0 {
		return nil, err
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(outBlob.pbData)))
	return outBlob.ToByteArray(), nil
}

func DPApi(data []byte) ([]byte, error) {
	dllCrypt := syscall.NewLazyDLL("Crypt32.dll")
	dllKernel := syscall.NewLazyDLL("Kernel32.dll")
	procDecryptData := dllCrypt.NewProc("CryptUnprotectData")
	procLocalFree := dllKernel.NewProc("LocalFree")
	var outBlob dataBlob
	r, _, err := procDecryptData.Call(uintptr(unsafe.Pointer(NewBlob(data))), 0, 0, 0, 0, 0, uintptr(unsafe.Pointer(&outBlob)))
	if r == 0 {
		return nil, err
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(outBlob.pbData)))
	return outBlob.ToByteArray(), nil
}

type NssPBE struct {
	SequenceA
	Encrypted []byte
}

type MetaPBE struct {
	SequenceA
	Encrypted []byte
}
type SequenceA struct {
	PKCS5PBES2 asn1.ObjectIdentifier
	SequenceB
}
type SequenceB struct {
	SequenceC
	SequenceD
}

type SequenceC struct {
	PKCS5PBKDF2 asn1.ObjectIdentifier
	SequenceE
}

type SequenceD struct {
	AES256CBC asn1.ObjectIdentifier
	IV        []byte
}

type SequenceE struct {
	EntrySalt      []byte
	IterationCount int
	KeySize        int
	SequenceF
}

type SequenceF struct {
	HMACWithSHA256 asn1.ObjectIdentifier
}

func DecodeMeta(decodeItem []byte) (pbe MetaPBE, err error) {
	_, err = asn1.Unmarshal(decodeItem, &pbe)
	if err != nil {
		log.Error(err)
		return
	}
	return
}

func DecodeNss(nssA11Bytes []byte) (pbe NssPBE, err error) {
	log.Debug(hex.EncodeToString(nssA11Bytes))
	_, err = asn1.Unmarshal(nssA11Bytes, &pbe)
	if err != nil {
		log.Error(err)
		return
	}
	return
}

func Meta(globalSalt, masterPwd []byte, pbe MetaPBE) ([]byte, error) {
	return decryptMeta(globalSalt, masterPwd, pbe.IV, pbe.EntrySalt, pbe.Encrypted, pbe.IterationCount, pbe.KeySize)
}

func Nss(globalSalt, masterPwd []byte, pbe NssPBE) ([]byte, error) {
	return decryptMeta(globalSalt, masterPwd, pbe.IV, pbe.EntrySalt, pbe.Encrypted, pbe.IterationCount, pbe.KeySize)
}

func decryptMeta(globalSalt, masterPwd, nssIv, entrySalt, encrypted []byte, iter, keySize int) ([]byte, error) {
	k := sha1.Sum(globalSalt)
	log.Debug(hex.EncodeToString(k[:]))
	key := pbkdf2.Key(k[:], entrySalt, iter, keySize, sha256.New)
	log.Debug(hex.EncodeToString(key))
	i, err := hex.DecodeString("040e")
	if err != nil {
		log.Debug(err)
	}
	// @https://hg.mozilla.org/projects/nss/rev/fc636973ad06392d11597620b602779b4af312f6#l6.49
	iv := append(i, nssIv...)
	dst, err := aes128CBCDecrypt(key, iv, encrypted)
	if err != nil {
		log.Debug(err)
	}
	return dst, err
}
