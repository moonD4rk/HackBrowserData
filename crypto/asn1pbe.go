package crypto

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/asn1"
)

type ASN1PBE interface {
	Decrypt(globalSalt []byte) ([]byte, error)

	Encrypt(globalSalt, plaintext []byte) ([]byte, error)
}

func NewASN1PBE(b []byte) (pbe ASN1PBE, err error) {
	var (
		nss   privateKeyPBE
		meta  passwordCheckPBE
		login credentialPBE
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
	return nil, errDecodeASN1
}

// privateKeyPBE Struct
//
//	SEQUENCE (2 elem)
//		OBJECT IDENTIFIER
//		SEQUENCE (2 elem)
//			OCTET STRING (20 byte)
//			INTEGER 1
//	OCTET STRING (16 byte)
type privateKeyPBE struct {
	AlgoAttr struct {
		asn1.ObjectIdentifier
		SaltAttr struct {
			EntrySalt []byte
			KeyLen    int
		}
	}
	Encrypted []byte
}

// Decrypt decrypts the encrypted password with the global salt.
func (n privateKeyPBE) Decrypt(globalSalt []byte) ([]byte, error) {
	key, iv := n.deriveKeyAndIV(globalSalt)

	return DES3Decrypt(key, iv, n.Encrypted)
}

func (n privateKeyPBE) Encrypt(globalSalt, plaintext []byte) ([]byte, error) {
	key, iv := n.deriveKeyAndIV(globalSalt)

	return DES3Encrypt(key, iv, plaintext)
}

// deriveKeyAndIV derives the key and initialization vector (IV)
// from the global salt and entry salt.
func (n privateKeyPBE) deriveKeyAndIV(globalSalt []byte) ([]byte, []byte) {
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
type passwordCheckPBE struct {
	AlgoAttr  algoAttr
	Encrypted []byte
}

type algoAttr struct {
	asn1.ObjectIdentifier
	KDFParams struct {
		PBKDF2 struct {
			asn1.ObjectIdentifier
			SaltAttr saltAttr
		}
		IVData ivAttr
	}
}

type ivAttr struct {
	asn1.ObjectIdentifier
	IV []byte
}

type saltAttr struct {
	EntrySalt      []byte
	IterationCount int
	KeySize        int
	Algorithm      struct {
		asn1.ObjectIdentifier
	}
}

func (m passwordCheckPBE) Decrypt(globalSalt []byte) ([]byte, error) {
	key, iv := m.deriveKeyAndIV(globalSalt)

	return AESCBCDecrypt(key, iv, m.Encrypted)
}

func (m passwordCheckPBE) Encrypt(globalSalt, plaintext []byte) ([]byte, error) {
	key, iv := m.deriveKeyAndIV(globalSalt)

	return AESCBCEncrypt(key, iv, plaintext)
}

func (m passwordCheckPBE) deriveKeyAndIV(globalSalt []byte) ([]byte, []byte) {
	password := sha1.Sum(globalSalt)

	salt := m.AlgoAttr.KDFParams.PBKDF2.SaltAttr.EntrySalt
	iter := m.AlgoAttr.KDFParams.PBKDF2.SaltAttr.IterationCount
	keyLen := m.AlgoAttr.KDFParams.PBKDF2.SaltAttr.KeySize

	key := PBKDF2Key(password[:], salt, iter, keyLen, sha256.New)
	iv := append([]byte{4, 14}, m.AlgoAttr.KDFParams.IVData.IV...)
	return key, iv
}

// credentialPBE Struct
//
//	OCTET STRING (16 byte)
//	SEQUENCE (2 elem)
//			OBJECT IDENTIFIER
//			OCTET STRING (8 byte)
//	OCTET STRING (16 byte)
type credentialPBE struct {
	KeyCheck []byte
	Algo     struct {
		asn1.ObjectIdentifier
		IV []byte
	}
	Encrypted []byte
}

func (l credentialPBE) Decrypt(globalSalt []byte) ([]byte, error) {
	key, iv := l.deriveKeyAndIV(globalSalt)
	// The encryption algorithm can be reliably inferred from IV length:
	// - 8 bytes  : 3DES-CBC (legacy Firefox versions)
	// - 16 bytes : AES-CBC (Firefox 144+)
	if len(iv) == 8 {
		// Use 3DES for old Firefox versions
		return DES3Decrypt(key[:24], iv, l.Encrypted)
	} else if len(iv) == 16 {
		// Firefox 144+ uses 32-byte keys (AES-256-CBC)
		return AESCBCDecrypt(key, iv, l.Encrypted)
	}

	return nil, errUnsupportedIVLen
}

func (l credentialPBE) Encrypt(globalSalt, plaintext []byte) ([]byte, error) {
	key, iv := l.deriveKeyAndIV(globalSalt)
	// The encryption algorithm can be reliably inferred from IV length:
	// - 8 bytes  : 3DES-CBC (legacy Firefox versions)
	// - 16 bytes : AES-CBC (Firefox 144+)
	// This avoids relying on NSS-specific OIDs, which have changed historically.
	if len(iv) == 8 {
		// Use 3DES for old Firefox versions
		return DES3Encrypt(key[:24], iv, plaintext)
	} else if len(iv) == 16 {
		// Firefox 144+ uses 32-byte keys (AES-256-CBC)
		return AESCBCEncrypt(key, iv, plaintext)
	}

	return nil, errUnsupportedIVLen
}

func (l credentialPBE) deriveKeyAndIV(globalSalt []byte) ([]byte, []byte) {
	return globalSalt, l.Algo.IV
}
