package crypto

import (
	"bytes"
	"encoding/asn1"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	pbeIV                  = []byte("01234567") // 8 bytes
	pbePlaintext           = []byte("Hello, World!")
	pbeKeyCheck            = []byte{0xf8, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x1}
	objWithMD5AndDESCBC    = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 5, 3}
	objWithSHA256AndAES    = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 1, 46}
	objWithSHA1AndAES      = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 5, 13}
	privateKeyPBETestCases = []struct {
		RawHexPBE        string
		GlobalSalt       []byte
		Encrypted        []byte
		IterationCount   int
		KeyLen           int
		Plaintext        []byte
		ObjectIdentifier asn1.ObjectIdentifier
	}{
		{
			RawHexPBE:        "303e302a06092a864886f70d01050d301d04186d6f6f6e6434726b6d6f6f6e6434726b6d6f6f6e6434726b020114041095183a14c752e7b1d0aaa47f53e05097",
			GlobalSalt:       bytes.Repeat([]byte(baseKey), 3),
			Encrypted:        []byte{0x95, 0x18, 0x3a, 0x14, 0xc7, 0x52, 0xe7, 0xb1, 0xd0, 0xaa, 0xa4, 0x7f, 0x53, 0xe0, 0x50, 0x97},
			Plaintext:        pbePlaintext,
			IterationCount:   1,
			KeyLen:           32,
			ObjectIdentifier: objWithSHA1AndAES,
		},
	}
	passwordCheckPBETestCases = []struct {
		RawHexPBE        string
		GlobalSalt       []byte
		Encrypted        []byte
		IV               []byte
		Plaintext        []byte
		ObjectIdentifier asn1.ObjectIdentifier
	}{
		{
			RawHexPBE:        "307a3066060960864801650304012e3059303a060960864801650304012e302d04186d6f6f6e6434726b6d6f6f6e6434726b6d6f6f6e6434726b020101020120300b060960864801650304012e301b060960864801650304012e040e303132333435363730313233343504100474679f2e6256518b7adb877beaa154",
			GlobalSalt:       bytes.Repeat([]byte(baseKey), 3),
			Encrypted:        []byte{0x4, 0x74, 0x67, 0x9f, 0x2e, 0x62, 0x56, 0x51, 0x8b, 0x7a, 0xdb, 0x87, 0x7b, 0xea, 0xa1, 0x54},
			IV:               bytes.Repeat(pbeIV, 2)[:14],
			Plaintext:        pbePlaintext,
			ObjectIdentifier: objWithSHA256AndAES,
		},
	}
	credentialPBETestCases = []struct {
		RawHexPBE        string
		GlobalSalt       []byte
		Encrypted        []byte
		IV               []byte
		Plaintext        []byte
		ObjectIdentifier asn1.ObjectIdentifier
	}{
		{
			RawHexPBE:        "303b0410f8000000000000000000000000000001301506092a864886f70d010503040830313233343536370410fe968b6565149114ea688defd6683e45303b0410f8000000000000000000000000000001301506092a864886f70d010503040830313233343536370410fe968b6565149114ea688defd6683e45303b0410f8000000000000000000000000000001301506092a864886f70d010503040830313233343536370410fe968b6565149114ea688defd6683e45",
			Encrypted:        []byte{0xfe, 0x96, 0x8b, 0x65, 0x65, 0x14, 0x91, 0x14, 0xea, 0x68, 0x8d, 0xef, 0xd6, 0x68, 0x3e, 0x45},
			GlobalSalt:       bytes.Repeat([]byte(baseKey), 3),
			IV:               pbeIV,
			Plaintext:        pbePlaintext,
			ObjectIdentifier: objWithMD5AndDESCBC,
		},
	}
)

func TestNewASN1PBE(t *testing.T) {
	for _, tc := range privateKeyPBETestCases {
		nssRaw, err := hex.DecodeString(tc.RawHexPBE)
		require.NoError(t, err)
		pbe, err := NewASN1PBE(nssRaw)
		require.NoError(t, err)
		privateKeyPBETC, ok := pbe.(privateKeyPBE)
		assert.True(t, ok)
		assert.Equal(t, privateKeyPBETC.Encrypted, tc.Encrypted)
		assert.Equal(t, privateKeyPBETC.AlgoAttr.SaltAttr.EntrySalt, tc.GlobalSalt)
		assert.Equal(t, 20, privateKeyPBETC.AlgoAttr.SaltAttr.KeyLen)
		assert.Equal(t, privateKeyPBETC.AlgoAttr.ObjectIdentifier, tc.ObjectIdentifier)
	}
}

