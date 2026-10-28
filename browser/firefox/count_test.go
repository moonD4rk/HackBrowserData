package firefox

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCountBookmarks(t *testing.T) {
	path := setupMozBookmarkDB(t)

	count, err := countBookmarks(path)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestCountBookmarks_Empty(t *testing.T) {
	path := createTestDB(t, "places.sqlite", []string{mozPlacesSchema, mozBookmarksSchema})

	count, err := countBookmarks(path)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestCountCookies(t *testing.T) {
	path := setupMozCookieDB(t)

	count, err := countCookies(path)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestCountCookies_Empty(t *testing.T) {
	path := createTestDB(t, "cookies.sqlite", []string{mozCookiesSchema})

	count, err := countCookies(path)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestCountDownloads(t *testing.T) {
	path := setupMozDownloadDB(t)

	count, err := countDownloads(path)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestCountDownloads_Empty(t *testing.T) {
	path := createTestDB(t, "places.sqlite", []string{mozPlacesSchema, mozAnnosSchema})

	count, err := countDownloads(path)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestCountExtensions(t *testing.T) {
	path := setupMozExtensionJSON(t)

	count, err := countExtensions(path)
	require.NoError(t, err)
	assert.Equal(t, 2, count) // system addon filtered out
}

func TestCountExtensions_Empty(t *testing.T) {
	path := createTestJSON(t, "extensions.json", `{"addons": []}`)

	count, err := countExtensions(path)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestCountHistories(t *testing.T) {
	path := setupMozHistoryDB(t)

	count, err := countHistories(path)
	require.NoError(t, err)
	assert.Equal(t, 3, count)
}

func TestCountHistories_Empty(t *testing.T) {
	path := createTestDB(t, "places.sqlite", []string{mozPlacesSchema})

	count, err := countHistories(path)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
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

func TestCountPasswords(t *testing.T) {
	json := `{
		"logins": [
			{"hostname": "https://a.com", "encryptedUsername": "", "encryptedPassword": "", "timeCreated": 1000},
			{"hostname": "https://b.com", "encryptedUsername": "", "encryptedPassword": "", "timeCreated": 2000},
			{"hostname": "https://c.com", "encryptedUsername": "", "encryptedPassword": "", "timeCreated": 3000}
		]
	}`
	path := createTestJSON(t, "logins.json", json)

	count, err := countPasswords(path)
	require.NoError(t, err)
	assert.Equal(t, 3, count)
}

func TestCountPasswords_Empty(t *testing.T) {
	path := createTestJSON(t, "logins.json", `{"logins": []}`)

	count, err := countPasswords(path)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}
