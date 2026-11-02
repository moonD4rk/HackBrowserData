package crypto

import (
	"crypto/aes"
	"crypto/des"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/asn1"
)

const des3KeySize = 24 // 3DES uses 24-byte (192-bit) keys

// ASN1PBE keys privateKeyPBE and passwordCheckPBE with the global salt, and credentialPBE with the master key.
type ASN1PBE interface {
	Decrypt(key []byte) ([]byte, error)
	Encrypt(key, plaintext []byte) ([]byte, error)
}

func NewASN1PBE(encoded []byte) (pbe ASN1PBE, err error) {
	var (
		nss   privateKeyPBE
		meta  passwordCheckPBE
		login credentialPBE
	)
	if _, err := asn1.Unmarshal(encoded, &nss); err == nil {
		return nss, nil
	}
	if _, err := asn1.Unmarshal(encoded, &meta); err == nil {
		return meta, nil
	}
	if _, err := asn1.Unmarshal(encoded, &login); err == nil {
		return login, nil
	}
	return nil, errDecodeASN1
}

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

func (n privateKeyPBE) Decrypt(globalSalt []byte) ([]byte, error) {
	key, iv := n.deriveKeyAndIV(globalSalt)
	return DES3Decrypt(key, iv, n.Encrypted)
}

func (n privateKeyPBE) Encrypt(globalSalt, plaintext []byte) ([]byte, error) {
	key, iv := n.deriveKeyAndIV(globalSalt)
	return DES3Encrypt(key, iv, plaintext)
}

// deriveKeyAndIV follows https://searchfox.org/mozilla-central/source/security/nss/lib/softoken/lowpbe.c
func (n privateKeyPBE) deriveKeyAndIV(globalSalt []byte) ([]byte, []byte) {
	entrySalt := n.AlgoAttr.SaltAttr.EntrySalt
	passwordHash := sha1.Sum(globalSalt)
	combinedHash := sha1.Sum(append(passwordHash[:], entrySalt...))
	paddedSalt := paddingZero(entrySalt, 20)

	saltHMAC := hmac.New(sha1.New, combinedHash[:])
	saltHMAC.Write(paddedSalt)

	firstHMAC := hmac.New(sha1.New, combinedHash[:])
	firstHMAC.Write(append(paddedSalt, entrySalt...))

	secondHMAC := hmac.New(sha1.New, combinedHash[:])
	secondHMAC.Write(append(saltHMAC.Sum(nil), entrySalt...))

	derivedKey := append(firstHMAC.Sum(nil), secondHMAC.Sum(nil)...)
	return derivedKey[:24], derivedKey[len(derivedKey)-8:]
}

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

	params := m.AlgoAttr.KDFParams.PBKDF2.SaltAttr
	key := PBKDF2Key(password[:], params.EntrySalt, params.IterationCount, params.KeySize, sha256.New)

	// Firefox stores the IV with its ASN.1 OCTET STRING header (tag=0x04, length=0x0E).
	// The full 16-byte IV = [0x04, 0x0E] + 14-byte IV value from the parsed structure.
	iv := append([]byte{0x04, 0x0E}, m.AlgoAttr.KDFParams.IVData.IV...)
	return key, iv
}

type credentialPBE struct {
	KeyCheck []byte
	Algo     struct {
		asn1.ObjectIdentifier
		IV []byte
	}
	Encrypted []byte
}

func (l credentialPBE) Decrypt(masterKey []byte) ([]byte, error) {
	key, iv := l.deriveKeyAndIV(masterKey)
	// The cipher is inferred from IV length (avoids fragile OID checks):
	switch len(iv) {
	case des.BlockSize: // 8: 3DES-CBC (legacy Firefox)
		if len(key) < des3KeySize {
			return nil, errShortKey
		}
		return DES3Decrypt(key[:des3KeySize], iv, l.Encrypted)
	case aes.BlockSize: // 16: AES-256-CBC (Firefox 144+)
		return AESCBCDecrypt(key, iv, l.Encrypted)
	default:
		return nil, errUnsupportedIVLen
	}
}

func (l credentialPBE) Encrypt(masterKey, plaintext []byte) ([]byte, error) {
	key, iv := l.deriveKeyAndIV(masterKey)
	switch len(iv) {
	case des.BlockSize:
		if len(key) < des3KeySize {
			return nil, errShortKey
		}
		return DES3Encrypt(key[:des3KeySize], iv, plaintext)
	case aes.BlockSize:
		return AESCBCEncrypt(key, iv, plaintext)
	default:
		return nil, errUnsupportedIVLen
	}
}

func (l credentialPBE) deriveKeyAndIV(masterKey []byte) ([]byte, []byte) {
	return masterKey, l.Algo.IV
}
