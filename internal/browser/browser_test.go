package browser

import (
	"os"
	"path"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moond4rk/hackbrowserdata/internal/masterkey"
)

var platformOf = map[string]func(Def) Platform{
	"darwin":  func(d Def) Platform { return d.Darwin },
	"linux":   func(d Def) Platform { return d.Linux },
	"windows": func(d Def) Platform { return d.Windows },
}

func idsOn(platform func(Def) Platform) []string {
	var ids []string
	for _, d := range defs {
		if len(platform(d).UserDataDirs) > 0 {
			ids = append(ids, d.ID)
		}
	}
	return ids
}

// Discovery order is output order: reordering the table reorders the records in every output file.
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
		assert.Equal(t, want[goos], idsOn(platform), goos)
	}
}

func TestSupported_ListsThisOSRowsInTableOrder(t *testing.T) {
	var ids []string
	for _, d := range Supported() {
		ids = append(ids, d.ID)
	}
	assert.Equal(t, idsOn(Def.Platform), ids)
}

func TestLookup_FindsEveryRowWhateverTheOS(t *testing.T) {
	for _, want := range defs {
		got, ok := Lookup(want.ID)
		require.True(t, ok, want.ID)
		assert.Equal(t, want, got)
	}
	_, ok := Lookup("netscape")
	assert.False(t, ok)
}

func TestTableRows(t *testing.T) {
	seen := make(map[string]bool)
	for _, d := range defs {
		assert.NotEmpty(t, d.ID)
		assert.Falsef(t, seen[d.ID], "duplicate ID %q", d.ID)
		seen[d.ID] = true
		assert.NotEmpty(t, d.Name, d.ID)
		assert.Contains(t, []Base{Chromium, Firefox, Safari}, d.Base, d.ID)

		for goos, platform := range platformOf {
			for _, dir := range platform(d).UserDataDirs {
				assert.Truef(t, dir != "" && !path.IsAbs(dir) && !strings.Contains(dir, `\`),
					"%s %s: user data dir %q must be slash-separated and relative to the home dir", d.ID, goos, dir)
			}
		}

		w := d.Windows
		if w.ABE == masterkey.ABENone {
			assert.Emptyf(t, w.Exe, "%s: an executable is only used by ABE", d.ID)
			assert.Emptyf(t, w.ExeFallbacks, "%s: executable fallbacks are only used by ABE", d.ID)
			continue
		}
		assert.Equal(t, Chromium, d.Base, d.ID)
		assert.NotEmpty(t, w.Exe, d.ID)
		for _, fallback := range w.ExeFallbacks {
			assert.Truef(t, strings.HasSuffix(strings.ToLower(fallback), `\`+strings.ToLower(w.Exe)),
				"%s: fallback %q does not name %s", d.ID, fallback, w.Exe)
		}
	}
}

// The payload looks a browser up by its executable inside the browser process, so its C table is the other half of each
// ABE row. It may list browsers the Go table does not.
func TestABERowsMatchPayloadTable(t *testing.T) {
	src, err := os.ReadFile("../abe/native/com_iid.c")
	require.NoError(t, err)

	kinds := map[string]masterkey.ABEKind{
		"BROWSER_CHROME_BASE": masterkey.ABEChromeBase,
		"BROWSER_EDGE":        masterkey.ABEEdge,
		"BROWSER_AVAST":       masterkey.ABEAvast,
	}
	payload := make(map[string]masterkey.ABEKind)
	for _, m := range regexp.MustCompile(`"([^"]+\.exe)",\s*(BROWSER_\w+)`).FindAllStringSubmatch(string(src), -1) {
		kind, ok := kinds[m[2]]
		require.Truef(t, ok, "unknown payload browser kind %s", m[2])
		exe := strings.ToLower(m[1])
		if _, dup := payload[exe]; !dup {
			payload[exe] = kind
		}
	}
	require.NotEmpty(t, payload, "no rows parsed from com_iid.c")

	for _, d := range defs {
		w := d.Windows
		if w.ABE == masterkey.ABENone {
			continue
		}
		kind, ok := payload[strings.ToLower(w.Exe)]
		if assert.Truef(t, ok, "%s: com_iid.c has no row for %s", d.ID, w.Exe) {
			assert.Equalf(t, w.ABE, kind, "%s: com_iid.c dispatches %s as another ABE kind", d.ID, w.Exe)
		}
	}
}

func TestIDs_AreSupportedSorted(t *testing.T) {
	ids := IDs()
	assert.True(t, sort.StringsAreSorted(ids), "the -b help has always listed them sorted")
	assert.ElementsMatch(t, idsOn(Def.Platform), ids)
}
