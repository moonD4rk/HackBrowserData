//go:build windows

package chromium

import (
	"fmt"
	"syscall"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/crypto"
	"github.com/moond4rk/hackbrowserdata/crypto/keyretriever"
)

// encryptWithDPAPI encrypts data using Windows DPAPI (CryptProtectData).
// This is the reverse of DecryptDPAPI, used only for testing.
func encryptWithDPAPI(plaintext []byte) ([]byte, error) {
	crypt32 := syscall.NewLazyDLL("Crypt32.dll")
	kernel32 := syscall.NewLazyDLL("Kernel32.dll")
	protectDataProc := crypt32.NewProc("CryptProtectData")
	localFreeProc := kernel32.NewProc("LocalFree")

	var inBlob struct {
		cbData uint32
		pbData *byte
	}
	inBlob.cbData = uint32(len(plaintext))
	if len(plaintext) > 0 {
		inBlob.pbData = &plaintext[0]
	}

	var outBlob struct {
		cbData uint32
		pbData *byte
	}

	r, _, err := protectDataProc.Call(
		uintptr(unsafe.Pointer(&inBlob)),
		0, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(&outBlob)),
	)
	if r == 0 {
		return nil, fmt.Errorf("CryptProtectData failed: %w", err)
	}
	defer localFreeProc.Call(uintptr(unsafe.Pointer(outBlob.pbData)))

	size := int(outBlob.cbData)
	result := make([]byte, size)
	copy(result, (*[1 << 30]byte)(unsafe.Pointer(outBlob.pbData))[:size])
	return result, nil
}

func TestDecryptValue_V10_Windows(t *testing.T) {
	// Windows uses AES-GCM for v10 (not AES-CBC like macOS/Linux)
	plaintext := []byte("test_secret_value")
	nonce := []byte("123456789012") // 12-byte nonce

	gcmEncrypted, err := crypto.AESGCMEncrypt(testAESKey, nonce, plaintext)
	require.NoError(t, err)

	// v10 format on Windows: "v10" + nonce(12) + encrypted
	ciphertext := append([]byte("v10"), append(nonce, gcmEncrypted...)...)

	got, err := decryptValue(keyretriever.MasterKeys{V10: testAESKey}, ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, got)
}

func TestDecryptValue_DPAPI_Windows(t *testing.T) {
	// Round-trip: encrypt with CryptProtectData, decrypt with decryptValue
	plaintext := []byte("dpapi_test_secret")

	encrypted, err := encryptWithDPAPI(plaintext)
	require.NoError(t, err)
	require.NotEmpty(t, encrypted)

	// No v10/v20 prefix → decryptValue routes to DPAPI path; no per-tier key needed.
	got, err := decryptValue(keyretriever.MasterKeys{}, encrypted)
	require.NoError(t, err)
	assert.Equal(t, plaintext, got)
}
