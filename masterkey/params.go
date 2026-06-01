//go:build darwin || linux

package masterkey

import (
	"hash"

	"github.com/moond4rk/hackbrowserdata/crypto"
)

// pbkdf2Params holds platform-specific PBKDF2 parameters (each platform file defines its own).
type pbkdf2Params struct {
	salt       []byte
	iterations int
	keySize    int
	hashFunc   func() hash.Hash
}

func (p pbkdf2Params) deriveKey(secret []byte) []byte {
	return crypto.PBKDF2Key(secret, p.salt, p.iterations, p.keySize, p.hashFunc)
}
