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
		Plaintext        []byte
		ObjectIdentifier asn1.ObjectIdentifier
	}{
		{
			RawHexPBE:        "303e302a06092a864886f70d01050d301d04186d6f6f6e6434726b6d6f6f6e6434726b6d6f6f6e6434726b020114041095183a14c752e7b1d0aaa47f53e05097",
			GlobalSalt:       bytes.Repeat([]byte(baseKey), 3),
			Encrypted:        []byte{0x95, 0x18, 0x3a, 0x14, 0xc7, 0x52, 0xe7, 0xb1, 0xd0, 0xaa, 0xa4, 0x7f, 0x53, 0xe0, 0x50, 0x97},
			Plaintext:        pbePlaintext,
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

func TestNewASN1PBE_PrivateKeyPBE(t *testing.T) {
	for _, testCase := range privateKeyPBETestCases {
		nssRaw, err := hex.DecodeString(testCase.RawHexPBE)
		require.NoError(t, err)
		pbe, err := NewASN1PBE(nssRaw)
		require.NoError(t, err)
		privateKeyPBETC, ok := pbe.(privateKeyPBE)
		assert.True(t, ok)
		assert.Equal(t, privateKeyPBETC.Encrypted, testCase.Encrypted)
		assert.Equal(t, privateKeyPBETC.Algorithm.Salt.EntrySalt, testCase.GlobalSalt)
		assert.Equal(t, 20, privateKeyPBETC.Algorithm.Salt.KeyLength)
		assert.Equal(t, privateKeyPBETC.Algorithm.ObjectIdentifier, testCase.ObjectIdentifier)
	}
}

func TestPrivateKeyPBE_Encrypt(t *testing.T) {
	for _, testCase := range privateKeyPBETestCases {
		privateKeyPBETC := privateKeyPBE{
			Encrypted: testCase.Encrypted,
			Algorithm: struct {
				asn1.ObjectIdentifier
				Salt struct {
					EntrySalt []byte
					KeyLength int
				}
			}{
				ObjectIdentifier: testCase.ObjectIdentifier,
				Salt: struct {
					EntrySalt []byte
					KeyLength int
				}{
					EntrySalt: testCase.GlobalSalt,
					KeyLength: 20,
				},
			},
		}
		encrypted, err := privateKeyPBETC.Encrypt(testCase.GlobalSalt, testCase.Plaintext)
		require.NoError(t, err)
		assert.NotEmpty(t, encrypted)
		assert.Equal(t, privateKeyPBETC.Encrypted, encrypted)
	}
}

func TestPrivateKeyPBE_Decrypt(t *testing.T) {
	for _, testCase := range privateKeyPBETestCases {
		privateKeyPBETC := privateKeyPBE{
			Encrypted: testCase.Encrypted,
			Algorithm: struct {
				asn1.ObjectIdentifier
				Salt struct {
					EntrySalt []byte
					KeyLength int
				}
			}{
				ObjectIdentifier: testCase.ObjectIdentifier,
				Salt: struct {
					EntrySalt []byte
					KeyLength int
				}{
					EntrySalt: testCase.GlobalSalt,
					KeyLength: 20,
				},
			},
		}
		decrypted, err := privateKeyPBETC.Decrypt(testCase.GlobalSalt)
		require.NoError(t, err)
		assert.NotEmpty(t, decrypted)
		assert.Equal(t, pbePlaintext, decrypted)
	}
}

func TestNewASN1PBE_PasswordCheckPBE(t *testing.T) {
	for _, testCase := range passwordCheckPBETestCases {
		metaRaw, err := hex.DecodeString(testCase.RawHexPBE)
		require.NoError(t, err)
		pbe, err := NewASN1PBE(metaRaw)
		require.NoError(t, err)
		passwordCheckPBETC, ok := pbe.(passwordCheckPBE)
		assert.True(t, ok)
		assert.Equal(t, passwordCheckPBETC.Encrypted, testCase.Encrypted)
		assert.Equal(t, passwordCheckPBETC.Algorithm.KDFParams.IVData.IV, testCase.IV)
		assert.Equal(t, passwordCheckPBETC.Algorithm.KDFParams.IVData.ObjectIdentifier, objWithSHA256AndAES)
	}
}

func TestPasswordCheckPBE_Encrypt(t *testing.T) {
	for _, testCase := range passwordCheckPBETestCases {
		passwordCheckPBETC := passwordCheckPBE{
			Algorithm: algorithmAttributes{
				ObjectIdentifier: testCase.ObjectIdentifier,
				KDFParams: struct {
					PBKDF2 struct {
						asn1.ObjectIdentifier
						Salt saltAttributes
					}
					IVData ivAttributes
				}{
					PBKDF2: struct {
						asn1.ObjectIdentifier
						Salt saltAttributes
					}{
						ObjectIdentifier: testCase.ObjectIdentifier,
						Salt: saltAttributes{
							EntrySalt:      testCase.GlobalSalt,
							IterationCount: 1,
							KeySize:        32,
							Algorithm: struct {
								asn1.ObjectIdentifier
							}{
								ObjectIdentifier: testCase.ObjectIdentifier,
							},
						},
					},
					IVData: ivAttributes{
						ObjectIdentifier: testCase.ObjectIdentifier,
						IV:               testCase.IV,
					},
				},
			},
			Encrypted: testCase.Encrypted,
		}
		encrypted, err := passwordCheckPBETC.Encrypt(testCase.GlobalSalt, testCase.Plaintext)
		require.NoError(t, err)
		assert.NotEmpty(t, encrypted)
		assert.Equal(t, passwordCheckPBETC.Encrypted, encrypted)
	}
}

func TestPasswordCheckPBE_Decrypt(t *testing.T) {
	for _, testCase := range passwordCheckPBETestCases {
		passwordCheckPBETC := passwordCheckPBE{
			Algorithm: algorithmAttributes{
				ObjectIdentifier: testCase.ObjectIdentifier,
				KDFParams: struct {
					PBKDF2 struct {
						asn1.ObjectIdentifier
						Salt saltAttributes
					}
					IVData ivAttributes
				}{
					PBKDF2: struct {
						asn1.ObjectIdentifier
						Salt saltAttributes
					}{
						ObjectIdentifier: testCase.ObjectIdentifier,
						Salt: saltAttributes{
							EntrySalt:      testCase.GlobalSalt,
							IterationCount: 1,
							KeySize:        32,
							Algorithm: struct {
								asn1.ObjectIdentifier
							}{
								ObjectIdentifier: testCase.ObjectIdentifier,
							},
						},
					},
					IVData: ivAttributes{
						ObjectIdentifier: testCase.ObjectIdentifier,
						IV:               testCase.IV,
					},
				},
			},
			Encrypted: testCase.Encrypted,
		}
		decrypted, err := passwordCheckPBETC.Decrypt(testCase.GlobalSalt)
		require.NoError(t, err)
		assert.NotEmpty(t, decrypted)
		assert.Equal(t, pbePlaintext, decrypted)
	}
}

func TestNewASN1PBE_CredentialPBE(t *testing.T) {
	for _, testCase := range credentialPBETestCases {
		loginRaw, err := hex.DecodeString(testCase.RawHexPBE)
		require.NoError(t, err)
		pbe, err := NewASN1PBE(loginRaw)
		require.NoError(t, err)
		credentialPBETC, ok := pbe.(credentialPBE)
		assert.True(t, ok)
		assert.Equal(t, credentialPBETC.Encrypted, testCase.Encrypted)
		assert.Equal(t, credentialPBETC.Algorithm.IV, testCase.IV)
		assert.Equal(t, credentialPBETC.Algorithm.ObjectIdentifier, objWithMD5AndDESCBC)
	}
}

func TestCredentialPBE_Encrypt(t *testing.T) {
	for _, testCase := range credentialPBETestCases {
		credentialPBETC := credentialPBE{
			KeyCheck: pbeKeyCheck,
			Algorithm: struct {
				asn1.ObjectIdentifier
				IV []byte
			}{
				ObjectIdentifier: testCase.ObjectIdentifier,
				IV:               testCase.IV,
			},
			Encrypted: testCase.Encrypted,
		}
		encrypted, err := credentialPBETC.Encrypt(testCase.GlobalSalt, testCase.Plaintext)
		require.NoError(t, err)
		assert.NotEmpty(t, encrypted)
		assert.Equal(t, credentialPBETC.Encrypted, encrypted)
	}
}

func TestCredentialPBE_Decrypt(t *testing.T) {
	for _, testCase := range credentialPBETestCases {
		credentialPBETC := credentialPBE{
			KeyCheck: pbeKeyCheck,
			Algorithm: struct {
				asn1.ObjectIdentifier
				IV []byte
			}{
				ObjectIdentifier: testCase.ObjectIdentifier,
				IV:               testCase.IV,
			},
			Encrypted: testCase.Encrypted,
		}
		decrypted, err := credentialPBETC.Decrypt(testCase.GlobalSalt)
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
	// Firefox 144+ path: a 16-byte IV selects AES-256-CBC.
	masterKey := bytes.Repeat([]byte("k"), 32)
	iv := bytes.Repeat([]byte{0x01}, 16)

	encrypted, err := AESCBCEncrypt(masterKey, iv, pbePlaintext)
	require.NoError(t, err)

	pbe := credentialPBE{
		KeyCheck: pbeKeyCheck,
		Algorithm: struct {
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

	reEncrypted, err := pbe.Encrypt(masterKey, pbePlaintext)
	require.NoError(t, err)
	assert.Equal(t, encrypted, reEncrypted)
}

func TestCredentialPBE_UnsupportedIVLength(t *testing.T) {
	pbe := credentialPBE{
		Algorithm: struct {
			asn1.ObjectIdentifier
			IV []byte
		}{
			IV: []byte{1, 2, 3}, // 3-byte IV: neither 8 nor 16
		},
		Encrypted: []byte("data"),
	}
	_, err := pbe.Decrypt([]byte("key"))
	require.ErrorIs(t, err, errUnsupportedIVLength)

	_, err = pbe.Encrypt([]byte("key"), []byte("data"))
	require.ErrorIs(t, err, errUnsupportedIVLength)
}

// TestCredentialPBE_ShortKey passes the nil key of a profile whose key could not be derived; it must fail, not panic.
func TestCredentialPBE_ShortKey(t *testing.T) {
	raw, err := hex.DecodeString(credentialPBETestCases[0].RawHexPBE)
	require.NoError(t, err)
	pbe, err := NewASN1PBE(raw)
	require.NoError(t, err)
	for _, key := range [][]byte{nil, make([]byte, des3KeySize-1)} {
		_, err := pbe.Decrypt(key)
		require.ErrorIs(t, err, errShortKey)
		_, err = pbe.Encrypt(key, pbePlaintext)
		require.ErrorIs(t, err, errShortKey)
	}
}
