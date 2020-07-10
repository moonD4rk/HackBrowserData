package utils

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/asn1"
	"encoding/hex"
	"errors"
	"hack-browser-data/log"
	"os/exec"
	"strings"

	"golang.org/x/crypto/pbkdf2"
)

const (
	chromeProfilePath  = "/Users/*/Library/Application Support/Google/Chrome/*/"
	chromeCommand      = "Chrome"
	edgeProfilePath    = "/Users/*/Library/Application Support/Microsoft Edge/*/"
	edgeCommand        = "Microsoft Edge"
	fireFoxProfilePath = "/Users/*/Library/Application Support/Firefox/Profiles/*.default-release/"
	fireFoxCommand     = ""
)

var (
	iv          = []byte{32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32}
	command     = []string{"security", "find-generic-password", "-wa"}
	chromeSalt  = []byte("saltysalt")
	chromeKey   []byte
	browserList = map[string]struct {
		ProfilePath string
		Command     string
	}{
		"chrome": {
			chromeProfilePath,
			chromeCommand,
		},
		"edge": {
			edgeProfilePath,
			edgeCommand,
		},
		"firefox": {
			fireFoxProfilePath,
			fireFoxCommand,
		},
	}
)

func DecryptStringWithDPAPI(data []byte) (string, error) {
	return string(data), nil
}

func PickBrowser(name string) (browserDir, command string, err error) {
	name = strings.ToLower(name)
	if choice, ok := browserList[name]; ok {
		return choice.ProfilePath, choice.Command, err
	}
	return "", "", errBrowserNotSupported
}

func InitKey(key string) error {
	var (
		cmd            *exec.Cmd
		stdout, stderr bytes.Buffer
	)
	cmd = exec.Command(command[0], command[1], command[2], key)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		log.Error(err)
		return err
	}
	if stderr.Len() > 0 {
		err = errors.New(stderr.String())
		log.Error(err)
	}
	temp := stdout.Bytes()
	chromePass := temp[:len(temp)-1]
	decryptChromeKey(chromePass)
	return err
}

func decryptChromeKey(chromePass []byte) {
	chromeKey = pbkdf2.Key(chromePass, chromeSalt, 1003, 16, sha1.New)
}

func DecryptChromePass(encryptPass []byte) (string, error) {
	if len(encryptPass) > 3 {
		if len(chromeKey) == 0 {
			return "", errKeyIsEmpty
		}
		m, err := aes128CBCDecrypt(chromeKey, iv, encryptPass[3:])
		return string(m), err
	} else {
		return "", &DecryptError{
			err: errPasswordIsEmpty,
		}
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
	log.Debug(hex.EncodeToString(metaBytes))
	_, err = asn1.Unmarshal(metaBytes, &pbe)
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

func DecryptMeta(globalSalt, masterPwd []byte, pbe MetaPBE) ([]byte, error) {
	return decryptPBE(globalSalt, masterPwd, pbe.EntrySalt, pbe.Encrypted)
}

func DecryptNss(globalSalt, masterPwd []byte, pbe NssPBE) ([]byte, error) {
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
	log.Warnf("key=%s iv=%s", hex.EncodeToString(key), hex.EncodeToString(iv))
	return Des3Decrypt(key, iv, encrypted)
}
