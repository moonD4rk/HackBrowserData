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

func TestOverrides_NameChromiumBrowsers(t *testing.T) {
	for _, fork := range overrides {
		for _, name := range fork.browserNames {
			chromiumBrowser, ok := browser.Lookup(name)
			if assert.Truef(t, ok, "override %q names no browser", name) {
				assert.Equalf(t, browser.Chromium, chromiumBrowser.Base, "override %q is not a Chromium browser", name)
			}
		}
	}
}

// TestOverrides_KindsAndNamesAreUnique keeps BrowserNameForKind and overrideFor from depending on table order.
func TestOverrides_KindsAndNamesAreUnique(t *testing.T) {
	kinds := map[string]bool{vaultKindChromium: true}
	names := map[string]bool{}
	for _, fork := range overrides {
		assert.Falsef(t, kinds[fork.vaultKind], "kind %q is used twice", fork.vaultKind)
		kinds[fork.vaultKind] = true
		for _, name := range fork.browserNames {
			assert.Falsef(t, names[name], "browser %q is in two overrides", name)
			names[name] = true
		}
	}
}

// TestVaultKind_RoundTripsEveryFork checks a kind leads back to its fork, or restore would parse it as plain Chromium.
func TestVaultKind_RoundTripsEveryFork(t *testing.T) {
	for _, fork := range overrides {
		for _, name := range fork.browserNames {
			kind := VaultKind(browser.Browser{Name: name})
			require.Equalf(t, fork.vaultKind, kind, "override %q", name)
			forkBrowserName, err := BrowserNameForKind(kind, "unknown")
			require.NoError(t, err)
			restored, ok := overrideFor(forkBrowserName)
			require.True(t, ok, forkBrowserName)
			assert.Equalf(t, categoriesOf(fork.extractors), categoriesOf(restored.extractors), "kind %s of %q", kind, name)
		}
	}
	assert.Equal(t, vaultKindChromium, VaultKind(browser.Browser{Name: "chrome"}))
	plainBrowserName, err := BrowserNameForKind(vaultKindChromium, "unknown")
	require.NoError(t, err)
	assert.Equal(t, "unknown", plainBrowserName)
	_, err = BrowserNameForKind("bogus", "unknown")
	assert.Error(t, err)
}

func categoriesOf(extractors browser.Extractors) []category.Category {
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
	profileSnapshot := browser.NewProfileSnapshot(snapshotOf(t, map[string]string{"Default/Secure Preferences": preferencesPath}),
		browser.Profile{Name: "Default", RelativePath: "Default"}, nil)

	for _, name := range []string{"opera", "opera-gx", "vought"} {
		chromiumBrowser, ok := browser.Lookup(name)
		require.True(t, ok, name)
		var out testCollector
		require.NoError(t, Extractors(browser.UserDataDir{Browser: chromiumBrowser}, profileSnapshot.Profile)[category.Extension].Extract(context.Background(), profileSnapshot, &out), name)
		got := collected[record.Extension](t, &out)
		require.Len(t, got, 1, name)
		assert.Equal(t, "Opera Ext", got[0].Name, name)
	}

	chrome, ok := browser.Lookup("chrome")
	require.True(t, ok)
	err := Extractors(browser.UserDataDir{Browser: chrome}, profileSnapshot.Profile)[category.Extension].Extract(context.Background(), profileSnapshot, &testCollector{})
	require.Error(t, err, "Chrome has no opsettings fallback")
}
