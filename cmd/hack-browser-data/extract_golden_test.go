package main

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/app"
	"github.com/moond4rk/hackbrowserdata/internal/browser"
	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
)

// The extract goldens pin dump's output on fixtures; the fixtures' keys are handed in, since no secret store holds them.
func TestExtractGolden(t *testing.T) {
	home := t.TempDir()
	keys := make(map[browser.DirID]masterkey.Set)
	for _, inst := range chromiumFixtureInstalls {
		def, ok := browser.Lookup(inst.key)
		require.True(t, ok, inst.key)
		require.Equal(t, inst.name, def.Name, "records are stamped with the table's display name")
		chromiumFixtureUserData(t, filepath.Join(home, filepath.FromSlash(def.Platform().UserDataDirs[0])), inst.key)
		keys[browser.DirID(inst.key)] = chromiumFixtureKeys
	}

	assertExtractGoldens(t, "extract", func(t *testing.T, format string) string {
		return extractGoldenRun(t, home, keys, format)
	})
}

func TestExtractGoldenFirefox(t *testing.T) {
	home := t.TempDir()
	def, ok := browser.Lookup("firefox")
	require.True(t, ok)
	firefoxFixtureUserData(t, filepath.Join(home, filepath.FromSlash(def.Platform().UserDataDirs[0])))

	assertExtractGoldens(t, "extract-firefox", func(t *testing.T, format string) string {
		return extractGoldenRun(t, home, nil, format)
	})
}

func TestExtractGoldenSafari(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Safari is in the browser table on macOS only")
	}
	home := t.TempDir()
	safariFixtureLibrary(t, filepath.Join(home, "Library"))

	assertExtractGoldens(t, "extract-safari", func(t *testing.T, format string) string {
		return extractGoldenRun(t, home, nil, format)
	})
}

func extractGoldenRun(t *testing.T, home string, keys map[browser.DirID]masterkey.Set, format string) string {
	t.Helper()
	out := t.TempDir()
	req, err := app.Validate(app.Flags{Browser: "all", Categories: "all", Format: format, Dir: out})
	require.NoError(t, err)
	req.Home, req.Keys = home, keys

	plan := app.NewPlan(app.Discover(req), req)
	_, err = app.Execute(context.Background(), plan, req)
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