func TestPrivateKeyPBE_Encrypt(t *testing.T) {
	for _, tc := range privateKeyPBETestCases {
		privateKeyPBETC := privateKeyPBE{
			Encrypted: tc.Encrypted,
			AlgoAttr: struct {
				asn1.ObjectIdentifier
				SaltAttr struct {
					EntrySalt []byte
					KeyLen    int
				}
			}{
				ObjectIdentifier: tc.ObjectIdentifier,
				SaltAttr: struct {
					EntrySalt []byte
					KeyLen    int
				}{
					EntrySalt: tc.GlobalSalt,
					KeyLen:    20,
				},
			},
		}
		encrypted, err := privateKeyPBETC.Encrypt(tc.GlobalSalt, tc.Plaintext)
		require.NoError(t, err)
		assert.NotEmpty(t, encrypted)
		assert.Equal(t, privateKeyPBETC.Encrypted, encrypted)
	}
}

func TestPrivateKeyPBE_Decrypt(t *testing.T) {
	for _, tc := range privateKeyPBETestCases {
		privateKeyPBETC := privateKeyPBE{
			Encrypted: tc.Encrypted,
			AlgoAttr: struct {
				asn1.ObjectIdentifier
				SaltAttr struct {
					EntrySalt []byte
					KeyLen    int
				}
			}{
				ObjectIdentifier: tc.ObjectIdentifier,
				SaltAttr: struct {
					EntrySalt []byte
					KeyLen    int
				}{
					EntrySalt: tc.GlobalSalt,
					KeyLen:    20,
				},
			},
		}
		decrypted, err := privateKeyPBETC.Decrypt(tc.GlobalSalt)
		require.NoError(t, err)
		assert.NotEmpty(t, decrypted)
		assert.Equal(t, pbePlaintext, decrypted)
	}
}

func TestNewASN1PBE_PasswordCheckPBE(t *testing.T) {
	for _, tc := range passwordCheckPBETestCases {
		metaRaw, err := hex.DecodeString(tc.RawHexPBE)
		require.NoError(t, err)
		pbe, err := NewASN1PBE(metaRaw)
		require.NoError(t, err)
		passwordCheckPBETC, ok := pbe.(passwordCheckPBE)
		assert.True(t, ok)
		assert.Equal(t, passwordCheckPBETC.Encrypted, tc.Encrypted)
		assert.Equal(t, passwordCheckPBETC.AlgoAttr.KDFParams.IVData.IV, tc.IV)
		assert.Equal(t, passwordCheckPBETC.AlgoAttr.KDFParams.IVData.ObjectIdentifier, objWithSHA256AndAES)
	}
}

func TestPasswordCheckPBE_Encrypt(t *testing.T) {
	for _, tc := range passwordCheckPBETestCases {
		passwordCheckPBETC := passwordCheckPBE{
			AlgoAttr: algoAttr{
				ObjectIdentifier: tc.ObjectIdentifier,
				KDFParams: struct {
					PBKDF2 struct {
						asn1.ObjectIdentifier
						SaltAttr saltAttr
					}
					IVData ivAttr
				}{
					PBKDF2: struct {
						asn1.ObjectIdentifier
						SaltAttr saltAttr
					}{
						ObjectIdentifier: tc.ObjectIdentifier,
						SaltAttr: saltAttr{
							EntrySalt:      tc.GlobalSalt,
							IterationCount: 1,
							KeySize:        32,
							Algorithm: struct {
								asn1.ObjectIdentifier
							}{
								ObjectIdentifier: tc.ObjectIdentifier,
							},
						},
					},
					IVData: ivAttr{
						ObjectIdentifier: tc.ObjectIdentifier,
						IV:               tc.IV,
					},
				},
			},
			Encrypted: tc.Encrypted,
		}
		encrypted, err := passwordCheckPBETC.Encrypt(tc.GlobalSalt, tc.Plaintext)
		require.NoError(t, err)
		assert.NotEmpty(t, encrypted)
		assert.Equal(t, passwordCheckPBETC.Encrypted, encrypted)
	}
}

func TestPasswordCheckPBE_Decrypt(t *testing.T) {
	for _, tc := range passwordCheckPBETestCases {
		passwordCheckPBETC := passwordCheckPBE{
			AlgoAttr: algoAttr{
				ObjectIdentifier: tc.ObjectIdentifier,
				KDFParams: struct {
					PBKDF2 struct {
						asn1.ObjectIdentifier
						SaltAttr saltAttr
					}
					IVData ivAttr
				}{
					PBKDF2: struct {
						asn1.ObjectIdentifier
						SaltAttr saltAttr
					}{
						ObjectIdentifier: tc.ObjectIdentifier,
						SaltAttr: saltAttr{
							EntrySalt:      tc.GlobalSalt,
							IterationCount: 1,
							KeySize:        32,
							Algorithm: struct {
								asn1.ObjectIdentifier
							}{
								ObjectIdentifier: tc.ObjectIdentifier,
							},
						},
					},
					IVData: ivAttr{
						ObjectIdentifier: tc.ObjectIdentifier,
						IV:               tc.IV,
					},
				},
			},
			Encrypted: tc.Encrypted,
		}
		decrypted, err := passwordCheckPBETC.Decrypt(tc.GlobalSalt)
		require.NoError(t, err)
		assert.NotEmpty(t, decrypted)
		assert.Equal(t, pbePlaintext, decrypted)
	}
}

