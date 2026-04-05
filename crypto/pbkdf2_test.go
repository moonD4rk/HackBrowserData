package crypto

import (
	"crypto/sha1"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test vectors from RFC 6070 (PKCS #5 PBKDF2 with HMAC-SHA1).
// https://www.rfc-editor.org/rfc/rfc6070
func TestPBKDF2Key_RFC6070(t *testing.T) {
	tests := []struct {
		name       string
		password   string
		salt       string
		iterations int
		keyLen     int
		want       string
	}{
		{
			name:       "iteration=1",
			password:   "password",
			salt:       "salt",
			iterations: 1,
			keyLen:     20,
			want:       "0c60c80f961f0e71f3a9b524af6012062fe037a6",
		},
		{
			name:       "iteration=2",
			password:   "password",
			salt:       "salt",
			iterations: 2,
			keyLen:     20,
			want:       "ea6c014dc72d6f8ccd1ed92ace1d41f0d8de8957",
		},
		{
			name:       "iteration=4096",
			password:   "password",
			salt:       "salt",
			iterations: 4096,
			keyLen:     20,
			want:       "4b007901b765489abead49d926f721d065a429c1",
		},
		{
			name:       "long_password_and_salt",
			password:   "passwordPASSWORDpassword",
			salt:       "saltSALTsaltSALTsaltSALTsaltSALTsalt",
			iterations: 4096,
			keyLen:     25,
			want:       "3d2eec4fe41c849b80c8d83662c0e44a8b291a964cf2f07038",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PBKDF2Key([]byte(tt.password), []byte(tt.salt), tt.iterations, tt.keyLen, sha1.New)
			assert.Equal(t, tt.want, hex.EncodeToString(got))
		})
	}
}
