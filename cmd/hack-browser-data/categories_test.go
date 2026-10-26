package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/category"
)

func TestParseCategories(t *testing.T) {
	for _, s := range []string{"all", " ALL "} {
		got, err := parseCategories(s)
		require.NoError(t, err)
		assert.Equal(t, category.All(), got)
	}

	got, err := parseCategories(" Cookie, password,,history ")
	require.NoError(t, err)
	assert.Equal(t, []category.Category{category.Cookie, category.Password, category.History}, got)
}

func TestParseCategories_Errors(t *testing.T) {
	_, err := parseCategories("cookie,bogus")
	require.EqualError(t, err, `unknown category: "bogus", available: all|`+
		"password,cookie,bookmark,history,download,creditcard,extension,localstorage,sessionstorage")

	_, err = parseCategories(" , ")
	require.EqualError(t, err, "no categories specified")
}
