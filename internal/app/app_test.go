package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/internal/export"
)

func TestValidate(t *testing.T) {
	req, err := Validate(Flags{Browser: " Chrome ", Categories: "cookie, password,cookie", Format: "csv", Dir: "out", KeychainPassword: "pw"})
	require.NoError(t, err)
	assert.Equal(t, "chrome", req.Browser)
	assert.Equal(t, []category.Category{category.Cookie, category.Password}, req.Categories, "a repeated category runs once")
	assert.Equal(t, export.CSV, req.Format)
	assert.Equal(t, "out", req.Dir)
	assert.NotEmpty(t, req.Home)
	password, err := req.KeychainPassword(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "pw", password)

	req, err = Validate(Flags{Categories: "all", Format: "json"})
	require.NoError(t, err)
	assert.Equal(t, "all", req.Browser)
	assert.Equal(t, category.All(), req.Categories)
}

func TestValidate_RejectsBadInput(t *testing.T) {
	for name, f := range map[string]Flags{
		"unknown category": {Categories: "cookie,bogus", Format: "json"},
		"no category":      {Categories: " , ", Format: "json"},
		"unknown format":   {Categories: "all", Format: "xml"},
	} {
		_, err := Validate(f)
		assert.Error(t, err, name)
	}
}
