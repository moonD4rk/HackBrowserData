package crypto

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/asn1"
	"errors"
)

type ASN1PBE interface {
	Decrypt(globalSalt []byte) ([]byte, error)

	Encrypt(globalSalt, plaintext []byte) ([]byte, error)
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

var ErrDecodeASN1Failed = errors.New("decode ASN1 data failed")

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

func (n nssPBE) Encrypt(globalSalt, plaintext []byte) ([]byte, error) {
	key, iv := n.deriveKeyAndIV(globalSalt)

	return DES3Encrypt(key, iv, plaintext)
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

func (m metaPBE) Encrypt(globalSalt, plaintext []byte) ([]byte, error) {
	key, iv := m.deriveKeyAndIV(globalSalt)

	return AES128CBCEncrypt(key, iv, plaintext)
}

func (m metaPBE) deriveKeyAndIV(globalSalt []byte) ([]byte, []byte) {
	password := sha1.Sum(globalSalt)

	salt := m.AlgoAttr.Data.Data.SlatAttr.EntrySalt
	iter := m.AlgoAttr.Data.Data.SlatAttr.IterationCount
	keyLen := m.AlgoAttr.Data.Data.SlatAttr.KeySize

	key := PBKDF2Key(password[:], salt, iter, keyLen, sha256.New)
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

func (l loginPBE) Decrypt(globalSalt []byte) ([]byte, error) {
	key, iv := l.deriveKeyAndIV(globalSalt)
	return DES3Decrypt(key, iv, l.Encrypted)
}

func (l loginPBE) Encrypt(globalSalt, plaintext []byte) ([]byte, error) {
	key, iv := l.deriveKeyAndIV(globalSalt)
	return DES3Encrypt(key, iv, plaintext)
}

func (l loginPBE) deriveKeyAndIV(globalSalt []byte) ([]byte, []byte) {
	return globalSalt, l.Data.IV
}
