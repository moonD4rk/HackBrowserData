// Package keyretriever owns the master-key acquisition chain shared by all Chromium variants (Chrome,
// Edge, Brave, Arc, Opera, Vivaldi, Yandex, …). The chain is built once per process and reused for
// every profile.
//
// Firefox and Safari do not route through this package — Firefox derives its own keys from key4.db via
// NSS PBE, and Safari reads InternetPassword records directly from login.keychain-db. Each browser
// package owns its own credential-acquisition strategy; see rfcs/006-key-retrieval-mechanisms.md §7 for
// the rationale.
package keyretriever

import (
	"errors"
	"fmt"

	"github.com/moond4rk/hackbrowserdata/log"
)

// errStorageNotFound is returned when the requested browser storage account is not found in the
// credential store (keychain, keyring, etc.). Only used on darwin and linux; Windows uses DPAPI which
// has no storage lookup.
var errStorageNotFound = errors.New("not found in credential store") //nolint:unused // only used on darwin and linux

// Hints bundles inputs for KeyRetriever; each retriever reads only the field that applies to it.
type Hints struct {
	KeychainLabel  string // macOS Keychain account / Linux D-Bus Secret Service label
	WindowsABEKey  string // Windows ABE browser key (e.g. "chrome"); "" → ABE not applicable
	LocalStatePath string // path to (temp-copied) Local State JSON; only used on Windows
}

// KeyRetriever retrieves the master encryption key for a Chromium-based browser.
type KeyRetriever interface {
	RetrieveKey(hints Hints) ([]byte, error)
}

// ChainRetriever tries multiple retrievers in order, returning the first success. Used on macOS
// (gcoredump → password → security) and Linux (D-Bus → peanuts).
type ChainRetriever struct {
	retrievers []KeyRetriever
}

// NewChain creates a ChainRetriever that tries each retriever in order.
func NewChain(retrievers ...KeyRetriever) KeyRetriever {
	return &ChainRetriever{retrievers: retrievers}
}

func (c *ChainRetriever) RetrieveKey(hints Hints) ([]byte, error) {
	var errs []error
	for _, r := range c.retrievers {
		key, err := r.RetrieveKey(hints)
		if err == nil && len(key) > 0 {
			return key, nil
		}
		if err != nil {
			log.Debugf("keyretriever %T failed: %v", r, err)
			errs = append(errs, fmt.Errorf("%T: %w", r, err))
		}
	}
	return nil, fmt.Errorf("all retrievers failed: %w", errors.Join(errs...))
}
