//go:build linux

package masterkey

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPosixRetriever(t *testing.T) {
	retriever := posixRetriever{}

	key, err := retriever.Retrieve(context.Background(), Locator{SecretLabel: "Chrome"})
	require.NoError(t, err)
	assert.Equal(t, linuxParams.deriveKey([]byte("peanuts")), key)
	assert.Len(t, key, linuxParams.keySize)

	allZero := true
	for _, keyByte := range key {
		if keyByte != 0 {
			allZero = false
			break
		}
	}
	assert.False(t, allZero, "derived key should not be all zeros")

	// The password is fixed, so the key is the same for any locator and on every call.
	braveKey, err := retriever.Retrieve(context.Background(), Locator{SecretLabel: "Brave"})
	require.NoError(t, err)
	assert.Equal(t, key, braveKey, "kV10Key should be constant across any storage label")
}

// TestPosixRetriever_MatchesChromiumKV10Key pins the output to Chromium's kV10Key reference bytes.
func TestPosixRetriever_MatchesChromiumKV10Key(t *testing.T) {
	want := []byte{
		0xfd, 0x62, 0x1f, 0xe5, 0xa2, 0xb4, 0x02, 0x53,
		0x9d, 0xfa, 0x14, 0x7c, 0xa9, 0x27, 0x27, 0x78,
	}
	key, err := posixRetriever{}.Retrieve(context.Background(), Locator{})
	require.NoError(t, err)
	assert.Equal(t, want, key)
}

func TestPlatformRetrievers_Linux(t *testing.T) {
	require.Len(t, platformRetrievers, 2)
	assert.IsType(t, posixRetriever{}, platformRetrievers[0], "v10 is the peanuts kV10Key")
	assert.IsType(t, dbusRetriever{}, platformRetrievers[1], "v11 is the D-Bus keyring kV11Key")
	assert.Equal(t, V10, platformRetrievers[0].Version())
	assert.Equal(t, V11, platformRetrievers[1].Version())
}

func TestLabelMatches(t *testing.T) {
	assert.True(t, labelMatches("Chrome Safe Storage", "Chrome Safe Storage"), "exact label")
	assert.True(t, labelMatches("Chrome Keys/Chrome Safe Storage", "Chrome Safe Storage"), "KWallet folder/entry form")
	assert.False(t, labelMatches("Chromium Safe Storage", "Chrome Safe Storage"), "a different browser must not match")
	assert.False(t, labelMatches("Chromium Keys/Chromium Safe Storage", "Chrome Safe Storage"), "nor its folder/entry form")
	assert.False(t, labelMatches("", "Chrome Safe Storage"), "an unreadable label must not match")
}

func TestFindStorageSecret(t *testing.T) {
	const storage = "Chrome Safe Storage"
	libsecret := map[string]string{"xdg:schema": "chrome_libsecret_os_crypt_password_v2"}
	kwallet := map[string]string{"xdg:schema": "org.qt.keychain", "user": storage}
	tests := []struct {
		name    string
		store   fakeSecretStore
		want    string
		wantErr error
	}{
		{
			name:  "libsecret item",
			store: fakeSecretStore{{attributes: libsecret, label: storage, secret: "gnome"}},
			want:  "gnome",
		},
		{
			name:  "KWallet folder/entry label",
			store: fakeSecretStore{{attributes: kwallet, label: "Chrome Keys/" + storage, secret: "kde"}},
			want:  "kde",
		},
		{
			name: "KWallet entry under another schema",
			store: fakeSecretStore{
				{attributes: map[string]string{"xdg:schema": "other", "user": storage}, label: "Chrome Keys/" + storage, secret: "kde"},
			},
			want: "kde",
		},
		{
			name:    "another browser's item",
			store:   fakeSecretStore{{attributes: libsecret, label: "Chromium Safe Storage", secret: "chromium"}},
			wantErr: errStorageNotFound,
		},
		{
			name: "stale path is skipped",
			store: fakeSecretStore{
				{attributes: libsecret, label: storage, stale: true},
				{attributes: libsecret, label: storage, secret: "live"},
			},
			want: "live",
		},
		{
			name:  "locked item that opens without a prompt",
			store: fakeSecretStore{{attributes: libsecret, label: storage, secret: "opened", locked: true, unlockable: true}},
			want:  "opened",
		},
		{
			name:    "wallet that stays locked",
			store:   fakeSecretStore{{attributes: libsecret, label: storage, secret: "hidden", locked: true}},
			wantErr: errKeyringLocked,
		},
		{
			name: "a later match wins over a locked wallet",
			store: fakeSecretStore{
				{attributes: libsecret, label: storage, secret: "hidden", locked: true},
				{attributes: kwallet, label: "Chrome Keys/" + storage, secret: "kde"},
			},
			want: "kde",
		},
		{
			name:    "empty store",
			wantErr: errStorageNotFound,
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			secret, err := findStorageSecret(context.Background(), testCase.store, storage)
			if testCase.wantErr != nil {
				require.ErrorIs(t, err, testCase.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, testCase.want, string(secret))
		})
	}
}

