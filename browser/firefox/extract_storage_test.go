package firefox

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupWebappsDB(t *testing.T) string {
	t.Helper()
	return createTestDB(t, "webappsstore.sqlite", []string{webappsstore2Schema},
		insertWebappsstore("moc.buhtig.:https:443", "theme", "dark"),
		insertWebappsstore("moc.buhtig.:https:443", "lang", "en"),
		insertWebappsstore("moc.elpmaxe.:http:8080", "token", "abc123"),
	)
}

func TestExtractLocalStorage(t *testing.T) {
	path := setupWebappsDB(t)

	got, err := extractLocalStorage(path)
	require.NoError(t, err)
	require.Len(t, got, 3)

	// Verify field mapping by collecting into lookup
	byKey := map[string]string{}
	for _, entry := range got {
		byKey[entry.URL+"/"+entry.Key] = entry.Value
	}
	assert.Equal(t, "dark", byKey["https://github.com:443/theme"])
	assert.Equal(t, "en", byKey["https://github.com:443/lang"])
	assert.Equal(t, "abc123", byKey["http://example.com:8080/token"])
}

func TestCountLocalStorage(t *testing.T) {
	path := setupWebappsDB(t)

	count, err := countLocalStorage(path)
	require.NoError(t, err)
	assert.Equal(t, 3, count)
}

func TestCountLocalStorage_Empty(t *testing.T) {
	path := createTestDB(t, "webappsstore.sqlite", []string{webappsstore2Schema})

	count, err := countLocalStorage(path)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestParseOriginKey(t *testing.T) {
	tests := []struct {
		name      string
		originKey string
		want      string
	}{
		{
			name:      "https with port",
			originKey: "moc.buhtig.:https:443",
			want:      "https://github.com:443",
		},
		{
			name:      "http with non-standard port",
			originKey: "moc.elpmaxe.:http:8080",
			want:      "http://example.com:8080",
		},
		{
			name:      "no port",
			originKey: "moc.elpmaxe.:https",
			want:      "https://example.com",
		},
		{
			name:      "invalid format",
			originKey: "something",
			want:      "something",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseOriginKey(tt.originKey)
			assert.Equal(t, tt.want, got)
		})
	}
}
