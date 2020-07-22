package decrypt

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/asn1"
	"hack-browser-data/log"
)

var (
	chromeIV = []byte{32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32}
)

func ChromePass(key, encryptPass []byte) ([]byte, error) {
	if len(encryptPass) > 3 {
		if len(key) == 0 {
			return nil, errKeyIsEmpty
		}
		m, err := aes128CBCDecrypt(key, chromeIV, encryptPass[3:])
		return m, err
	} else {
		return nil, errDecryptFailed
	}
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

type NssPBE struct {
	SequenceA
	Encrypted []byte
}

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

func DecodeMeta(metaBytes []byte) (pbe MetaPBE, err error) {
	_, err = asn1.Unmarshal(metaBytes, &pbe)
	if err != nil {
		log.Error(err)
		return
	}
	return
}

func DPApi(data []byte) ([]byte, error) {
	return nil, nil
}

func DecodeNss(nssA11Bytes []byte) (pbe NssPBE, err error) {
	_, err = asn1.Unmarshal(nssA11Bytes, &pbe)
	if err != nil {
		log.Error(err)
		return
	}
	return
}

func Meta(globalSalt, masterPwd []byte, pbe MetaPBE) ([]byte, error) {
	return decryptPBE(globalSalt, masterPwd, pbe.EntrySalt, pbe.Encrypted)
}

func Nss(globalSalt, masterPwd []byte, pbe NssPBE) ([]byte, error) {
	return decryptPBE(globalSalt, masterPwd, pbe.EntrySalt, pbe.Encrypted)
}

func decryptPBE(globalSalt, masterPwd, entrySalt, encrypted []byte) ([]byte, error) {
	//byte[] GLMP; // GlobalSalt + MasterPassword
	//byte[] HP; // SHA1(GLMP)
	//byte[] HPES; // HP + EntrySalt
	//byte[] CHP; // SHA1(HPES)
	//byte[] PES; // EntrySalt completed to 20 bytes by zero
	//byte[] PESES; // PES + EntrySalt
	//byte[] k1;
	//byte[] tk;
	//byte[] k2;
	//byte[] k; // final value conytaining key and iv
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
	log.Debug("get firefox pbe key and iv success")
	return Des3Decrypt(key, iv, encrypted)
}
