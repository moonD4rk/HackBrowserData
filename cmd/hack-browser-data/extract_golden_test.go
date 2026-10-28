package main

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/browser"
	"github.com/moond4rk/hackbrowserdata/internal/category"
)

func TestExtractGolden(t *testing.T) {
	root := t.TempDir()
	chromiumFixtureInstallations(t, root)
	assertExtractGoldens(t, "extract", func(t *testing.T, format string) string {
		return extractGoldenRun(t, chromiumFixtureBrowsers(t, root), format)
	})
}

func TestExtractGoldenFirefox(t *testing.T) {
	profilesDir := filepath.Join(t.TempDir(), "firefox")
	firefoxFixtureUserData(t, profilesDir)
	assertExtractGoldens(t, "extract-firefox", func(t *testing.T, format string) string {
		return extractGoldenRun(t, firefoxFixtureBrowsers(t, profilesDir), format)
	})
}

func TestExtractGoldenSafari(t *testing.T) {
	home := t.TempDir()
	// Safari reads passwords from the login keychain under the home directory, which must not be the real one.
	t.Setenv("HOME", home)
	safariFixtureLibrary(t, filepath.Join(home, "Library"))
	assertExtractGoldens(t, "extract-safari", func(t *testing.T, format string) string {
		return extractGoldenRun(t, safariFixtureBrowsers(t, home), format)
	})
}

var extractGoldenFormats = []struct {
	format       string
	jsonFallback bool
}{
	{format: "csv"},
	{format: "json"},
	{format: "cookie-editor", jsonFallback: true},
}

func assertExtractGoldens(t *testing.T, prefix string, run func(t *testing.T, format string) string) {
	t.Helper()
	for _, tt := range extractGoldenFormats {
		tt := tt
		t.Run(tt.format, func(t *testing.T) {
			out := run(t, tt.format)
			got := bundleDir(t, out)
			if tt.jsonFallback {
				assertJSONFallback(t, out, run(t, "json"))
				got = bundleFiles(t, out, "cookie.json")
			}
			assertGolden(t, prefix+"-"+tt.format+".golden", got)
		})
	}
}

func extractGoldenRun(t *testing.T, browsers []browser.Browser, format string) string {
	t.Helper()
	out := t.TempDir()
	require.NoError(t, extractAndWrite(browsers, category.All(), out, format, false))
	return out
}
