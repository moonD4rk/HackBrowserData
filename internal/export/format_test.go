package export

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFormat(t *testing.T) {
	tests := []struct {
		input string
		want  Format
	}{
		{"json", JSON},
		{"csv", CSV},
		{"cookie-editor", CookieEditor},
	}
	for _, testCase := range tests {
		got, err := ParseFormat(testCase.input)
		require.NoError(t, err)
		assert.Equal(t, testCase.want, got)
	}
	_, err := ParseFormat("unknown")
	require.EqualError(t, err, "unsupported format: unknown")
}

func TestFormat_ZeroIsJSON(t *testing.T) {
	var format Format
	assert.Equal(t, JSON, format)
}
