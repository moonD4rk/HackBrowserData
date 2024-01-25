package crypto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/des"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/asn1"
	"errors"
	"fmt"

	"golang.org/x/crypto/pbkdf2"
)

var (
	ErrCiphertextLengthIsInvalid = errors.New("ciphertext length is invalid")
	ErrDecodeASN1Failed          = errors.New("decode ASN1 data failed")
	errEncryptedLength           = errors.New("length of encrypted password less than block size")
)

type ASN1PBE interface {
	Decrypt(globalSalt []byte) (key []byte, err error)
}

func NewASN1PBE(b []byte) (pbe ASN1PBE, err error) {
	var (
		nss   nssPBE
		meta  metaPBE
		login loginPBE
	)
	if _, err := asn1.Unmarshal(b, &nss); err == nil {
		return nss, nil
	}
	if _, err := asn1.Unmarshal(b, &meta); err == nil {
		return meta, nil
	}
	if _, err := asn1.Unmarshal(b, &login); err == nil {
		return login, nil
	}
	return nil, ErrDecodeASN1Failed
}

// nssPBE Struct
//
//	SEQUENCE (2 elem)
//		OBJECT IDENTIFIER
//		SEQUENCE (2 elem)
//			OCTET STRING (20 byte)
//			INTEGER 1
//	OCTET STRING (16 byte)
type nssPBE struct {
	AlgoAttr struct {
		asn1.ObjectIdentifier
		SaltAttr struct {
			EntrySalt []byte
			Len       int
		}
	}
	Encrypted []byte
}

// Decrypt decrypts the encrypted password with the global salt.
func (n nssPBE) Decrypt(globalSalt []byte) ([]byte, error) {
	key, iv := n.deriveKeyAndIV(globalSalt)
	return DES3Decrypt(key, iv, n.Encrypted)
}

// deriveKeyAndIV derives the key and initialization vector (IV)
// from the global salt and entry salt.
func (n nssPBE) deriveKeyAndIV(globalSalt []byte) ([]byte, []byte) {
	salt := n.AlgoAttr.SaltAttr.EntrySalt
	hashPrefix := sha1.Sum(globalSalt)
	compositeHash := sha1.Sum(append(hashPrefix[:], salt...))
	paddedEntrySalt := paddingZero(salt, 20)

	hmacProcessor := hmac.New(sha1.New, compositeHash[:])
	hmacProcessor.Write(paddedEntrySalt)

	paddedEntrySalt = append(paddedEntrySalt, salt...)
	keyComponent1 := hmac.New(sha1.New, compositeHash[:])
	keyComponent1.Write(paddedEntrySalt)

	hmacWithSalt := append(hmacProcessor.Sum(nil), salt...)
	keyComponent2 := hmac.New(sha1.New, compositeHash[:])
	keyComponent2.Write(hmacWithSalt)

	key := append(keyComponent1.Sum(nil), keyComponent2.Sum(nil)...)
	iv := key[len(key)-8:]
	return key[:24], iv
}

// MetaPBE Struct
//
//	SEQUENCE (2 elem)
//		OBJECT IDENTIFIER
//	    SEQUENCE (2 elem)
//	    SEQUENCE (2 elem)
//	      	OBJECT IDENTIFIER
//	       	SEQUENCE (4 elem)
//	       	OCTET STRING (32 byte)
//	      		INTEGER 1
//	       		INTEGER 32
//	       		SEQUENCE (1 elem)
//	          	OBJECT IDENTIFIER
//	    SEQUENCE (2 elem)
//	      	OBJECT IDENTIFIER
//	      	OCTET STRING (14 byte)
//	OCTET STRING (16 byte)
type metaPBE struct {
	AlgoAttr  algoAttr
	Encrypted []byte
}

type algoAttr struct {
	asn1.ObjectIdentifier
	Data struct {
		Data struct {
			asn1.ObjectIdentifier
			SlatAttr slatAttr
		}
		IVData ivAttr
	}
}

type ivAttr struct {
	asn1.ObjectIdentifier
	IV []byte
}

type slatAttr struct {
	EntrySalt      []byte
	IterationCount int
	KeySize        int
	Algorithm      struct {
		asn1.ObjectIdentifier
	}
}

func (m metaPBE) Decrypt(globalSalt []byte) ([]byte, error) {
	key, iv := m.deriveKeyAndIV(globalSalt)

	return AES128CBCDecrypt(key, iv, m.Encrypted)
}

