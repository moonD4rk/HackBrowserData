package crypto

import (
	"crypto/hmac"
	"hash"
)

// PBKDF2Key is adapted from golang.org/x/crypto/pbkdf2.
func PBKDF2Key(password, salt []byte, iterations, keyLength int, newHash func() hash.Hash) []byte {
	prf := hmac.New(newHash, password)
	hashLen := prf.Size()
	numBlocks := (keyLength + hashLen - 1) / hashLen

	var blockIndex [4]byte
	derivedKey := make([]byte, 0, numBlocks*hashLen)
	previous := make([]byte, hashLen)
	for block := 1; block <= numBlocks; block++ {
		prf.Reset()
		prf.Write(salt)
		blockIndex[0] = byte(block >> 24)
		blockIndex[1] = byte(block >> 16)
		blockIndex[2] = byte(block >> 8)
		blockIndex[3] = byte(block)
		prf.Write(blockIndex[:4])
		derivedKey = prf.Sum(derivedKey)
		lastBlock := derivedKey[len(derivedKey)-hashLen:]
		copy(previous, lastBlock)

		for iteration := 2; iteration <= iterations; iteration++ {
			prf.Reset()
			prf.Write(previous)
			previous = previous[:0]
			previous = prf.Sum(previous)
			for byteIndex := range previous {
				lastBlock[byteIndex] ^= previous[byteIndex]
			}
		}
	}
	return derivedKey[:keyLength]
}
