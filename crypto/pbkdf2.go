package crypto

import (
	"crypto/hmac"
	"hash"
)

// PBKDF2Key derives a key from the password, salt and iteration count, returning a
// []byte of length keylen that can be used as cryptographic key. The key is
// derived based on the method described as PBKDF2 with the HMAC variant using
// the supplied hash function.
//
// For example, to use a HMAC-SHA-1 based PBKDF2 key derivation function, you
// can get a derived key for e.g. AES-256 (which needs a 32-byte key) by
// doing:
//
//	dk := pbkdf2.Key([]byte("some password"), salt, 4096, 32, sha1.New)
//
// Remember to get a good random salt. At least 8 bytes is recommended by the
// RFC.
//
// Using a higher iteration count will increase the cost of an exhaustive
// search but will also make derivation proportionally slower.
// Copy from https://golang.org/x/crypto/pbkdf2
func PBKDF2Key(password, salt []byte, iter, keyLen int, h func() hash.Hash) []byte {
	prf := hmac.New(h, password)
	hashLen := prf.Size()
	numBlocks := (keyLen + hashLen - 1) / hashLen

	var buf [4]byte
	dk := make([]byte, 0, numBlocks*hashLen)
	u := make([]byte, hashLen)
	for block := 1; block <= numBlocks; block++ {
		// N.B.: || means concatenation, ^ means XOR
		// for each block T_i = U_1 ^ U_2 ^ ... ^ U_iter
		// U_1 = PRF(password, salt || uint(i))
		prf.Reset()
		prf.Write(salt)
		buf[0] = byte(block >> 24)
		buf[1] = byte(block >> 16)
		buf[2] = byte(block >> 8)
		buf[3] = byte(block)
		prf.Write(buf[:4])
		dk = prf.Sum(dk)
		t := dk[len(dk)-hashLen:]
		copy(u, t)

		for n := 2; n <= iter; n++ {
			prf.Reset()
			prf.Write(u)
			u = u[:0]
			u = prf.Sum(u)
			for x := range u {
				t[x] ^= u[x]
			}
		}
	}
	return dk[:keyLen]
}
