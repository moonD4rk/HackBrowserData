package keyretriever

// StaticProvider returns pre-supplied master-key bytes; used by cross-host workflows where keys come
// from a Dump rather than platform-native retrieval. RetrieveKey ignores Hints and returns the stored
// bytes verbatim; an empty StaticProvider returns (nil, nil), the "not applicable" signal accepted
// by NewMasterKeys when a tier was not present in the source Dump.
type StaticProvider struct {
	key []byte
}

// NewStaticProvider wraps key bytes as a KeyRetriever. A nil/empty key produces a provider that
// reports the tier as unavailable (nil, nil) rather than returning a zero-length key.
func NewStaticProvider(key []byte) *StaticProvider {
	return &StaticProvider{key: key}
}

// RetrieveKey returns the stored key bytes, ignoring Hints.
func (p *StaticProvider) RetrieveKey(_ Hints) ([]byte, error) {
	if len(p.key) == 0 {
		return nil, nil
	}
	return p.key, nil
}
