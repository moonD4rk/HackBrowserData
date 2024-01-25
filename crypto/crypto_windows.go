//go:build windows

package crypto

import (
	"fmt"
	"syscall"
	"unsafe"
)

const (
	// Assuming the nonce size is 12 bytes and the minimum encrypted data size is 3 bytes
	minEncryptedDataSize = 15
	nonceSize            = 12
)

func DecryptWithChromium(key, ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < minEncryptedDataSize {
		return nil, ErrCiphertextLengthIsInvalid
	}

	nonce := ciphertext[3 : 3+nonceSize]
	encryptedPassword := ciphertext[3+nonceSize:]

	return AESGCMDecrypt(key, nonce, encryptedPassword)
}

// DecryptWithYandex decrypts the password with AES-GCM
func DecryptWithYandex(key, ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < minEncryptedDataSize {
		return nil, ErrCiphertextLengthIsInvalid
	}
	// remove Prefix 'v10'
	// gcmBlockSize         = 16
	// gcmTagSize           = 16
	// gcmMinimumTagSize    = 12 // NIST SP 800-38D recommends tags with 12 or more bytes.
	// gcmStandardNonceSize = 12
	nonce := ciphertext[3 : 3+nonceSize]
	encryptedPassword := ciphertext[3+nonceSize:]
	return AESGCMDecrypt(key, nonce, encryptedPassword)
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

// DecryptWithDPAPI (Data Protection Application Programming Interface)
// is a simple cryptographic application programming interface
// available as a built-in component in Windows 2000 and
// later versions of Microsoft Windows operating systems
func DecryptWithDPAPI(ciphertext []byte) ([]byte, error) {
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
