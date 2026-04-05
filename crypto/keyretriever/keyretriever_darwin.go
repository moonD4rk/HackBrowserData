//go:build darwin

package keyretriever

import (
	"bytes"
	"context"
	"crypto/sha1"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/moond4rk/keychainbreaker"
	"golang.org/x/term"
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
// The result is cached via sync.Once to avoid repeated memory dumps
// when multiple profiles share the same retriever instance.
type GcoredumpRetriever struct {
	once sync.Once
	key  []byte
	err  error
}

func (r *GcoredumpRetriever) RetrieveKey(storage, _ string) ([]byte, error) {
	r.once.Do(func() {
		r.key, r.err = r.retrieveKeyOnce(storage)
	})
	return r.key, r.err
}

func (r *GcoredumpRetriever) retrieveKeyOnce(storage string) ([]byte, error) {
	secret, err := DecryptKeychain(storage)
	if err != nil {
		return nil, fmt.Errorf("gcoredump: %w", err)
	}
	if secret == "" {
		return nil, fmt.Errorf("gcoredump: empty secret for %s", storage)
	}
	return darwinParams.deriveKey([]byte(secret)), nil
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

// TerminalPasswordRetriever prompts for the keychain password interactively
// via the terminal using golang.org/x/term (with echo disabled).
// Automatically skipped when stdin is not a TTY.
type TerminalPasswordRetriever struct {
	once    sync.Once
	records []keychainbreaker.GenericPassword
	err     error
}

func (r *TerminalPasswordRetriever) RetrieveKey(storage, _ string) ([]byte, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return nil, nil
	}

	r.once.Do(func() {
		fmt.Fprintf(os.Stderr, "Enter macOS login password for %s: ", storage)
		pwd, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			r.err = fmt.Errorf("terminal: read password: %w", err)
			return
		}
		r.records, r.err = loadKeychainRecords(string(pwd))
	})
	if r.err != nil {
		return nil, r.err
	}

	return findStorageKey(r.records, storage)
}

// SecurityCmdRetriever uses macOS `security` CLI to query Keychain.
// This may trigger a password dialog on macOS. The result is cached
// via sync.Once so that multiple profiles sharing the same retriever
// instance only prompt the user once.
type SecurityCmdRetriever struct {
	once sync.Once
	key  []byte
	err  error
}

func (r *SecurityCmdRetriever) RetrieveKey(storage, _ string) ([]byte, error) {
	r.once.Do(func() {
		r.key, r.err = r.retrieveKeyOnce(storage)
	})
	return r.key, r.err
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

// DefaultRetriever returns the macOS retriever chain.
// The chain tries each method in order until one succeeds:
//  1. GcoredumpRetriever — CVE-2025-24204 exploit (root only, non-interactive)
//  2. KeychainPasswordRetriever — direct unlock with --keychain-pw flag
//  3. TerminalPasswordRetriever — interactive password prompt via terminal
//  4. SecurityCmdRetriever — security CLI fallback (may trigger system dialog)
func DefaultRetriever(keychainPassword string) KeyRetriever {
	retrievers := []KeyRetriever{
		&GcoredumpRetriever{},
	}
	if keychainPassword != "" {
		retrievers = append(retrievers, &KeychainPasswordRetriever{Password: keychainPassword})
	}
	retrievers = append(retrievers,
		&TerminalPasswordRetriever{},
		&SecurityCmdRetriever{},
	)
	return NewChain(retrievers...)
}
