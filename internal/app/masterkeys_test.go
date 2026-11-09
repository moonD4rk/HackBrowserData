package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
	"github.com/moond4rk/hackbrowserdata/internal/snapshot"
)

func TestKeysFromFile(t *testing.T) {
	want := masterkey.Set{masterkey.V10: []byte("dumped")}
	keys := keysFromFile(map[browser.UserDataDirID]masterkey.Set{"chrome": want})
	got, err := keys(context.Background(), browser.UserDataDir{ID: "chrome"}, snapshot.Snapshot{})
	require.NoError(t, err)
	assert.Equal(t, want, got)

	got, err = keys(context.Background(), browser.UserDataDir{ID: "edge"}, snapshot.Snapshot{})
	require.NoError(t, err)
	assert.Empty(t, got, "a key file without the dir's keys decrypts nothing")
}
