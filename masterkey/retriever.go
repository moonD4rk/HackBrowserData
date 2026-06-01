// Package masterkey retrieves Chromium master keys (per-platform retrievers + a cross-host Dump format).
// Firefox and Safari own their own key paths and don't route through here.
package masterkey

import (
	"errors"
	"fmt"

	"github.com/moond4rk/hackbrowserdata/log"
)

// errStorageNotFound: the browser's account is absent from the credential store (keychain/keyring).
var errStorageNotFound = errors.New("not found in credential store") //nolint:unused // only used on darwin and linux

// Hints bundles inputs for Retriever; each retriever reads only the field that applies to it.
type Hints struct {
	KeychainLabel  string // macOS Keychain account / Linux D-Bus Secret Service label
	WindowsABEKey  string // Windows ABE browser key (e.g. "chrome"); "" → ABE not applicable
	LocalStatePath string // path to (temp-copied) Local State JSON; only used on Windows
}

// Retriever obtains a Chromium master key from one platform source (DPAPI, Keychain, D-Bus, …).
type Retriever interface {
	RetrieveKey(hints Hints) ([]byte, error)
}

// ChainRetriever tries retrievers in order, first success wins (macOS V10: gcoredump→password→security).
type ChainRetriever struct {
	retrievers []Retriever
}

func NewChain(retrievers ...Retriever) Retriever {
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
			log.Debugf("retriever %T failed: %v", r, err)
			errs = append(errs, fmt.Errorf("%T: %w", r, err))
		}
	}
	return nil, fmt.Errorf("all retrievers failed: %w", errors.Join(errs...))
}
