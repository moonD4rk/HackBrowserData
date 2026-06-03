//go:build linux

package crypto

func DecryptDPAPI(_ []byte) ([]byte, error) {
	return nil, errDPAPINotSupported
}
