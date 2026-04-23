package crypto

import (
	"bytes"
	"errors"
)

// yandexSignature is the protobuf wire-format header (field1 varint=1, field2 len=32) on every wrapped key.
var yandexSignature = []byte{0x08, 0x01, 0x12, 0x20}

var localEncryptorPrefix = []byte("v10")

const (
	yandexIntKeyBlobLen = 96 // 12B nonce + 68B ciphertext + 16B GCM tag
	yandexDataKeyLen    = 32
)

var (
	errYandexMarkerNotFound = errors.New("yandex: v10 marker not found in local_encryptor_data")
	errYandexBlobShort      = errors.New("yandex: encrypted intermediate key truncated")
	errYandexBadSignature   = errors.New("yandex: invalid protobuf signature on decrypted key")
	errYandexKeyTooShort    = errors.New("yandex: decrypted intermediate key shorter than 32 bytes")
)

// DecryptYandexIntermediateKey unwraps the per-DB data key from meta.local_encryptor_data. See RFC-012 §4.2.
func DecryptYandexIntermediateKey(masterKey, blob []byte) ([]byte, error) {
	idx := bytes.Index(blob, localEncryptorPrefix)
	if idx < 0 {
		return nil, errYandexMarkerNotFound
	}
	payload := blob[idx+len(localEncryptorPrefix):]
	if len(payload) < yandexIntKeyBlobLen {
		return nil, errYandexBlobShort
	}

	plaintext, err := AESGCMDecryptBlob(masterKey, payload[:yandexIntKeyBlobLen], nil)
	if err != nil {
		return nil, err
	}
	if !bytes.HasPrefix(plaintext, yandexSignature) {
		return nil, errYandexBadSignature
	}
	plaintext = plaintext[len(yandexSignature):]
	if len(plaintext) < yandexDataKeyLen {
		return nil, errYandexKeyTooShort
	}
	return plaintext[:yandexDataKeyLen], nil
}
