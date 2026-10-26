package main

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/types"
)

func TestExtractGolden(t *testing.T) {
	root := t.TempDir()
	chromiumFixtureInstallations(t, root)

	tests := []struct {
		format       string
		golden       string
		jsonFallback bool
	}{
		{format: "csv", golden: "extract-csv.golden"},
		{format: "json", golden: "extract-json.golden"},
		{format: "cookie-editor", golden: "extract-cookie-editor.golden", jsonFallback: true},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.format, func(t *testing.T) {
			out := extractGoldenRun(t, root, tt.format)
			got := bundleDir(t, out)
			if tt.jsonFallback {
				assertJSONFallback(t, out, extractGoldenRun(t, root, "json"))
				got = bundleFiles(t, out, "cookie.json")
			}
			assertGolden(t, tt.golden, got)
		})
	}
}

func extractGoldenRun(t *testing.T, root, format string) string {
	t.Helper()
	out := t.TempDir()
	require.NoError(t, extractAndWrite(chromiumFixtureBrowsers(t, root), types.AllCategories, out, format, false))
	return out
}
