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

// darwinParams follow Chromium's os_crypt_mac.mm:
// https://source.chromium.org/chromium/chromium/src/+/master:components/os_crypt/os_crypt_mac.mm;l=157
var darwinParams = pbkdf2Params{
	salt:       []byte("saltysalt"),
	iterations: 1003,
	keySize:    16,
	hashFunc:   sha1.New,
}

const securityCmdTimeout = 30 * time.Second

// platformRetrievers stop at the first success; the security CLI goes last because it may prompt.
var platformRetrievers = []keyRetriever{
	&gcoredumpRetriever{},
	keychainRetriever{},
	&securityRetriever{cache: make(map[string]securityResult)},
}

// gcoredumpRetriever reads records once (sync.Once), so the dump runs a single time for all browsers.
type gcoredumpRetriever struct {
	once    sync.Once
	records []keychainbreaker.GenericPassword
	err     error
}

func (*gcoredumpRetriever) Version() Version { return V10 }

// Retrieve steps aside on failure: the common "needs root" case isn't warning-worthy and would drown real warnings.
func (r *gcoredumpRetriever) Retrieve(_ context.Context, locator Locator) ([]byte, error) {
	r.once.Do(func() {
		r.records, r.err = decryptKeychainRecords()
	})
	if r.err != nil {
		log.Debugf("gcoredump: %v", r.err)
		return nil, errNotApplicable
	}

	key, err := findStorageKey(r.records, locator.SecretLabel)
	if err != nil {
		log.Debugf("gcoredump: %v", err)
		return nil, errNotApplicable
	}
	return key, nil
}

func findStorageKey(records []keychainbreaker.GenericPassword, storage string) ([]byte, error) {
	for _, genericPassword := range records {
		if genericPassword.Account == storage {
			return darwinParams.deriveKey(genericPassword.Password), nil
		}
	}
	return nil, fmt.Errorf("%q: %w", storage, errStorageNotFound)
}

// keychainRetriever steps aside without a keychain or with a locked one; the keychain reports why once for the run.
type keychainRetriever struct{}

func (keychainRetriever) Version() Version { return V10 }

func (keychainRetriever) Retrieve(ctx context.Context, locator Locator) ([]byte, error) {
	if locator.Keychain == nil {
		return nil, errNotApplicable
	}
	secret, err := locator.Keychain.GenericPassword(ctx, locator.SecretLabel)
	if errors.Is(err, keychain.ErrLocked) {
		return nil, errNotApplicable
	}
	if err != nil {
		return nil, err
	}
	return darwinParams.deriveKey(secret), nil
}

// securityRetriever caches results per storage name, so each browser's key is fetched, and prompted for, once.
type securityRetriever struct {
	mutex sync.Mutex
	cache map[string]securityResult
}

type securityResult struct {
	key []byte
	err error
}

func (*securityRetriever) Version() Version { return V10 }

func (r *securityRetriever) Retrieve(ctx context.Context, locator Locator) ([]byte, error) {
	storage := locator.SecretLabel
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if cached, ok := r.cache[storage]; ok {
		return cached.key, cached.err
	}

	key, err := r.retrieveKeyOnce(ctx, storage)
	r.cache[storage] = securityResult{key: key, err: err}
	return key, err
}

func (r *securityRetriever) retrieveKeyOnce(ctx context.Context, storage string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, securityCmdTimeout)
	defer cancel()

	var stdout, stderr bytes.Buffer
	command := exec.CommandContext(ctx, "security", "find-generic-password", "-wa", strings.TrimSpace(storage)) //nolint:gosec
	command.Stdout = &stdout
	command.Stderr = &stderr

	if err := command.Run(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, fmt.Errorf("security command timed out after %s", securityCmdTimeout)
		}
		// `security` exits non-zero with empty stderr when the prompt is denied; say so instead of "exit status 128 ()".
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
