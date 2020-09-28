package decrypt

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/asn1"
	"encoding/hex"
	"hack-browser-data/log"

	"golang.org/x/crypto/pbkdf2"
)

var (
	chromeIV = []byte{32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32}
)

func ChromePass(key, encryptPass []byte) ([]byte, error) {
	if len(encryptPass) > 3 {
		if len(key) == 0 {
			return nil, errSecurityKeyIsEmpty
		}
		m, err := aes128CBCDecrypt(key, chromeIV, encryptPass[3:])
		return m, err
	} else {
		return nil, errDecryptFailed
	}
}

func DPApi(data []byte) ([]byte, error) {
	return nil, nil
}

/*
SEQUENCE (2 elem)
	SEQUENCE (2 elem)
		OBJECT IDENTIFIER
		SEQUENCE (2 elem)
			OCTET STRING (20 byte)
			INTEGER 1
	OCTET STRING (16 byte)
*/

type MetaPBE struct {
	SequenceA
	Encrypted []byte
}

type SequenceA struct {
	DecryptMethod asn1.ObjectIdentifier
	SequenceB
}

type SequenceB struct {
	EntrySalt []byte
	Len       int
}

type NssPBE struct {
	SequenceNSSA
	Encrypted []byte
}

type SequenceNSSA struct {
	PKCS5PBES2 asn1.ObjectIdentifier
	SequenceNSSB
}
type SequenceNSSB struct {
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
	return decryptMeta(globalSalt, masterPwd, pbe.EntrySalt, pbe.Encrypted)
}

func Nss(globalSalt, masterPwd []byte, pbe NssPBE) ([]byte, error) {
	return decryptNss(globalSalt, masterPwd, pbe.IV, pbe.EntrySalt, pbe.Encrypted, pbe.IterationCount, pbe.KeySize)
}

func decryptMeta(globalSalt, masterPwd, entrySalt, encrypted []byte) ([]byte, error) {
	// byte[] GLMP; // GlobalSalt + MasterPassword
	// byte[] HP; // SHA1(GLMP)
	// byte[] HPES; // HP + EntrySalt
	// byte[] CHP; // SHA1(HPES)
	// byte[] PES; // EntrySalt completed to 20 bytes by zero
	// byte[] PESES; // PES + EntrySalt
	// byte[] k1;
	// byte[] tk;
	// byte[] k2;
	// byte[] k; // final value conytaining key and iv
	glmp := append(globalSalt, masterPwd...)
	hp := sha1.Sum(glmp)
	s := append(hp[:], entrySalt...)
	chp := sha1.Sum(s)
	pes := PaddingZero(entrySalt, 20)
	tk := hmac.New(sha1.New, chp[:])
	tk.Write(pes)
	pes = append(pes, entrySalt...)
	k1 := hmac.New(sha1.New, chp[:])
	k1.Write(pes)
	tkPlus := append(tk.Sum(nil), entrySalt...)
	k2 := hmac.New(sha1.New, chp[:])
	k2.Write(tkPlus)
	k := append(k1.Sum(nil), k2.Sum(nil)...)
	iv := k[len(k)-8:]
	key := k[:24]
	log.Warnf("key=%s iv=%s", hex.EncodeToString(key), hex.EncodeToString(iv))
	return Des3Decrypt(key, iv, encrypted)
}

func decryptNss(globalSalt, masterPwd, nssIv, entrySalt, encrypted []byte, iter, keySize int) ([]byte, error) {
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
