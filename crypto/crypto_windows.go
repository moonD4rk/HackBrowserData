//go:build windows

package crypto

import (
	"fmt"
	"syscall"
	"unsafe"
)

const (
	gcmNonceSize   = 12                              // AES-GCM standard nonce size
	minGCMDataSize = versionPrefixLen + gcmNonceSize // "v10" + nonce = 15 bytes minimum
)

func DecryptChromium(key, ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < minGCMDataSize {
		return nil, errShortCiphertext
	}
	nonce := ciphertext[versionPrefixLen : versionPrefixLen+gcmNonceSize]
	payload := ciphertext[versionPrefixLen+gcmNonceSize:]
	return AESGCMDecrypt(key, nonce, payload)
}

// DecryptYandex decrypts a Yandex-encrypted value.
// TODO: Yandex uses the same AES-GCM format as Chromium for now;
// update when Yandex-specific decryption diverges.
func DecryptYandex(key, ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < minGCMDataSize {
		return nil, errShortCiphertext
	}
	nonce := ciphertext[versionPrefixLen : versionPrefixLen+gcmNonceSize]
	payload := ciphertext[versionPrefixLen+gcmNonceSize:]
	return AESGCMDecrypt(key, nonce, payload)
}

type dataBlob struct {
	cbData uint32
	pbData *byte
}

func newBlob(d []byte) *dataBlob {
	if len(d) == 0 {
		return &dataBlob{}
	}
	return &dataBlob{
		pbData: &d[0],
		cbData: uint32(len(d)),
	}
}

func (b *dataBlob) bytes() []byte {
	d := make([]byte, b.cbData)
	copy(d, (*[1 << 30]byte)(unsafe.Pointer(b.pbData))[:])
	return d
}

// DecryptDPAPI (Data Protection Application Programming Interface)
// is a simple cryptographic application programming interface
// available as a built-in component in Windows 2000 and
// later versions of Microsoft Windows operating systems
func DecryptDPAPI(ciphertext []byte) ([]byte, error) {
	crypt32 := syscall.NewLazyDLL("Crypt32.dll")
	kernel32 := syscall.NewLazyDLL("Kernel32.dll")
	unprotectDataProc := crypt32.NewProc("CryptUnprotectData")
	localFreeProc := kernel32.NewProc("LocalFree")

	var outBlob dataBlob
	r, _, err := unprotectDataProc.Call(
		uintptr(unsafe.Pointer(newBlob(ciphertext))),
		0, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(&outBlob)),
	)
	if r == 0 {
		return nil, fmt.Errorf("CryptUnprotectData failed with error %w", err)
	}

	defer localFreeProc.Call(uintptr(unsafe.Pointer(outBlob.pbData)))
	return outBlob.bytes(), nil
}
