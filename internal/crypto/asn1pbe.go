package crypto

import (
	"crypto/aes"
	"crypto/des"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/asn1"
)

const des3KeySize = 24 // 3DES uses 24-byte (192-bit) keys

// ASN1PBE keys privateKeyPBE and passwordCheckPBE with an NSSPasswordKey, and credentialPBE with the master key.
type ASN1PBE interface {
	Decrypt(key []byte) ([]byte, error)
	Encrypt(key, plaintext []byte) ([]byte, error)
}

// NSSPasswordKey hashes globalSalt+password as NSS does: a newly set primary password gets a 48-byte salt and SHA-384.
func NSSPasswordKey(globalSalt, password []byte) []byte {
	input := make([]byte, 0, len(globalSalt)+len(password))
	input = append(append(input, globalSalt...), password...)
	if len(globalSalt) == sha512.Size384 {
		sum := sha512.Sum384(input)
		return sum[:]
	}
	sum := sha1.Sum(input)
	return sum[:]
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
	Algorithm struct {
		asn1.ObjectIdentifier
		Salt struct {
			EntrySalt []byte
			KeyLength int
		}
	}
	Encrypted []byte
}

func (pbe privateKeyPBE) Decrypt(passwordKey []byte) ([]byte, error) {
	key, iv := pbe.deriveKeyAndIV(passwordKey)
	return des3Decrypt(key, iv, pbe.Encrypted)
}

func (pbe privateKeyPBE) Encrypt(passwordKey, plaintext []byte) ([]byte, error) {
	key, iv := pbe.deriveKeyAndIV(passwordKey)
	return des3Encrypt(key, iv, plaintext)
}

// deriveKeyAndIV follows https://searchfox.org/mozilla-central/source/security/nss/lib/softoken/lowpbe.c
func (pbe privateKeyPBE) deriveKeyAndIV(passwordKey []byte) ([]byte, []byte) {
	entrySalt := pbe.Algorithm.Salt.EntrySalt
	combinedHash := sha1.Sum(append(append([]byte(nil), passwordKey...), entrySalt...))
	paddedSalt := zeroPad(entrySalt, 20)

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
	Algorithm algorithmAttributes
	Encrypted []byte
}

type algorithmAttributes struct {
	asn1.ObjectIdentifier
	KDFParams struct {
		PBKDF2 struct {
			asn1.ObjectIdentifier
			Salt saltAttributes
		}
		IVData ivAttributes
	}
}

type ivAttributes struct {
	asn1.ObjectIdentifier
	IV []byte
}

type saltAttributes struct {
	EntrySalt      []byte
	IterationCount int
	KeySize        int
	Algorithm      struct {
		asn1.ObjectIdentifier
	}
}

func (pbe passwordCheckPBE) Decrypt(passwordKey []byte) ([]byte, error) {
	key, iv := pbe.deriveKeyAndIV(passwordKey)
	return AESCBCDecrypt(key, iv, pbe.Encrypted)
}

func (pbe passwordCheckPBE) Encrypt(passwordKey, plaintext []byte) ([]byte, error) {
	key, iv := pbe.deriveKeyAndIV(passwordKey)
	return AESCBCEncrypt(key, iv, plaintext)
}

func (pbe passwordCheckPBE) deriveKeyAndIV(passwordKey []byte) ([]byte, []byte) {
	params := pbe.Algorithm.KDFParams.PBKDF2.Salt
	key := PBKDF2Key(passwordKey, params.EntrySalt, params.IterationCount, params.KeySize, sha256.New)

	// Firefox stores the IV with its ASN.1 OCTET STRING header (tag=0x04, length=0x0E).
	// The full 16-byte IV = [0x04, 0x0E] + 14-byte IV value from the parsed structure.
	iv := append([]byte{0x04, 0x0E}, pbe.Algorithm.KDFParams.IVData.IV...)
	return key, iv
}

type credentialPBE struct {
	KeyCheck  []byte
	Algorithm struct {
		asn1.ObjectIdentifier
		IV []byte
	}
	Encrypted []byte
}

func (pbe credentialPBE) Decrypt(masterKey []byte) ([]byte, error) {
	key, iv := pbe.deriveKeyAndIV(masterKey)
	// The cipher is inferred from IV length (avoids fragile OID checks):
	switch len(iv) {
	case des.BlockSize: // 8: 3DES-CBC (legacy Firefox)
		if len(key) < des3KeySize {
			return nil, errShortKey
		}
		return des3Decrypt(key[:des3KeySize], iv, pbe.Encrypted)
	case aes.BlockSize: // 16: AES-256-CBC (Firefox 144+)
		return AESCBCDecrypt(key, iv, pbe.Encrypted)
	default:
		return nil, errUnsupportedIVLength
	}
}

func (pbe credentialPBE) Encrypt(masterKey, plaintext []byte) ([]byte, error) {
	key, iv := pbe.deriveKeyAndIV(masterKey)
	switch len(iv) {
	case des.BlockSize:
		if len(key) < des3KeySize {
			return nil, errShortKey
		}
		return des3Encrypt(key[:des3KeySize], iv, plaintext)
	case aes.BlockSize:
		return AESCBCEncrypt(key, iv, plaintext)
	default:
		return nil, errUnsupportedIVLength
	}
}

func (pbe credentialPBE) deriveKeyAndIV(masterKey []byte) ([]byte, []byte) {
	return masterKey, pbe.Algorithm.IV
}
