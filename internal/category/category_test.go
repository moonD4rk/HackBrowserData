package category

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestString(t *testing.T) {
	tests := []struct {
		c    Category
		want string
	}{
		{Password, "password"},
		{Cookie, "cookie"},
		{Bookmark, "bookmark"},
		{History, "history"},
		{Download, "download"},
		{CreditCard, "creditcard"},
		{Extension, "extension"},
		{LocalStorage, "localstorage"},
		{SessionStorage, "sessionstorage"},
		{Category(0), "unknown"},
		{Category(999), "unknown"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, tt.c.String())
	}
}

func TestAll(t *testing.T) {
	assert.Equal(t, []Category{
		Password, Cookie, Bookmark, History, Download, CreditCard, Extension, LocalStorage, SessionStorage,
	}, All())
}

func TestAll_ReturnsFreshSlice(t *testing.T) {
	All()[0] = SessionStorage
	assert.Equal(t, Password, All()[0])
}

func TestParse_RoundTrip(t *testing.T) {
	for _, c := range All() {
		got, err := Parse(c.String())
		require.NoError(t, err)
		assert.Equal(t, c, got)
	}
}

func TestParse_Unknown(t *testing.T) {
	for _, s := range []string{"", "unknown", "all", "Password", " cookie"} {
		got, err := Parse(s)
		require.EqualError(t, err, "unknown category: \""+s+"\"")
		assert.Zero(t, got)
	}
}

func TestZeroIsNone(t *testing.T) {
	var c Category
	assert.NotContains(t, All(), c)
}

func TestNames(t *testing.T) {
	assert.Equal(t, []string{
		"password", "cookie", "bookmark", "history", "download", "creditcard", "extension", "localstorage", "sessionstorage",
	}, Names())
}
