//go:build windows

package chromium

import (
	"fmt"
	"syscall"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
)

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

	returned, _, err := protectDataProc.Call(
		uintptr(unsafe.Pointer(&inBlob)),
		0, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(&outBlob)),
	)
	if returned == 0 {
		return nil, fmt.Errorf("CryptProtectData failed: %w", err)
	}
	defer localFreeProc.Call(uintptr(unsafe.Pointer(outBlob.pbData)))

	size := int(outBlob.cbData)
	result := make([]byte, size)
	copy(result, (*[1 << 30]byte)(unsafe.Pointer(outBlob.pbData))[:size])
	return result, nil
}

func TestDecryptValue_DPAPI_Windows(t *testing.T) {
	plaintext := []byte("dpapi_test_secret")

	encrypted, err := encryptWithDPAPI(plaintext)
	require.NoError(t, err)
	require.NotEmpty(t, encrypted)

	// No v10/v20 prefix → decryptValue routes to DPAPI path; no per-tier key needed.
	got, err := decryptValue(masterkey.Set{}, encrypted)
	require.NoError(t, err)
	assert.Equal(t, plaintext, got)
}
