package decrypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/des"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/asn1"
	"errors"

	"hack-browser-data/log"

	"golang.org/x/crypto/pbkdf2"
)

var (
	errSecurityKeyIsEmpty = errors.New("input [security find-generic-password -wa 'Chrome'] in terminal")
	errDecryptFailed      = errors.New("decrypt failed, password is empty")
	errDecodeASN1Failed   = errors.New("decode ASN1 data failed")
	errEncryptedLength    = errors.New("length of encrypted password less than block size")
)

type ASN1PBE interface {
	Decrypt(globalSalt, masterPwd []byte) (key []byte, err error)
}

func NewASN1PBE(b []byte) (pbe ASN1PBE, err error) {
	var (
		n NssPBE
		m MetaPBE
		l LoginPBE
	)
	if _, err := asn1.Unmarshal(b, &n); err == nil {
		return n, nil
	}
	if _, err := asn1.Unmarshal(b, &m); err == nil {
		return m, nil
	}
	if _, err := asn1.Unmarshal(b, &l); err == nil {
		return l, nil
	}
	return nil, errDecodeASN1Failed
}

/* NSS Struct
SEQUENCE (2 elem)
	SEQUENCE (2 elem)
		OBJECT IDENTIFIER
		SEQUENCE (2 elem)
			OCTET STRING (20 byte)
			INTEGER 1
	OCTET STRING (16 byte)
*/
type NssPBE struct {
	NssSequenceA
	Encrypted []byte
}

type NssSequenceA struct {
	DecryptMethod asn1.ObjectIdentifier
	NssSequenceB
}

type NssSequenceB struct {
	EntrySalt []byte
	Len       int
}

func (n NssPBE) Decrypt(globalSalt, masterPwd []byte) (key []byte, err error) {
	// byte[] GLMP; // GlobalSalt + MasterPassword
	// byte[] HP; // SHA1(GLMP)
	// byte[] HPES; // HP + EntrySalt
	// byte[] CHP; // SHA1(HPES)
	// byte[] PES; // EntrySalt completed to 20 bytes by zero
	// byte[] PESES; // PES + EntrySalt
	// byte[] k1;
	// byte[] tk;
	// byte[] k2;
	// byte[] k; // final value containing key and iv
	glmp := append(globalSalt, masterPwd...)
	hp := sha1.Sum(glmp)
	s := append(hp[:], n.EntrySalt...)
	chp := sha1.Sum(s)
	pes := PaddingZero(n.EntrySalt, 20)
	tk := hmac.New(sha1.New, chp[:])
	tk.Write(pes)
	pes = append(pes, n.EntrySalt...)
	k1 := hmac.New(sha1.New, chp[:])
	k1.Write(pes)
	tkPlus := append(tk.Sum(nil), n.EntrySalt...)
	k2 := hmac.New(sha1.New, chp[:])
	k2.Write(tkPlus)
	k := append(k1.Sum(nil), k2.Sum(nil)...)
	iv := k[len(k)-8:]
	log.Debug("get firefox pbe key and iv success")
	return des3Decrypt(k[:24], iv, n.Encrypted)
}

/* META Struct
SEQUENCE (2 elem)
	SEQUENCE (2 elem)
    	OBJECT IDENTIFIER
    	SEQUENCE (2 elem)
      	SEQUENCE (2 elem)
        	OBJECT IDENTIFIER
        	SEQUENCE (4 elem)
          	OCTET STRING (32 byte)
          		INTEGER 1
          		INTEGER 32
          		SEQUENCE (1 elem)
            	OBJECT IDENTIFIER
      	SEQUENCE (2 elem)
        	OBJECT IDENTIFIER
        	OCTET STRING (14 byte)
  	OCTET STRING (16 byte)
*/
type MetaPBE struct {
	MetaSequenceA
	Encrypted []byte
}

type MetaSequenceA struct {
	PKCS5PBES2 asn1.ObjectIdentifier
	MetaSequenceB
}
type MetaSequenceB struct {
	MetaSequenceC
	MetaSequenceD
}

type MetaSequenceC struct {
	PKCS5PBKDF2 asn1.ObjectIdentifier
	MetaSequenceE
}

type MetaSequenceD struct {
	AES256CBC asn1.ObjectIdentifier
	IV        []byte
}

type MetaSequenceE struct {
	EntrySalt      []byte
	IterationCount int
	KeySize        int
	MetaSequenceF
}

type MetaSequenceF struct {
	HMACWithSHA256 asn1.ObjectIdentifier
}

func (m MetaPBE) Decrypt(globalSalt, masterPwd []byte) (key2 []byte, err error) {
	k := sha1.Sum(globalSalt)
	key := pbkdf2.Key(k[:], m.EntrySalt, m.IterationCount, m.KeySize, sha256.New)
	iv := append([]byte{4, 14}, m.IV...)
	return aes128CBCDecrypt(key, iv, m.Encrypted)
}

func aes128CBCDecrypt(key, iv, encryptPass []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	encryptLen := len(encryptPass)
	if encryptLen < block.BlockSize() {
		return nil, errEncryptedLength
	}

	dst := make([]byte, encryptLen)
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

// des3Decrypt use for decrypt firefox PBE
func des3Decrypt(key, iv []byte, src []byte) ([]byte, error) {
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

/* Login Struct
SEQUENCE (3 elem)
	OCTET STRING (16 byte)
	SEQUENCE (2 elem)
		OBJECT IDENTIFIER
		OCTET STRING (8 byte)
	OCTET STRING (16 byte)
*/
type LoginPBE struct {
	CipherText []byte
	LoginSequence
	Encrypted []byte
}

type LoginSequence struct {
	asn1.ObjectIdentifier
	IV []byte
}

func (l LoginPBE) Decrypt(globalSalt, masterPwd []byte) (key []byte, err error) {
	return des3Decrypt(globalSalt, l.IV, l.Encrypted)
}
