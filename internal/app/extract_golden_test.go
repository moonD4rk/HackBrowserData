package app_test

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/app"
	"github.com/moond4rk/hackbrowserdata/internal/browser/browsertest"
	"github.com/moond4rk/hackbrowserdata/internal/golden"

	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
)

// TestExtractGolden hands in the fixtures' keys, since no secret store holds them.
func TestExtractGolden(t *testing.T) {
	home := t.TempDir()
	keys := make(map[browser.UserDataDirID]masterkey.Set)
	for _, fixture := range browsertest.ChromiumUserDataDirs {
		namedBrowser, ok := browser.Lookup(fixture.Name)
		require.True(t, ok, fixture.Name)
		require.Equal(t, fixture.DisplayName, namedBrowser.DisplayName, "records carry the table's display name")
		browsertest.WriteChromiumUserDataDir(t, filepath.Join(home, filepath.FromSlash(namedBrowser.Platform().UserDataDirs[0])), fixture.Name)
		keys[browser.UserDataDirID(fixture.Name)] = browsertest.ChromiumKeys
	}

	assertExtractGoldens(t, "extract", func(t *testing.T, format string) string {
		return extractGoldenRun(t, home, keys, format)
	})
}

func TestExtractGoldenFirefox(t *testing.T) {
	home := t.TempDir()
	firefoxBrowser, ok := browser.Lookup("firefox")
	require.True(t, ok)
	browsertest.WriteFirefoxUserDataDir(t, filepath.Join(home, filepath.FromSlash(firefoxBrowser.Platform().UserDataDirs[0])))

	assertExtractGoldens(t, "extract-firefox", func(t *testing.T, format string) string {
		return extractGoldenRun(t, home, nil, format)
	})
}

func TestExtractGoldenSafari(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Safari is in the browser table on macOS only")
	}
	home := t.TempDir()
	browsertest.WriteSafariLibrary(t, filepath.Join(home, "Library"))

	assertExtractGoldens(t, "extract-safari", func(t *testing.T, format string) string {
		return extractGoldenRun(t, home, nil, format)
	})
}

func extractGoldenRun(t *testing.T, home string, keys map[browser.UserDataDirID]masterkey.Set, format string) string {
	t.Helper()
	out := t.TempDir()
	options, err := app.ValidateDump(app.DumpFlags{Browser: "all", Categories: "all", Format: format, OutputDir: out}, nil)
	require.NoError(t, err)
	options.Target.HomeDir = home

	_, err = app.ExecuteWithKeys(context.Background(), app.Discover(options.Target), options.Categories, options.Output, keys)
	require.NoError(t, err)
	return out
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
	for _, testCase := range extractGoldenFormats {
		testCase := testCase
		t.Run(testCase.format, func(t *testing.T) {
			out := run(t, testCase.format)
			got := golden.ConcatDir(t, out)
			if testCase.jsonFallback {
				golden.AssertJSONFallback(t, out, run(t, "json"))
				got = golden.ConcatFiles(t, out, "cookie.json")
			}
			golden.Assert(t, prefix+"-"+testCase.format+".golden", got)
		})
	}
}
