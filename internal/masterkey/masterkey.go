// Package masterkey retrieves Chromium master keys (per-platform retrievers + a cross-host Dump format).
// Firefox and Safari own their own key paths and don't route through here.
package masterkey

import (
	"context"
	"errors"
	"fmt"

	"github.com/moond4rk/hackbrowserdata/internal/log"
)

// Version is a Chromium ciphertext's version prefix.
type Version string

const (
	V10 Version = "v10"
	V11 Version = "v11"
	V20 Version = "v20"
)

// Set holds one user data dir's key per version; encoding/json writes it as keys.json's "keys" object.
type Set map[Version][]byte

// Retriever obtains one version's key from one OS secret store.
type Retriever interface {
	Version() Version
	Retrieve(ctx context.Context, loc Locator) ([]byte, error)
}

// ErrNotApplicable is a retriever's way of stepping aside; it never counts as a failure.
var ErrNotApplicable = errors.New("retriever not applicable")

// errStorageNotFound: the browser's account is absent from the credential store (keychain/keyring).
var errStorageNotFound = errors.New("not found in credential store") //nolint:unused // only used on darwin and linux

// Locator is where one user data dir's keys are found; each retriever reads only the fields of its OS.
type Locator struct {
	LocalState    string // the copied Local State; "" when the user data dir has none
	KeychainLabel string // macOS Keychain account / Linux D-Bus Secret Service label
	Exe           string
	ExeFallbacks  []string
	ABE           ABEKind
	Keychain      LoginKeychain // macOS: the run's login keychain; nil elsewhere
}

// LoginKeychain is the one method the macOS v10 retriever reads from the run's login keychain.
type LoginKeychain interface {
	GenericPassword(ctx context.Context, account string) ([]byte, error)
}

// ABEKind is the payload's elevation-service dialect; the zero ABENone leaves a browser on DPAPI alone.
type ABEKind int

const (
	ABENone ABEKind = iota
	ABEChromeBase
	ABEEdge
	ABEAvast
)

// RetrieveAll runs this OS's retrievers in order. Per version the first key wins and later retrievers are not
// tried; a version's error is reported only when all its retrievers fail, and a v20 failure never drops the v10 key.
func RetrieveAll(ctx context.Context, loc Locator) (Set, error) {
	return retrieveAll(ctx, loc, platformRetrievers)
}

func retrieveAll(ctx context.Context, loc Locator, retrievers []Retriever) (Set, error) {
	keys := make(Set)
	var versions []Version
	failures := make(map[Version][]error)
	for _, r := range retrievers {
		v := r.Version()
		if _, ok := keys[v]; ok {
			continue
		}
		if _, ok := failures[v]; !ok {
			versions = append(versions, v)
			failures[v] = nil
		}
		key, err := r.Retrieve(ctx, loc)
		switch {
		case errors.Is(err, ErrNotApplicable):
		case err != nil:
			log.Debugf("%s retriever %T failed: %v", v, r, err)
			failures[v] = append(failures[v], err)
		case len(key) > 0:
			keys[v] = key
		}
	}

	var errs []error
	for _, v := range versions {
		if _, ok := keys[v]; !ok && len(failures[v]) > 0 {
			errs = append(errs, fmt.Errorf("%s: %w", v, errors.Join(failures[v]...)))
		}
	}
	return keys, errors.Join(errs...)
}
