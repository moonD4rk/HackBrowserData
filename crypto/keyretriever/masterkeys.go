package keyretriever

import (
	"errors"
	"fmt"
)

// MasterKeys holds per-cipher-version Chromium master keys. A profile may carry mixed prefixes
// (Chrome 127+ on Windows mixes v10+v20; Linux can mix v10+v11), so each tier must be populated
// independently for lossless decryption. A nil tier means that cipher version cannot be decrypted.
type MasterKeys struct {
	V10 []byte
	V11 []byte
	V20 []byte
}

// Retrievers is the per-tier retriever configuration; unused slots are nil.
type Retrievers struct {
	V10 KeyRetriever
	V11 KeyRetriever
	V20 KeyRetriever
}

// NewMasterKeys fetches each non-nil tier in r and returns the assembled MasterKeys with per-tier
// errors joined. A retriever returning (nil, nil) signals "not applicable" and contributes no key
// silently. This function never logs; the caller decides severity.
func NewMasterKeys(r Retrievers, hints Hints) (MasterKeys, error) {
	var keys MasterKeys
	var errs []error

	for _, t := range []struct {
		name string
		r    KeyRetriever
		dst  *[]byte
	}{
		{"v10", r.V10, &keys.V10},
		{"v11", r.V11, &keys.V11},
		{"v20", r.V20, &keys.V20},
	} {
		if t.r == nil {
			continue
		}
		k, err := t.r.RetrieveKey(hints)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", t.name, err))
			continue
		}
		*t.dst = k
	}
	return keys, errors.Join(errs...)
}
