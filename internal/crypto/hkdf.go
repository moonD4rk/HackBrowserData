package crypto

import (
	"crypto/hmac"
	"hash"
)

// HKDFKey is the extract-and-expand HKDF, written out because Go 1.20 has no crypto/hkdf and the project adds no
// x/crypto dependency; an empty salt means a block of newHash-sized zeros, as the construction specifies.
func HKDFKey(secret, salt, info []byte, keyLength int, newHash func() hash.Hash) []byte {
	if len(salt) == 0 {
		salt = make([]byte, newHash().Size())
	}
	extractor := hmac.New(newHash, salt)
	extractor.Write(secret)
	pseudoRandomKey := extractor.Sum(nil)

	expander := hmac.New(newHash, pseudoRandomKey)
	key := make([]byte, 0, keyLength)
	var block []byte
	for counter := byte(1); len(key) < keyLength; counter++ {
		expander.Reset()
		expander.Write(block)
		expander.Write(info)
		expander.Write([]byte{counter})
		block = expander.Sum(nil)
		key = append(key, block...)
	}
	return key[:keyLength]
}
