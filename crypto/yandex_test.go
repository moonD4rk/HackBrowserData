package crypto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"errors"
	"testing"
)

// encryptAESGCM is a test helper that produces a GCM ciphertext with caller-supplied AAD.
func encryptAESGCM(t *testing.T, key, nonce, plaintext, aad []byte) []byte {
	t.Helper()
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("aes.NewCipher: %v", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatalf("cipher.NewGCM: %v", err)
	}
	return aead.Seal(nil, nonce, plaintext, aad)
}

// testPlaintextPayloadLen: plaintext size before AES-GCM seal inside meta.local_encryptor_data.
// 96 (blob) - 12 (nonce) - 16 (tag) = 68 bytes.
const testPlaintextPayloadLen = yandexIntKeyBlobLen - gcmNonceSize - 16

func buildLocalEncryptorBlob(t *testing.T, masterKey, dataKey []byte) []byte {
	t.Helper()
	nonce := bytes.Repeat([]byte{0xAB}, gcmNonceSize)
	plaintext := append([]byte{}, yandexSignature...)
	plaintext = append(plaintext, dataKey...)
	plaintext = append(plaintext, make([]byte, testPlaintextPayloadLen-len(plaintext))...)
	ciphertext := encryptAESGCM(t, masterKey, nonce, plaintext, nil)
	if len(ciphertext) != yandexIntKeyBlobLen-gcmNonceSize {
		t.Fatalf("unexpected ciphertext len: got %d want %d", len(ciphertext), yandexIntKeyBlobLen-gcmNonceSize)
	}
	blob := []byte{0x01, 0x02, 0x03, 0x04} // arbitrary protobuf preamble
	blob = append(blob, localEncryptorPrefix...)
	blob = append(blob, nonce...)
	blob = append(blob, ciphertext...)
	blob = append(blob, 0xFF, 0xFE) // trailing junk should be ignored
	return blob
}

func TestDecryptYandexIntermediateKey_RoundTrip(t *testing.T) {
	masterKey := bytes.Repeat([]byte{0x11}, 32)
	dataKey := bytes.Repeat([]byte{0x22}, yandexDataKeyLen)
	blob := buildLocalEncryptorBlob(t, masterKey, dataKey)

	got, err := DecryptYandexIntermediateKey(masterKey, blob)
	if err != nil {
		t.Fatalf("DecryptYandexIntermediateKey: %v", err)
	}
	if !bytes.Equal(got, dataKey) {
		t.Errorf("key mismatch: got %x want %x", got, dataKey)
	}
}

func TestDecryptYandexIntermediateKey_MissingMarker(t *testing.T) {
	_, err := DecryptYandexIntermediateKey(bytes.Repeat([]byte{0x11}, 32), []byte("no marker here"))
	if !errors.Is(err, errYandexMarkerNotFound) {
		t.Fatalf("expected errYandexMarkerNotFound, got %v", err)
	}
}

func TestDecryptYandexIntermediateKey_Truncated(t *testing.T) {
	blob := append([]byte{0x00, 0x00}, localEncryptorPrefix...)
	blob = append(blob, bytes.Repeat([]byte{0x55}, yandexIntKeyBlobLen-1)...)
	_, err := DecryptYandexIntermediateKey(bytes.Repeat([]byte{0x11}, 32), blob)
	if !errors.Is(err, errYandexBlobShort) {
		t.Fatalf("expected errYandexBlobShort, got %v", err)
	}
}

func TestDecryptYandexIntermediateKey_BadSignature(t *testing.T) {
	masterKey := bytes.Repeat([]byte{0x11}, 32)
	nonce := bytes.Repeat([]byte{0xAB}, gcmNonceSize)
	plaintext := append([]byte{0xDE, 0xAD, 0xBE, 0xEF}, bytes.Repeat([]byte{0x22}, yandexDataKeyLen)...)
	plaintext = append(plaintext, make([]byte, testPlaintextPayloadLen-len(plaintext))...)
	ciphertext := encryptAESGCM(t, masterKey, nonce, plaintext, nil)
	blob := append([]byte{}, localEncryptorPrefix...)
	blob = append(blob, nonce...)
	blob = append(blob, ciphertext...)

	_, err := DecryptYandexIntermediateKey(masterKey, blob)
	if !errors.Is(err, errYandexBadSignature) {
		t.Fatalf("expected errYandexBadSignature, got %v", err)
	}
}

// TestDecryptYandexIntermediateKey_TrailingDataIgnored verifies that trailing bytes past
// signature+32 are discarded.
func TestDecryptYandexIntermediateKey_TrailingDataIgnored(t *testing.T) {
	masterKey := bytes.Repeat([]byte{0x11}, 32)
	nonce := bytes.Repeat([]byte{0xAB}, gcmNonceSize)
	plaintext := append([]byte{}, yandexSignature...)
	plaintext = append(plaintext, bytes.Repeat([]byte{0x22}, 16)...)
	plaintext = append(plaintext, make([]byte, testPlaintextPayloadLen-len(plaintext))...)
	ciphertext := encryptAESGCM(t, masterKey, nonce, plaintext, nil)
	blob := append([]byte{}, localEncryptorPrefix...)
	blob = append(blob, nonce...)
	blob = append(blob, ciphertext...)

	got, err := DecryptYandexIntermediateKey(masterKey, blob)
	if err != nil {
		t.Fatalf("DecryptYandexIntermediateKey: %v", err)
	}
	want := bytes.Repeat([]byte{0x22}, 16)
	want = append(want, make([]byte, 16)...)
	if !bytes.Equal(got, want) {
		t.Errorf("key mismatch: got %x want %x", got, want)
	}
}

func TestAESGCMDecryptBlob_RoundTrip(t *testing.T) {
	key := bytes.Repeat([]byte{0x55}, 32)
	nonce := bytes.Repeat([]byte{0x66}, gcmNonceSize)
	aad := []byte("row-aad")
	plaintext := []byte("row-plaintext")
	blob := append([]byte{}, nonce...)
	blob = append(blob, encryptAESGCM(t, key, nonce, plaintext, aad)...)

	got, err := AESGCMDecryptBlob(key, blob, aad)
	if err != nil {
		t.Fatalf("AESGCMDecryptBlob: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Errorf("plaintext mismatch: got %q want %q", got, plaintext)
	}
}

func TestAESGCMDecryptBlob_BadAAD(t *testing.T) {
	key := bytes.Repeat([]byte{0x55}, 32)
	nonce := bytes.Repeat([]byte{0x66}, gcmNonceSize)
	blob := append([]byte{}, nonce...)
	blob = append(blob, encryptAESGCM(t, key, nonce, []byte("x"), []byte("aad-A"))...)

	if _, err := AESGCMDecryptBlob(key, blob, []byte("aad-B")); err == nil {
		t.Fatal("expected authentication failure with mismatched AAD")
	}
}

func TestAESGCMDecryptBlob_TooShort(t *testing.T) {
	_, err := AESGCMDecryptBlob(bytes.Repeat([]byte{0x55}, 32), []byte{0x01, 0x02}, nil)
	if !errors.Is(err, errShortCiphertext) {
		t.Fatalf("expected errShortCiphertext, got %v", err)
	}
}
