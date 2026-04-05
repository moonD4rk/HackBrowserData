package crypto

import "errors"

// Sentinel errors for crypto operations.
var (
	errShortCiphertext   = errors.New("ciphertext too short")
	errInvalidBlockSize  = errors.New("ciphertext is not a multiple of the block size")
	errInvalidIVLength   = errors.New("IV length must equal block size")
	errInvalidPadding    = errors.New("invalid PKCS5 padding")
	errInvalidNonceLen   = errors.New("nonce length must equal GCM nonce size")
	errUnsupportedIVLen  = errors.New("unsupported IV length")
	errDecodeASN1        = errors.New("failed to decode ASN1 data")
	errDPAPINotSupported = errors.New("DPAPI not supported on this platform") //nolint:unused // used on darwin/linux only
)
