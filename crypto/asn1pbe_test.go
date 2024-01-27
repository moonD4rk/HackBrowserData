package crypto

import (
	"bytes"
	"encoding/asn1"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	pbeIV               = []byte("01234567") // 8 bytes
	pbePlaintext        = []byte("Hello, World!")
	pbeCipherText       = []byte{0xf8, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x1}
	objWithMD5AndDESCBC = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 5, 3}
	objWithSHA256AndAES = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 1, 46}
	objWithSHA1AndAES   = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 5, 13}
	nssPBETestCases     = []struct {
		RawHexPBE        string
		GlobalSalt       []byte
		Encrypted        []byte
		IterationCount   int
		Len              int
		Plaintext        []byte
		ObjectIdentifier asn1.ObjectIdentifier
	}{
		{
			RawHexPBE:        "303e302a06092a864886f70d01050d301d04186d6f6f6e6434726b6d6f6f6e6434726b6d6f6f6e6434726b020114041095183a14c752e7b1d0aaa47f53e05097",
			GlobalSalt:       bytes.Repeat([]byte(baseKey), 3),
			Encrypted:        []byte{0x95, 0x18, 0x3a, 0x14, 0xc7, 0x52, 0xe7, 0xb1, 0xd0, 0xaa, 0xa4, 0x7f, 0x53, 0xe0, 0x50, 0x97},
			Plaintext:        pbePlaintext,
			IterationCount:   1,
			Len:              32,
			ObjectIdentifier: objWithSHA1AndAES,
		},
	}
	metaPBETestCases = []struct {
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
	loginPBETestCases = []struct {
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
	for _, tc := range nssPBETestCases {
		nssRaw, err := hex.DecodeString(tc.RawHexPBE)
		assert.Equal(t, nil, err)
		pbe, err := NewASN1PBE(nssRaw)
		assert.Equal(t, nil, err)
		nssPBETC, ok := pbe.(nssPBE)
		assert.Equal(t, true, ok)
		assert.Equal(t, nssPBETC.Encrypted, tc.Encrypted)
		assert.Equal(t, nssPBETC.AlgoAttr.SaltAttr.EntrySalt, tc.GlobalSalt)
		assert.Equal(t, nssPBETC.AlgoAttr.SaltAttr.Len, 20)
		assert.Equal(t, nssPBETC.AlgoAttr.ObjectIdentifier, tc.ObjectIdentifier)
	}
}

func TestNssPBE_Encrypt(t *testing.T) {
	for _, tc := range nssPBETestCases {
		nssPBETC := nssPBE{
			Encrypted: tc.Encrypted,
			AlgoAttr: struct {
				asn1.ObjectIdentifier
				SaltAttr struct {
					EntrySalt []byte
					Len       int
				}
			}{
				ObjectIdentifier: tc.ObjectIdentifier,
				SaltAttr: struct {
					EntrySalt []byte
					Len       int
				}{
					EntrySalt: tc.GlobalSalt,
					Len:       20,
				},
			},
		}
		encrypted, err := nssPBETC.Encrypt(tc.GlobalSalt, tc.Plaintext)
		assert.Equal(t, nil, err)
		assert.Equal(t, true, len(encrypted) > 0)
		assert.Equal(t, nssPBETC.Encrypted, encrypted)
	}
}

func TestNssPBE_Decrypt(t *testing.T) {
	for _, tc := range nssPBETestCases {
		nssPBETC := nssPBE{
			Encrypted: tc.Encrypted,
			AlgoAttr: struct {
				asn1.ObjectIdentifier
				SaltAttr struct {
					EntrySalt []byte
					Len       int
				}
			}{
				ObjectIdentifier: tc.ObjectIdentifier,
				SaltAttr: struct {
					EntrySalt []byte
					Len       int
				}{
					EntrySalt: tc.GlobalSalt,
					Len:       20,
				},
			},
		}
		decrypted, err := nssPBETC.Decrypt(tc.GlobalSalt)
		assert.Equal(t, nil, err)
		assert.Equal(t, true, len(decrypted) > 0)
		assert.Equal(t, pbePlaintext, decrypted)
	}
}

func TestNewASN1PBE_MetaPBE(t *testing.T) {
	for _, tc := range metaPBETestCases {
		metaRaw, err := hex.DecodeString(tc.RawHexPBE)
		assert.Equal(t, nil, err)
		pbe, err := NewASN1PBE(metaRaw)
		assert.Equal(t, nil, err)
		metaPBETC, ok := pbe.(metaPBE)
		assert.Equal(t, true, ok)
		assert.Equal(t, metaPBETC.Encrypted, tc.Encrypted)
		assert.Equal(t, metaPBETC.AlgoAttr.Data.IVData.IV, tc.IV)
		assert.Equal(t, metaPBETC.AlgoAttr.Data.IVData.ObjectIdentifier, objWithSHA256AndAES)
	}
}

func TestMetaPBE_Encrypt(t *testing.T) {
	for _, tc := range metaPBETestCases {
		metaPBETC := metaPBE{
			AlgoAttr: algoAttr{
				ObjectIdentifier: tc.ObjectIdentifier,
				Data: struct {
					Data struct {
						asn1.ObjectIdentifier
						SlatAttr slatAttr
					}
					IVData ivAttr
				}{
					Data: struct {
						asn1.ObjectIdentifier
						SlatAttr slatAttr
					}{
						ObjectIdentifier: tc.ObjectIdentifier,
						SlatAttr: slatAttr{
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
		encrypted, err := metaPBETC.Encrypt(tc.GlobalSalt, tc.Plaintext)
		assert.Equal(t, nil, err)
		assert.Equal(t, true, len(encrypted) > 0)
		assert.Equal(t, metaPBETC.Encrypted, encrypted)
	}
}

func TestMetaPBE_Decrypt(t *testing.T) {
	for _, tc := range metaPBETestCases {
		metaPBETC := metaPBE{
			AlgoAttr: algoAttr{
				ObjectIdentifier: tc.ObjectIdentifier,
				Data: struct {
					Data struct {
						asn1.ObjectIdentifier
						SlatAttr slatAttr
					}
					IVData ivAttr
				}{
					Data: struct {
						asn1.ObjectIdentifier
						SlatAttr slatAttr
					}{
						ObjectIdentifier: tc.ObjectIdentifier,
						SlatAttr: slatAttr{
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
		decrypted, err := metaPBETC.Decrypt(tc.GlobalSalt)
		assert.Equal(t, nil, err)
		assert.Equal(t, true, len(decrypted) > 0)
		assert.Equal(t, pbePlaintext, decrypted)
	}
}

func TestNewASN1PBE_LoginPBE(t *testing.T) {
	for _, tc := range loginPBETestCases {
		loginRaw, err := hex.DecodeString(tc.RawHexPBE)
		assert.Equal(t, nil, err)
		pbe, err := NewASN1PBE(loginRaw)
		assert.Equal(t, nil, err)
		loginPBETC, ok := pbe.(loginPBE)
		assert.Equal(t, true, ok)
		assert.Equal(t, loginPBETC.Encrypted, tc.Encrypted)
		assert.Equal(t, loginPBETC.Data.IV, tc.IV)
		assert.Equal(t, loginPBETC.Data.ObjectIdentifier, objWithMD5AndDESCBC)
	}
}

func TestLoginPBE_Encrypt(t *testing.T) {
	for _, tc := range loginPBETestCases {
		loginPBETC := loginPBE{
			CipherText: pbeCipherText,
			Data: struct {
				asn1.ObjectIdentifier
				IV []byte
			}{
				ObjectIdentifier: tc.ObjectIdentifier,
				IV:               tc.IV,
			},
			Encrypted: tc.Encrypted,
		}
		encrypted, err := loginPBETC.Encrypt(tc.GlobalSalt, plainText)
		assert.Equal(t, nil, err)
		assert.Equal(t, true, len(encrypted) > 0)
		assert.Equal(t, loginPBETC.Encrypted, encrypted)
	}
}

func TestLoginPBE_Decrypt(t *testing.T) {
	for _, tc := range loginPBETestCases {
		loginPBETC := loginPBE{
			CipherText: pbeCipherText,
			Data: struct {
				asn1.ObjectIdentifier
				IV []byte
			}{
				ObjectIdentifier: tc.ObjectIdentifier,
				IV:               tc.IV,
			},
			Encrypted: tc.Encrypted,
		}
		decrypted, err := loginPBETC.Decrypt(tc.GlobalSalt)
		assert.Equal(t, nil, err)
		assert.Equal(t, true, len(decrypted) > 0)
		assert.Equal(t, pbePlaintext, decrypted)
	}
}