func (m metaPBE) deriveKeyAndIV(globalSalt []byte) ([]byte, []byte) {
	password := sha1.Sum(globalSalt)

	salt := m.AlgoAttr.Data.Data.SlatAttr.EntrySalt
	iter := m.AlgoAttr.Data.Data.SlatAttr.IterationCount
	keyLen := m.AlgoAttr.Data.Data.SlatAttr.KeySize

	key := pbkdf2.Key(password[:], salt, iter, keyLen, sha256.New)
	iv := append([]byte{4, 14}, m.AlgoAttr.Data.IVData.IV...)
	return key, iv
}

// loginPBE Struct
//
//	OCTET STRING (16 byte)
//	SEQUENCE (2 elem)
//			OBJECT IDENTIFIER
//			OCTET STRING (8 byte)
//	OCTET STRING (16 byte)
type loginPBE struct {
	CipherText []byte
	Data       struct {
		asn1.ObjectIdentifier
		IV []byte
	}
	Encrypted []byte
}

func (l loginPBE) Decrypt(globalSalt []byte) (key []byte, err error) {
	key, iv := l.deriveKeyAndIV(globalSalt)
	return DES3Decrypt(key, iv, l.Encrypted)
}

func (l loginPBE) deriveKeyAndIV(globalSalt []byte) ([]byte, []byte) {
	return globalSalt, l.Data.IV
}

func AES128CBCDecrypt(key, iv, encryptedData []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	// Check encrypted data length
	if len(encryptedData) < aes.BlockSize {
		return nil, errors.New("AES128CBCDecrypt: encrypted data too short")
	}
	if len(encryptedData)%aes.BlockSize != 0 {
		return nil, errors.New("AES128CBCDecrypt: encrypted data is not a multiple of the block size")
	}

	decryptedData := make([]byte, len(encryptedData))
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(decryptedData, encryptedData)

	// Unpad the decrypted data and handle potential padding errors
	decryptedData, err = pkcs5UnPadding(decryptedData)
	if err != nil {
		return nil, fmt.Errorf("AES128CBCDecrypt: %w", err)
	}

	return decryptedData, nil
}

func AES128CBCEncrypt(key, iv, data []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	data = pkcs5Padding(data, block.BlockSize())
	encryptedData := make([]byte, len(data))
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(encryptedData, data)

	return encryptedData, nil
}

func DES3Decrypt(key, iv []byte, src []byte) ([]byte, error) {
	block, err := des.NewTripleDESCipher(key)
	if err != nil {
		return nil, err
	}
	if len(src) < des.BlockSize {
		return nil, errors.New("DES3Decrypt: ciphertext too short")
	}
	if len(src)%block.BlockSize() != 0 {
		return nil, errors.New("DES3Decrypt: ciphertext is not a multiple of the block size")
	}

	blockMode := cipher.NewCBCDecrypter(block, iv)
	sq := make([]byte, len(src))
	blockMode.CryptBlocks(sq, src)

	return pkcs5UnPadding(sq)
}

func DES3Encrypt(key, iv []byte, src []byte) ([]byte, error) {
	block, err := des.NewTripleDESCipher(key)
	if err != nil {
		return nil, err
	}

	src = pkcs5Padding(src, block.BlockSize())
	dst := make([]byte, len(src))
	blockMode := cipher.NewCBCEncrypter(block, iv)
	blockMode.CryptBlocks(dst, src)

	return dst, nil
}

// AESGCMEncrypt encrypts plaintext using AES encryption in GCM mode.
func AESGCMEncrypt(key, nonce, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	blockMode, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	// The first parameter is the prefix for the output, we can leave it nil.
	// The Seal method encrypts and authenticates the data, appending the result to the dst.
	encryptedData := blockMode.Seal(nil, nonce, plaintext, nil)
	return encryptedData, nil
}

// AESGCMDecrypt chromium > 80 https://source.chromium.org/chromium/chromium/src/+/master:components/os_crypt/os_crypt_win.cc
func AESGCMDecrypt(key, nounce, encrypted []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	blockMode, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	origData, err := blockMode.Open(nil, nounce, encrypted, nil)
	if err != nil {
		return nil, err
	}
	return origData, nil
}

func paddingZero(s []byte, length int) []byte {
	padding := length - len(s)
	if padding <= 0 {
		return s
	}
	return append(s, make([]byte, padding)...)
}

func pkcs5UnPadding(src []byte) ([]byte, error) {
	length := len(src)
	if length == 0 {
		return nil, errors.New("pkcs5UnPadding: src should not be empty")
	}
	padding := int(src[length-1])
	if padding < 1 || padding > aes.BlockSize {
		return nil, errors.New("pkcs5UnPadding: invalid padding size")
	}
	return src[:length-padding], nil
}

func pkcs5Padding(src []byte, blockSize int) []byte {
	padding := blockSize - (len(src) % blockSize)
	padText := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(src, padText...)
}
