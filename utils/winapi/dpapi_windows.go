//go:build windows

package winapi

import (
	"fmt"
	"unsafe"
)

var (
	procCryptUnprotectData = Crypt32.NewProc("CryptUnprotectData")
	procLocalFree          = Kernel32.NewProc("LocalFree")
)

// dataBlob mirrors the DPAPI DATA_BLOB struct.
type dataBlob struct {
	cbData uint32
	pbData *byte
}

func newBlob(d []byte) *dataBlob {
	if len(d) == 0 {
		return &dataBlob{}
	}
	return &dataBlob{pbData: &d[0], cbData: uint32(len(d))}
}

func (b *dataBlob) bytes() []byte {
	d := make([]byte, b.cbData)
	copy(d, (*[1 << 30]byte)(unsafe.Pointer(b.pbData))[:])
	return d
}

// DecryptDPAPI decrypts a DPAPI-protected blob using the current user's
// master key. It is the Windows counterpart to macOS/Linux os_crypt
// fallbacks and is called by crypto.DecryptDPAPI.
func DecryptDPAPI(ciphertext []byte) ([]byte, error) {
	var out dataBlob
	r, _, err := procCryptUnprotectData.Call(
		uintptr(unsafe.Pointer(newBlob(ciphertext))),
		0, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(&out)),
	)
	if r == 0 {
		return nil, fmt.Errorf("CryptUnprotectData: %w", err)
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(out.pbData)))
	return out.bytes(), nil
}
