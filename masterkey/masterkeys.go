package masterkey

import (
	"errors"
	"fmt"
)

// MasterKeys holds one key per cipher tier; a profile can mix tiers (Win v10+v20, Linux v10+v11),
// so each is populated independently. A nil tier = that cipher version can't be decrypted.
type MasterKeys struct {
	V10 []byte `json:"v10,omitempty"`
	V11 []byte `json:"v11,omitempty"`
	V20 []byte `json:"v20,omitempty"`
}

func (k MasterKeys) HasAny() bool {
	return k.V10 != nil || k.V11 != nil || k.V20 != nil
}

// Retrievers is the per-tier retriever configuration; unused slots are nil.
type Retrievers struct {
	V10 Retriever
	V11 Retriever
	V20 Retriever
}

// NewMasterKeys fetches each non-nil tier and joins per-tier errors. A retriever returning (nil, nil)
// means "tier not applicable" and contributes no key. Never logs — the caller decides severity.
func NewMasterKeys(r Retrievers, hints Hints) (MasterKeys, error) {
	var keys MasterKeys
	var errs []error

	for _, t := range []struct {
		name string
		r    Retriever
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
