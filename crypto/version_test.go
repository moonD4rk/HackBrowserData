package crypto

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetectVersion(t *testing.T) {
	tests := []struct {
		name       string
		ciphertext []byte
		want       CipherVersion
	}{
		{"v10 prefix", []byte("v10" + "encrypted_data"), CipherV10},
		{"v11 prefix", []byte("v11" + "encrypted_data"), CipherV11},
		{"v12 prefix", []byte("v12" + "encrypted_data"), CipherV12},
		{"v20 prefix", []byte("v20" + "encrypted_data"), CipherV20},
		{"no prefix (DPAPI)", []byte{0x01, 0x00, 0x00, 0x00}, CipherDPAPI},
		{"short input", []byte{0x01, 0x02}, CipherDPAPI},
		{"empty input", []byte{}, CipherDPAPI},
		{"nil input", nil, CipherDPAPI},
		{"unknown prefix", []byte("xyz_data"), CipherDPAPI},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, DetectVersion(tt.ciphertext))
		})
	}
}

func Test_stripPrefix(t *testing.T) {
	tests := []struct {
		name       string
		ciphertext []byte
		want       []byte
	}{
		{"strips v10", []byte("v10PAYLOAD"), []byte("PAYLOAD")},
		{"strips v11", []byte("v11PAYLOAD"), []byte("PAYLOAD")},
		{"strips v12", []byte("v12PAYLOAD"), []byte("PAYLOAD")},
		{"strips v20", []byte("v20PAYLOAD"), []byte("PAYLOAD")},
		{"keeps DPAPI unchanged", []byte{0x01, 0x00, 0x00}, []byte{0x01, 0x00, 0x00}},
		{"keeps short unchanged", []byte{0x01}, []byte{0x01}},
		{"keeps nil unchanged", nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, stripPrefix(tt.ciphertext))
		})
	}
}
