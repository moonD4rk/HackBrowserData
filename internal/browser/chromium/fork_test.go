package chromium

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/category"
	"github.com/moond4rk/hackbrowserdata/internal/record"
)

func TestForks_NameChromiumBrowsers(t *testing.T) {
	for _, fork := range forks {
		for _, name := range fork.browserNames {
			chromiumBrowser, ok := browser.Lookup(name)
			if assert.Truef(t, ok, "fork %q names no browser", name) {
				assert.Equalf(t, browser.Chromium, chromiumBrowser.Base, "fork %q is not a Chromium browser", name)
			}
		}
	}
}

// TestForks_KindsAndNamesAreUnique keeps findForkByName and findForkByVaultKind from depending on table order.
func TestForks_KindsAndNamesAreUnique(t *testing.T) {
	kinds := map[string]bool{vaultKindChromium: true}
	names := map[string]bool{}
	for _, fork := range forks {
		if fork.vaultKind != "" {
			assert.Falsef(t, kinds[fork.vaultKind], "kind %q is used twice", fork.vaultKind)
			kinds[fork.vaultKind] = true
		}
		for _, name := range fork.browserNames {
			assert.Falsef(t, names[name], "browser %q is in two forks", name)
			names[name] = true
		}
	}
}

// TestVaultKind_RoundTripsEveryFork checks a kind leads back to its fork, or restore would parse it as plain Chromium.
func TestVaultKind_RoundTripsEveryFork(t *testing.T) {
	for _, fork := range forks {
		for _, name := range fork.browserNames {
			kind := VaultKind(browser.Browser{Name: name})
			if fork.vaultKind == "" {
				assert.Equalf(t, vaultKindChromium, kind, "fork %q keeps Chromium's kind", name)
				continue
			}
			require.Equalf(t, fork.vaultKind, kind, "fork %q", name)
			restored, ok := findForkByVaultKind(kind)
			require.True(t, ok, kind)
			assert.Equalf(t, categoriesOf(fork.extractors), categoriesOf(restored.extractors), "kind %s of %q", kind, name)
		}
	}
	assert.Equal(t, vaultKindChromium, VaultKind(browser.Browser{Name: "chrome"}))
	assert.True(t, isKnownVaultKind(vaultKindChromium))
	assert.False(t, isKnownVaultKind("bogus"))
	assert.False(t, isKnownVaultKind(""))
}

// TestDiscoverProfiles_ReadsAnUnknownBrowserInItsVaultKind covers a restored fork this build does not know by name.
func TestDiscoverProfiles_ReadsAnUnknownBrowserInItsVaultKind(t *testing.T) {
	nightly := browser.Browser{Name: "nightly", DisplayName: "Nightly", Base: browser.Chromium}
	dir := browser.UserDataDir{ID: "nightly", Browser: nightly, Path: t.TempDir(), VaultKind: "chromium-yandex"}
	_, err := DiscoverProfiles(dir)
	require.NoError(t, err)
	assert.Equal(t, "Ya Passman Data", Sources(dir)[category.Password][0].RelativePath)

	dir.VaultKind = "bogus"
	_, err = DiscoverProfiles(dir)
	require.Error(t, err)
}

func categoriesOf(extractors map[category.Category]extractor) []category.Category {
	var out []category.Category
	for _, dataCategory := range category.All() {
		if _, ok := extractors[dataCategory]; ok {
			out = append(out, dataCategory)
		}
	}
	return out
}

func TestExtractors_OperaReadsOpsettings(t *testing.T) {
	preferencesPath := createTestJSON(t, "Secure Preferences", `{
		"extensions": {"opsettings": {"ext1": {"location": 1, "manifest": {"name": "Opera Ext", "version": "1.0"}}}}
	}`)
	dirSnapshot := snapshotOf(t, map[string]string{"Default/Secure Preferences": preferencesPath})
	profile := browser.Profile{Name: "Default", RelativePath: "Default"}

	for _, name := range []string{"opera", "opera-gx", "vought"} {
		chromiumBrowser, ok := browser.Lookup(name)
		require.True(t, ok, name)
		var out testCollector
		captured := browser.NewProfileSnapshot(browser.UserDataDir{Browser: chromiumBrowser}, profile, dirSnapshot)
		require.NoError(t, Extract(captured, nil, category.Extension, &out), name)
		got := collected[record.Extension](t, &out)
		require.Len(t, got, 1, name)
		assert.Equal(t, "Opera Ext", got[0].Name, name)
	}

	chrome, ok := browser.Lookup("chrome")
	require.True(t, ok)
	err := Extract(browser.NewProfileSnapshot(browser.UserDataDir{Browser: chrome}, profile, dirSnapshot), nil, category.Extension, &testCollector{})
	require.Error(t, err, "Chrome has no opsettings fallback")
}
