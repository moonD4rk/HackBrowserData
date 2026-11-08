package crypto

import "errors"

var (
	errShortCiphertext     = errors.New("ciphertext too short")
	errInvalidBlockSize    = errors.New("ciphertext is not a multiple of the block size")
	errInvalidIVLength     = errors.New("IV length must equal block size")
	errInvalidPadding      = errors.New("invalid PKCS5 padding")
	errInvalidNonceLength  = errors.New("nonce length must equal GCM nonce size")
	errUnsupportedIVLength = errors.New("unsupported IV length")
	errShortKey            = errors.New("key is shorter than the cipher needs")
	errDecodeASN1          = errors.New("failed to decode ASN1 data")
)
