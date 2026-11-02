// Package masterkey retrieves Chromium master keys; Firefox and Safari keep their own key paths.
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

// RetrieveAll keeps the first key per version and reports an error only when every retriever of that version fails.
func RetrieveAll(ctx context.Context, locator Locator) (Set, error) {
	return retrieveAll(ctx, locator, platformRetrievers)
}

func retrieveAll(ctx context.Context, locator Locator, retrievers []Retriever) (Set, error) {
	keys := make(Set)
	var versions []Version
	failures := make(map[Version][]error)
	for _, retriever := range retrievers {
		version := retriever.Version()
		if _, ok := keys[version]; ok {
			continue
		}
		if _, ok := failures[version]; !ok {
			versions = append(versions, version)
			failures[version] = nil
		}
		key, err := retriever.Retrieve(ctx, locator)
		switch {
		case errors.Is(err, ErrNotApplicable):
		case err != nil:
			log.Debugf("%s retriever %T failed: %v", version, retriever, err)
			failures[version] = append(failures[version], err)
		case len(key) > 0:
			keys[version] = key
		}
	}

	var errs []error
	for _, version := range versions {
		if _, ok := keys[version]; !ok && len(failures[version]) > 0 {
			errs = append(errs, fmt.Errorf("%s: %w", version, errors.Join(failures[version]...)))
		}
	}
	return keys, errors.Join(errs...)
}
