//go:build windows

package crypto

import (
	"encoding/hex"
	"fmt"
	"sync"
)

var (
	abeCLIKeyMu sync.RWMutex
	abeCLIKey   []byte
)

func SetABEMasterKeyFromHex(hexKey string) error {
	if hexKey == "" {
		return fmt.Errorf("abe: empty hex key")
	}
	b, err := hex.DecodeString(hexKey)
	if err != nil {
		return fmt.Errorf("abe: decode hex key: %w", err)
	}
	if len(b) != 32 {
		return fmt.Errorf("abe: key must be 32 bytes (got %d)", len(b))
	}
	abeCLIKeyMu.Lock()
	abeCLIKey = b
	abeCLIKeyMu.Unlock()
	return nil
}

func GetABEMasterKey() []byte {
	abeCLIKeyMu.RLock()
	defer abeCLIKeyMu.RUnlock()
	if len(abeCLIKey) == 0 {
		return nil
	}
	out := make([]byte, len(abeCLIKey))
	copy(out, abeCLIKey)
	return out
}

func ABEPayload(arch string) ([]byte, error) {
	return getPayloadForArch(arch)
}
