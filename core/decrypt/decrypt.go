package decrypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/des"
	"encoding/asn1"
	"errors"

	"hack-browser-data/log"
)

var (
	errSecurityKeyIsEmpty = errors.New("input [security find-generic-password -wa 'Chrome'] in terminal")
	errPasswordIsEmpty    = errors.New("password is empty")
	errDecryptFailed      = errors.New("decrypt failed, password is empty")
)

func aes128CBCDecrypt(key, iv, encryptPass []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	dst := make([]byte, len(encryptPass))
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(dst, encryptPass)
	dst = PKCS5UnPadding(dst)
	return dst, nil
}

func PKCS5UnPadding(src []byte) []byte {
	length := len(src)
	unpad := int(src[length-1])
	return src[:(length - unpad)]
}

// Des3Decrypt use for decrypt firefox PBE
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