func TestNewASN1PBE_CredentialPBE(t *testing.T) {
	for _, tc := range credentialPBETestCases {
		loginRaw, err := hex.DecodeString(tc.RawHexPBE)
		require.NoError(t, err)
		pbe, err := NewASN1PBE(loginRaw)
		require.NoError(t, err)
		credentialPBETC, ok := pbe.(credentialPBE)
		assert.True(t, ok)
		assert.Equal(t, credentialPBETC.Encrypted, tc.Encrypted)
		assert.Equal(t, credentialPBETC.Algo.IV, tc.IV)
		assert.Equal(t, credentialPBETC.Algo.ObjectIdentifier, objWithMD5AndDESCBC)
	}
}

func TestCredentialPBE_Encrypt(t *testing.T) {
	for _, tc := range credentialPBETestCases {
		credentialPBETC := credentialPBE{
			KeyCheck: pbeKeyCheck,
			Algo: struct {
				asn1.ObjectIdentifier
				IV []byte
			}{
				ObjectIdentifier: tc.ObjectIdentifier,
				IV:               tc.IV,
			},
			Encrypted: tc.Encrypted,
		}
		encrypted, err := credentialPBETC.Encrypt(tc.GlobalSalt, plainText)
		require.NoError(t, err)
		assert.NotEmpty(t, encrypted)
		assert.Equal(t, credentialPBETC.Encrypted, encrypted)
	}
}

func TestCredentialPBE_Decrypt(t *testing.T) {
	for _, tc := range credentialPBETestCases {
		credentialPBETC := credentialPBE{
			KeyCheck: pbeKeyCheck,
			Algo: struct {
				asn1.ObjectIdentifier
				IV []byte
			}{
				ObjectIdentifier: tc.ObjectIdentifier,
				IV:               tc.IV,
			},
			Encrypted: tc.Encrypted,
		}
		decrypted, err := credentialPBETC.Decrypt(tc.GlobalSalt)
		require.NoError(t, err)
		assert.NotEmpty(t, decrypted)
		assert.Equal(t, pbePlaintext, decrypted)
	}
}

func TestNewASN1PBE_InvalidData(t *testing.T) {
	_, err := NewASN1PBE([]byte{0xFF, 0xFF})
	assert.ErrorIs(t, err, errDecodeASN1)
}

func TestCredentialPBE_AES256CBC(t *testing.T) {
	// Test the Firefox 144+ AES-256-CBC path (IV length = 16).
	// Construct a credentialPBE with a 16-byte IV to exercise the AES branch.
	masterKey := bytes.Repeat([]byte("k"), 32) // AES-256 key
	iv := bytes.Repeat([]byte{0x01}, 16)       // 16-byte IV → AES-CBC path

	// Encrypt plaintext to get valid ciphertext for round-trip test.
	encrypted, err := AESCBCEncrypt(masterKey, iv, pbePlaintext)
	require.NoError(t, err)

	pbe := credentialPBE{
		KeyCheck: pbeKeyCheck,
		Algo: struct {
			asn1.ObjectIdentifier
			IV []byte
		}{
			ObjectIdentifier: objWithSHA256AndAES,
			IV:               iv,
		},
		Encrypted: encrypted,
	}

	decrypted, err := pbe.Decrypt(masterKey)
	require.NoError(t, err)
	assert.Equal(t, pbePlaintext, decrypted)

	// Verify encrypt round-trip
	reEncrypted, err := pbe.Encrypt(masterKey, pbePlaintext)
	require.NoError(t, err)
	assert.Equal(t, encrypted, reEncrypted)
}

func TestCredentialPBE_UnsupportedIVLength(t *testing.T) {
	pbe := credentialPBE{
		Algo: struct {
			asn1.ObjectIdentifier
			IV []byte
		}{
			IV: []byte{1, 2, 3}, // 3-byte IV: neither 8 nor 16
		},
		Encrypted: []byte("data"),
	}
	_, err := pbe.Decrypt([]byte("key"))
	require.ErrorIs(t, err, errUnsupportedIVLen)

	_, err = pbe.Encrypt([]byte("key"), []byte("data"))
	require.ErrorIs(t, err, errUnsupportedIVLen)
}