func TestFindStorageSecret_Timeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()
	store := fakeSecretStore{{attributes: map[string]string{"user": "Chrome Safe Storage"}, label: "Chrome Safe Storage", secret: "late"}}

	_, err := findStorageSecret(ctx, store, "Chrome Safe Storage")
	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.NotErrorIs(t, err, errStorageNotFound, "a lookup that ran out of time must not read as not found")
}

// fakeItem is one Secret Service item: a locked one reads only once unlocked, and a stale one never does.
type fakeItem struct {
	attributes map[string]string
	label      string
	secret     string
	locked     bool
	unlockable bool
	stale      bool
}

type fakeSecretStore []fakeItem

func fakeItemPath(index int) dbus.ObjectPath {
	return dbus.ObjectPath("/item/" + strconv.Itoa(index))
}

func (s fakeSecretStore) item(path dbus.ObjectPath) fakeItem {
	for index, item := range s {
		if fakeItemPath(index) == path {
			return item
		}
	}
	return fakeItem{stale: true}
}

func (s fakeSecretStore) searchItems(ctx context.Context, attributes map[string]string) (unlocked, locked []dbus.ObjectPath, err error) {
	if ctx.Err() != nil {
		return nil, nil, ctx.Err()
	}
	for index, item := range s {
		if !hasAttributes(item.attributes, attributes) {
			continue
		}
		if item.locked {
			locked = append(locked, fakeItemPath(index))
		} else {
			unlocked = append(unlocked, fakeItemPath(index))
		}
	}
	return unlocked, locked, nil
}

func (s fakeSecretStore) unlock(_ context.Context, items []dbus.ObjectPath) ([]dbus.ObjectPath, error) {
	var opened []dbus.ObjectPath
	for _, path := range items {
		for index := range s {
			if fakeItemPath(index) == path && s[index].unlockable {
				s[index].locked = false
				opened = append(opened, path)
			}
		}
	}
	return opened, nil
}

func (s fakeSecretStore) itemLabel(_ context.Context, path dbus.ObjectPath) (string, error) {
	item := s.item(path)
	if item.stale {
		return "", errors.New("no such object")
	}
	return item.label, nil
}

func (s fakeSecretStore) itemSecret(_ context.Context, path dbus.ObjectPath) ([]byte, error) {
	item := s.item(path)
	switch {
	case item.stale:
		return nil, errors.New("no such object")
	case item.locked:
		return nil, errors.New("cannot get secret of a locked object")
	}
	return []byte(item.secret), nil
}

func hasAttributes(attributes, query map[string]string) bool {
	for key, value := range query {
		if attributes[key] != value {
			return false
		}
	}
	return true
}
