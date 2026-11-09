package browser

import (
	"os"
	"path"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
)

// TestTableOrderPerOS pins discovery order, which is output order: reordering the table reorders every output file.
func TestTableOrderPerOS(t *testing.T) {
	want := map[string][]string{
		"darwin": {
			"chrome", "edge", "chromium", "chrome-beta", "opera", "opera-gx", "vivaldi", "coccoc", "brave", "yandex", "arc",
			"firefox", "safari",
		},
		"linux": {"chrome", "edge", "chromium", "chrome-beta", "opera", "vivaldi", "brave", "firefox"},
		"windows": {
			"chrome", "edge", "chromium", "chrome-beta", "opera", "opera-gx", "vought", "vivaldi", "coccoc", "brave", "yandex",
			"360x", "360", "qq", "dc", "sogou", "arc", "duckduckgo", "firefox",
		},
	}
	for goos, platform := range platformOf {
		assert.Equal(t, want[goos], namesOn(platform), goos)
	}
}

func TestTableRows(t *testing.T) {
	seen := make(map[string]bool)
	for _, browser := range browsers {
		assert.NotEmpty(t, browser.Name)
		assert.Falsef(t, seen[browser.Name], "duplicate name %q", browser.Name)
		seen[browser.Name] = true
		assert.NotEmpty(t, browser.DisplayName, browser.Name)
		assert.Contains(t, []Base{Chromium, Firefox, Safari}, browser.Base, browser.Name)

		for goos, platform := range platformOf {
			for _, dir := range platform(browser).UserDataDirs {
				assert.Truef(t, dir != "" && !path.IsAbs(dir) && !strings.Contains(dir, `\`),
					"%s %s: user data dir %q must be slash-separated and relative to the home dir", browser.Name, goos, dir)
			}
		}

		windows := browser.Windows
		if windows.ABE == masterkey.ABENone {
			assert.Emptyf(t, windows.Executable, "%s: an executable is only used by ABE", browser.Name)
			assert.Emptyf(t, windows.ExecutableFallbacks, "%s: executable fallbacks are only used by ABE", browser.Name)
			continue
		}
		assert.Equal(t, Chromium, browser.Base, browser.Name)
		assert.NotEmpty(t, windows.Executable, browser.Name)
		for _, fallback := range windows.ExecutableFallbacks {
			assert.Truef(t, strings.HasSuffix(strings.ToLower(fallback), `\`+strings.ToLower(windows.Executable)),
				"%s: fallback %q does not name %s", browser.Name, fallback, windows.Executable)
		}
	}
}

// TestABERowsMatchPayloadTable needs a C row for every Go ABE row, since the payload finds a browser by its executable.
func TestABERowsMatchPayloadTable(t *testing.T) {
	comIIDTable, err := os.ReadFile("../abe/native/com_iid.c")
	require.NoError(t, err)

	kinds := map[string]masterkey.ABEKind{
		"BROWSER_CHROME_BASE": masterkey.ABEChromeBase,
		"BROWSER_EDGE":        masterkey.ABEEdge,
		"BROWSER_AVAST":       masterkey.ABEAvast,
	}
	payload := make(map[string]masterkey.ABEKind)
	for _, match := range regexp.MustCompile(`"([^"]+\.exe)",\s*(BROWSER_\w+)`).FindAllStringSubmatch(string(comIIDTable), -1) {
		kind, ok := kinds[match[2]]
		require.Truef(t, ok, "unknown payload browser kind %s", match[2])
		exe := strings.ToLower(match[1])
		if _, duplicate := payload[exe]; !duplicate {
			payload[exe] = kind
		}
	}
	require.NotEmpty(t, payload, "no rows parsed from com_iid.c")

	for _, browser := range browsers {
		platform := browser.Windows
		if platform.ABE == masterkey.ABENone {
			continue
		}
		kind, ok := payload[strings.ToLower(platform.Executable)]
		if assert.Truef(t, ok, "%s: com_iid.c has no row for %s", browser.Name, platform.Executable) {
			assert.Equalf(t, platform.ABE, kind, "%s: com_iid.c dispatches %s as another ABE kind", browser.Name, platform.Executable)
		}
	}
}
