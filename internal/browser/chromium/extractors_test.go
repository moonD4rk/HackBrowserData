package chromium

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/internal/record"
)

func TestExtractors_CoverEveryCategory(t *testing.T) {
	extractors := Extractors(browser.UserDataDir{Browser: testBrowser("chrome")}, browser.Profile{})
	for _, dataCategory := range category.All() {
		extractor, ok := extractors[dataCategory]
		if assert.Truef(t, ok, "no extractor for %s", dataCategory) {
			assert.NotNil(t, extractor.Extract, dataCategory)
			assert.NotEmpty(t, extractor.Sources, dataCategory)
		}
	}
}

func TestExtractors_OverridesFollowTheBrowserNotTheDirectory(t *testing.T) {
	yandex, ok := browser.Lookup("yandex")
	require.True(t, ok)
	extractors := Extractors(browser.UserDataDir{ID: "yandex-2", Browser: yandex}, browser.Profile{})
	assert.Equal(t, "Ya Passman Data", extractors[category.Password].Sources[0].Path)
	assert.Equal(t, "Ya Credit Cards", extractors[category.CreditCard].Sources[0].Path)
}

// TestExtractors_EachProfileReadsItsOwnFiles reads one snapshot shared by every profile; each must get its own files.
func TestExtractors_EachProfileReadsItsOwnFiles(t *testing.T) {
	dirSnapshot := snapshotOf(t, map[string]string{
		"Default/History":   createTestDB(t, "History", urlsSchema, insertURL("https://home.example", "Home", 1, 13350000000000000)),
		"Profile 1/History": createTestDB(t, "History", urlsSchema, insertURL("https://work.example", "Work", 1, 13350000000000000)),
	})
	dir := browser.UserDataDir{Browser: testBrowser("chrome")}
	want := map[string]string{"Default": "Home", "Profile 1": "Work"}
	for _, profile := range []browser.Profile{{Name: "Default", RelativePath: "Default"}, {Name: "Profile 1", RelativePath: "Profile 1"}} {
		var out testCollector
		require.NoError(t, Extractors(dir, profile)[category.History].Extract(context.Background(), browser.NewInput(dirSnapshot, profile, nil, nil), &out))
		got := collected[record.History](t, &out)
		require.Len(t, got, 1, profile.Name)
		assert.Equal(t, want[profile.Name], got[0].Title, profile.Name)
	}
}

func TestDecryptingExtractor_ReadsSourcesInOrder(t *testing.T) {
	local := createTestDB(t, "Login Data", loginsSchema, insertLogin("https://local.example", "", "local-user", "", 13340000000000000))
	account := createTestDB(t, "Login Data For Account", loginsSchema,
		insertLogin("https://account.example", "", "account-user", "", 13360000000000000))
	input := browser.NewInput(snapshotOf(t, map[string]string{
		"Default/Login Data":             local,
		"Default/Login Data For Account": account,
	}), browser.Profile{Name: "Default", RelativePath: "Default"}, nil, nil)

	var out testCollector
	require.NoError(t, defaultExtractors[category.Password].Extract(context.Background(), input, &out))
	got := collected[record.Login](t, &out)
	require.Len(t, got, 2)
	assert.Equal(t, "local-user", got[0].Username)
	assert.Equal(t, record.StoreLocal, got[0].Store)
	assert.Equal(t, "account-user", got[1].Username)
	assert.Equal(t, record.StoreAccount, got[1].Store)
}

func TestDecryptingExtractor_SkipsAbsentOptionalSource(t *testing.T) {
	account := createTestDB(t, "Login Data For Account", loginsSchema,
		insertLogin("https://account.example", "", "account-user", "", 13360000000000000))
	input := browser.NewInput(snapshotOf(t, map[string]string{"Default/Login Data For Account": account}),
		browser.Profile{Name: "Default", RelativePath: "Default"}, nil, nil)

	var out testCollector
	require.NoError(t, defaultExtractors[category.Password].Extract(context.Background(), input, &out))
	got := collected[record.Login](t, &out)
	require.Len(t, got, 1)
	assert.Equal(t, record.StoreAccount, got[0].Store)
}

func TestDecryptingExtractor_BrokenSourceKeepsTheOthers(t *testing.T) {
	broken := createTestJSON(t, "Login Data", "not a database")
	account := createTestDB(t, "Login Data For Account", loginsSchema,
		insertLogin("https://account.example", "", "account-user", "", 13360000000000000))
	input := browser.NewInput(snapshotOf(t, map[string]string{
		"Default/Login Data":             broken,
		"Default/Login Data For Account": account,
	}), browser.Profile{Name: "Default", RelativePath: "Default"}, nil, nil)

	var out testCollector
	require.Error(t, defaultExtractors[category.Password].Extract(context.Background(), input, &out))
	assert.Len(t, collected[record.Login](t, &out), 1)
}
