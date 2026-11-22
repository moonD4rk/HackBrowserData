package chromium

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/internal/record"
	"github.com/moond4rk/hackbrowserdata/internal/snapshot"
)

func TestExtractors_CoverEveryCategory(t *testing.T) {
	extractors := dirExtractors(browser.UserDataDir{Browser: testBrowser("chrome")})
	for _, dataCategory := range category.All() {
		extractor, ok := extractors[dataCategory]
		if assert.Truef(t, ok, "no extractor for %s", dataCategory) {
			assert.NotNil(t, extractor.extract, dataCategory)
			assert.NotEmpty(t, extractor.sources, dataCategory)
		}
	}
}

func TestExtractors_OverridesFollowTheBrowserNotTheDirectory(t *testing.T) {
	yandex, ok := browser.Lookup("yandex")
	require.True(t, ok)
	dir := browser.UserDataDir{ID: "yandex-2", Browser: yandex}
	for dataCategory, relativePath := range map[category.Category]string{
		category.Password: "Ya Passman Data", category.CreditCard: "Ya Credit Cards",
	} {
		sources, supported := Sources(dir, dataCategory)
		require.True(t, supported, dataCategory)
		assert.Equal(t, relativePath, sources[0].RelativePath)
	}
}

func TestSources_ForksWithoutAnExtensionList(t *testing.T) {
	for _, name := range []string{"qq", "sogou"} {
		chromiumBrowser, ok := browser.Lookup(name)
		require.True(t, ok, name)
		_, supported := Sources(browser.UserDataDir{Browser: chromiumBrowser}, category.Extension)
		assert.False(t, supported, name)
	}
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
		require.NoError(t, Extract(browser.NewProfileSnapshot(dir, profile, dirSnapshot), nil, category.History, &out))
		got := collected[record.History](t, &out)
		require.Len(t, got, 1, profile.Name)
		assert.Equal(t, want[profile.Name], got[0].Title, profile.Name)
	}
}

// TestExtractors_ExtensionsFallBackToPreferences covers Linux builds and 360, whose Secure Preferences holds no list.
func TestExtractors_ExtensionsFallBackToPreferences(t *testing.T) {
	tests := []struct {
		browserName string
		preferences string
	}{
		{"chrome", `{"extensions": {"settings": {"ext1": {"location": 1, "manifest": {"name": "Ext", "version": "1.0"}}}}}`},
		{"opera", `{"extensions": {"opsettings": {"ext1": {"location": 1, "manifest": {"name": "Ext", "version": "1.0"}}}}}`},
	}
	for _, testCase := range tests {
		chromiumBrowser, ok := browser.Lookup(testCase.browserName)
		require.True(t, ok, testCase.browserName)
		captured := browser.NewProfileSnapshot(browser.UserDataDir{Browser: chromiumBrowser}, browser.Profile{Name: "Default", RelativePath: "Default"},
			snapshotOf(t, map[string]string{
				"Default/Secure Preferences": createTestJSON(t, "Secure Preferences", `{"protection": {"macs": {}}}`),
				"Default/Preferences":        createTestJSON(t, "Preferences", testCase.preferences),
			}))

		var out testCollector
		require.NoError(t, Extract(captured, nil, category.Extension, &out), testCase.browserName)
		got := collected[record.Extension](t, &out)
		require.Len(t, got, 1, testCase.browserName)
		assert.Equal(t, "Ext", got[0].Name, testCase.browserName)
	}
}

func TestDecryptingExtractor_ReadsSourcesInOrder(t *testing.T) {
	local := createTestDB(t, "Login Data", loginsSchema, insertLogin("https://local.example", "", "local-user", "", 13340000000000000))
	account := createTestDB(t, "Login Data For Account", loginsSchema,
		insertLogin("https://account.example", "", "account-user", "", 13360000000000000))
	captured := defaultProfileSnapshot(snapshotOf(t, map[string]string{
		"Default/Login Data":             local,
		"Default/Login Data For Account": account,
	}))

	var out testCollector
	require.NoError(t, defaultExtractors[category.Password].extract(captured, nil, &out))
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
	captured := defaultProfileSnapshot(snapshotOf(t, map[string]string{"Default/Login Data For Account": account}))

	var out testCollector
	require.NoError(t, defaultExtractors[category.Password].extract(captured, nil, &out))
	got := collected[record.Login](t, &out)
	require.Len(t, got, 1)
	assert.Equal(t, record.StoreAccount, got[0].Store)
}

func TestDecryptingExtractor_BrokenSourceKeepsTheOthers(t *testing.T) {
	broken := createTestJSON(t, "Login Data", "not a database")
	account := createTestDB(t, "Login Data For Account", loginsSchema,
		insertLogin("https://account.example", "", "account-user", "", 13360000000000000))
	captured := defaultProfileSnapshot(snapshotOf(t, map[string]string{
		"Default/Login Data":             broken,
		"Default/Login Data For Account": account,
	}))

	var out testCollector
	require.Error(t, defaultExtractors[category.Password].extract(captured, nil, &out))
	assert.Len(t, collected[record.Login](t, &out), 1)
}

func defaultProfileSnapshot(dirSnapshot snapshot.Snapshot) browser.ProfileSnapshot {
	return browser.NewProfileSnapshot(browser.UserDataDir{Browser: testBrowser("chrome")}, browser.Profile{Name: "Default", RelativePath: "Default"}, dirSnapshot)
}
