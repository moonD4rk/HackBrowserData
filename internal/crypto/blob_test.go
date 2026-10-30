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
