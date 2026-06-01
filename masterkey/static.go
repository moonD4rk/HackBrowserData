package masterkey

// StaticRetriever returns pre-supplied key bytes (from a Dump) instead of platform retrieval, ignoring
// Hints. An empty key returns (nil, nil) — the "tier not applicable" signal NewMasterKeys expects.
type StaticRetriever struct {
	key []byte
}

// NewStaticRetriever wraps key bytes; a nil/empty key yields a retriever that reports the tier unavailable.
func NewStaticRetriever(key []byte) *StaticRetriever {
	return &StaticRetriever{key: key}
}

func (p *StaticRetriever) RetrieveKey(_ Hints) ([]byte, error) {
	if len(p.key) == 0 {
		return nil, nil
	}
	return p.key, nil
}
